package di

import (
	"database/sql"

	"github.com/boreq/errors"
	"github.com/google/wire"
	"github.com/planetary-social/nos-event-service/internal/logging"
	"github.com/planetary-social/nos-event-service/service/adapters/prometheus"
	"github.com/planetary-social/nos-event-service/service/adapters/sqlite"
	"github.com/planetary-social/nos-event-service/service/app"
	"github.com/planetary-social/nos-event-service/service/config"
	"github.com/planetary-social/nos-event-service/service/domain/relays"
)

var sqliteAdaptersSet = wire.NewSet(
	newSqliteDB,
	sqlite.NewDatabaseMutex,

	sqlite.NewTransactionProvider,
	wire.Bind(new(app.TransactionProvider), new(*sqlite.TransactionProvider)),

	sqlite.NewPubSubTxTransactionProvider,
	wire.Bind(new(sqlite.PubsubTransactionProvider), new(*sqlite.PubSubTxTransactionProvider)),

	newAdaptersFactoryFn,

	wire.Struct(new(buildTransactionSqliteAdaptersDependencies), "*"),
)

var sqliteTestAdaptersSet = wire.NewSet(
	newSqliteDB,
	sqlite.NewDatabaseMutex,

	sqlite.NewTestTransactionProvider,

	sqlite.NewPubSubTxTransactionProvider,
	wire.Bind(new(sqlite.PubsubTransactionProvider), new(*sqlite.PubSubTxTransactionProvider)),

	newTestAdaptersFactoryFn,

	wire.Struct(new(buildTransactionSqliteAdaptersDependencies), "*"),
)

var sqliteTxAdaptersSet = wire.NewSet(
	sqlite.NewEventRepository,
	wire.Bind(new(app.EventRepository), new(*sqlite.EventRepository)),

	sqlite.NewRelayRepository,
	wire.Bind(new(app.RelayRepository), new(*sqlite.RelayRepository)),

	sqlite.NewContactRepository,
	wire.Bind(new(app.ContactRepository), new(*sqlite.ContactRepository)),

	sqlite.NewPublicKeysToMonitorRepository,
	wire.Bind(new(app.PublicKeysToMonitorRepository), new(*sqlite.PublicKeysToMonitorRepository)),

	sqlite.NewTxPubSub,
)

var adaptersSet = wire.NewSet(
	prometheus.NewPrometheus,
	wire.Bind(new(app.Metrics), new(*prometheus.Prometheus)),
	wire.Bind(new(relays.Metrics), new(*prometheus.Prometheus)),
)

func newAdaptersFactoryFn(deps buildTransactionSqliteAdaptersDependencies) sqlite.AdaptersFactoryFn {
	return func(db *sql.DB, tx *sql.Tx) (app.Adapters, error) {
		return buildTransactionSqliteAdapters(db, tx, deps)
	}
}

func newTestAdaptersFactoryFn(deps buildTransactionSqliteAdaptersDependencies) sqlite.TestAdaptersFactoryFn {
	return func(db *sql.DB, tx *sql.Tx) (sqlite.TestAdapters, error) {
		return buildTestTransactionSqliteAdapters(db, tx, deps)
	}
}

func newSqliteDB(conf config.Config, logger logging.Logger) (*sql.DB, func(), error) {
	v, err := sqlite.Open(conf)
	if err != nil {
		return nil, nil, errors.Wrap(err, "error opening sqlite")
	}

	return v, func() {
		if err := v.Close(); err != nil {
			logger.Error().WithError(err).Message("error closing sqlite")
		}
	}, nil
}
