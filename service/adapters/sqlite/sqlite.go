package sqlite

import (
	"context"
	"database/sql"
	"sync"

	"github.com/boreq/errors"
	"github.com/hashicorp/go-multierror"
	_ "github.com/mattn/go-sqlite3"
	"github.com/planetary-social/nos-event-service/internal/migrations"
	"github.com/planetary-social/nos-event-service/service/app"
	"github.com/planetary-social/nos-event-service/service/config"
)

type TestAdapters struct {
	EventRepository               *EventRepository
	RelayRepository               *RelayRepository
	ContactRepository             *ContactRepository
	PublicKeysToMonitorRepository *PublicKeysToMonitorRepository
	Publisher                     *Publisher
}

type TestedItems struct {
	TransactionProvider *TestTransactionProvider
	Subscriber          *Subscriber
	MigrationsStorage   *MigrationsStorage
	PubSub              *PubSub

	MigrationsRunner           *migrations.Runner
	Migrations                 migrations.Migrations
	MigrationsProgressCallback migrations.ProgressCallback
}

func Open(conf config.Config) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", conf.DatabasePath())
	if err != nil {
		return nil, errors.Wrap(err, "error opening the database")
	}

	return db, nil
}

type AdaptersFactoryFn = GenericAdaptersFactoryFn[app.Adapters]
type TransactionProvider = GenericTransactionProvider[app.Adapters]

func NewTransactionProvider(
	db *sql.DB,
	fn AdaptersFactoryFn,
	mutex *DatabaseMutex,
) *TransactionProvider {
	return &TransactionProvider{
		db:    db,
		fn:    fn,
		mutex: mutex,
	}
}

type TestAdaptersFactoryFn = GenericAdaptersFactoryFn[TestAdapters]
type TestTransactionProvider = GenericTransactionProvider[TestAdapters]

func NewTestTransactionProvider(
	db *sql.DB,
	fn TestAdaptersFactoryFn,
	mutex *DatabaseMutex,
) *TestTransactionProvider {
	return &TestTransactionProvider{
		db:    db,
		fn:    fn,
		mutex: mutex,
	}
}

type PubSubTxTransactionProvider = GenericTransactionProvider[*sql.Tx]

func NewPubSubTxTransactionProvider(
	db *sql.DB,
	mutex *DatabaseMutex,
) *PubSubTxTransactionProvider {
	return &PubSubTxTransactionProvider{
		db: db,
		fn: func(db *sql.DB, tx *sql.Tx) (*sql.Tx, error) {
			return tx, nil
		},
		mutex: mutex,
	}
}

type GenericAdaptersFactoryFn[T any] func(*sql.DB, *sql.Tx) (T, error)

type GenericTransactionProvider[T any] struct {
	db    *sql.DB
	fn    GenericAdaptersFactoryFn[T]
	mutex *DatabaseMutex
}

func (t *GenericTransactionProvider[T]) Transact(ctx context.Context, f func(context.Context, T) error) error {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	tx, err := t.db.BeginTx(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "error starting the transaction")
	}

	adapters, err := t.fn(t.db, tx)
	if err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			err = multierror.Append(err, errors.Wrap(rollbackErr, "rollback error"))
		}
		return errors.Wrap(err, "error building the adapters")
	}

	if err := f(ctx, adapters); err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			err = multierror.Append(err, errors.Wrap(rollbackErr, "rollback error"))
		}
		return errors.Wrap(err, "error calling the provided function")
	}

	if err := tx.Commit(); err != nil {
		return errors.Wrap(err, "error committing the transaction")
	}

	return nil
}

type DatabaseMutex struct {
	m sync.Mutex
}

func NewDatabaseMutex() *DatabaseMutex {
	return &DatabaseMutex{}
}

func (m *DatabaseMutex) Lock() {
	m.m.Lock()
}

func (m *DatabaseMutex) Unlock() {
	m.m.Unlock()
}
