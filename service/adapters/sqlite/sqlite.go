package sqlite

import (
	"context"
	"database/sql"

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
	TransactionRunner          *TransactionRunner
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
	runner *TransactionRunner,
) *TransactionProvider {
	return &TransactionProvider{
		db:     db,
		fn:     fn,
		runner: runner,
	}
}

type TestAdaptersFactoryFn = GenericAdaptersFactoryFn[TestAdapters]
type TestTransactionProvider = GenericTransactionProvider[TestAdapters]

func NewTestTransactionProvider(
	db *sql.DB,
	fn TestAdaptersFactoryFn,
	runner *TransactionRunner,
) *TestTransactionProvider {
	return &TestTransactionProvider{
		db:     db,
		fn:     fn,
		runner: runner,
	}
}

type PubSubTxTransactionProvider = GenericTransactionProvider[*sql.Tx]

func NewPubSubTxTransactionProvider(
	db *sql.DB,
	runner *TransactionRunner,
) *PubSubTxTransactionProvider {
	return &PubSubTxTransactionProvider{
		db: db,
		fn: func(db *sql.DB, tx *sql.Tx) (*sql.Tx, error) {
			return tx, nil
		},
		runner: runner,
	}
}

type GenericAdaptersFactoryFn[T any] func(*sql.DB, *sql.Tx) (T, error)

type GenericTransactionProvider[T any] struct {
	db     *sql.DB
	fn     GenericAdaptersFactoryFn[T]
	runner *TransactionRunner
}

func (t *GenericTransactionProvider[T]) Transact(ctx context.Context, fn func(context.Context, T) error) error {
	transactionFunc := t.makeTransactionFunc(fn)
	return t.runner.TryRun(ctx, transactionFunc)
}

func (t *GenericTransactionProvider[T]) makeTransactionFunc(fn func(context.Context, T) error) TransactionFunc {
	return func(ctx context.Context, db *sql.DB, tx *sql.Tx) error {
		adapters, err := t.fn(t.db, tx)
		if err != nil {
			return errors.Wrap(err, "error building the adapters")
		}

		if err := fn(ctx, adapters); err != nil {
			return errors.Wrap(err, "error calling the adapters callback")
		}

		return nil
	}
}

type TransactionFunc func(context.Context, *sql.DB, *sql.Tx) error

type TransactionRunner struct {
	db   *sql.DB
	chIn chan transactionTask
}

func NewTransactionRunner(db *sql.DB) *TransactionRunner {
	return &TransactionRunner{
		db:   db,
		chIn: make(chan transactionTask),
	}
}

func (t *TransactionRunner) TryRun(ctx context.Context, fn TransactionFunc) error {
	resultCh := make(chan error)

	select {
	case t.chIn <- newTransactionTask(ctx, fn, resultCh):
	case <-ctx.Done():
		return ctx.Err()
	}

	select {
	case err := <-resultCh:
		return errors.Wrap(err, "received an error")
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (t *TransactionRunner) Run(ctx context.Context) error {
	for {
		select {
		case task := <-t.chIn:
			select {
			case task.ResultCh <- t.run(task.Ctx, task.Fn):
				continue
			case <-task.Ctx.Done():
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (t *TransactionRunner) run(ctx context.Context, fn TransactionFunc) error {
	tx, err := t.db.BeginTx(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "error starting the transaction")
	}

	if err := fn(ctx, t.db, tx); err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			err = multierror.Append(err, errors.Wrap(rollbackErr, "rollback error"))
		}
		return errors.Wrap(err, "error calling the callback")
	}

	if err := tx.Commit(); err != nil {
		return errors.Wrap(err, "error committing the transaction")
	}

	return nil
}

type transactionTask struct {
	Ctx      context.Context
	Fn       TransactionFunc
	ResultCh chan<- error
}

func newTransactionTask(ctx context.Context, fn TransactionFunc, resultCh chan<- error) transactionTask {
	return transactionTask{Ctx: ctx, Fn: fn, ResultCh: resultCh}
}
