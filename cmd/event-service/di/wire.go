//go:build wireinject
// +build wireinject

package di

import (
	"context"
	"database/sql"
	"testing"

	"github.com/google/wire"
	"github.com/planetary-social/nos-event-service/internal/fixtures"
	"github.com/planetary-social/nos-event-service/internal/logging"
	"github.com/planetary-social/nos-event-service/service/adapters/sqlite"
	"github.com/planetary-social/nos-event-service/service/app"
	"github.com/planetary-social/nos-event-service/service/config"
	"github.com/planetary-social/nos-event-service/service/domain"
	"github.com/planetary-social/nos-event-service/service/domain/relays"
)

func BuildService(context.Context, config.Config) (Service, func(), error) {
	wire.Build(
		NewService,

		portsSet,
		applicationSet,
		sqliteAdaptersSet,
		downloaderSet,
		memoryPubsubSet,
		sqlitePubsubSet,
		loggingSet,
		adaptersSet,
		migrationsAdaptersSet,
		domainSet,
		externalPubsubSet,
	)
	return Service{}, nil, nil
}

func BuildTestAdapters(context.Context, testing.TB) (sqlite.TestedItems, func(), error) {
	wire.Build(
		wire.Struct(new(sqlite.TestedItems), "*"),

		sqliteTestAdaptersSet,
		sqlitePubsubSet,
		loggingSet,
		newTestAdaptersConfig,
		migrationsAdaptersSet,
	)
	return sqlite.TestedItems{}, nil, nil
}

func newTestAdaptersConfig(tb testing.TB) (config.Config, error) {
	return config.NewConfig(
		fixtures.SomeString(),
		config.EnvironmentDevelopment,
		logging.LevelDebug,
		fixtures.SomeString(),
		nil,
		fixtures.SomeFile(tb),
	)
}

type buildTransactionSqliteAdaptersDependencies struct {
	Logger logging.Logger
}

func buildTransactionSqliteAdapters(*sql.DB, *sql.Tx, buildTransactionSqliteAdaptersDependencies) (app.Adapters, error) {
	wire.Build(
		wire.Struct(new(app.Adapters), "*"),
		wire.FieldsOf(new(buildTransactionSqliteAdaptersDependencies), "Logger"),

		sqliteTxAdaptersSet,
		sqliteTxPubsubSet,
		sqlitePubsubSet,
	)
	return app.Adapters{}, nil
}

func buildTestTransactionSqliteAdapters(*sql.DB, *sql.Tx, buildTransactionSqliteAdaptersDependencies) (sqlite.TestAdapters, error) {
	wire.Build(
		wire.Struct(new(sqlite.TestAdapters), "*"),
		wire.FieldsOf(new(buildTransactionSqliteAdaptersDependencies), "Logger"),

		sqliteTxAdaptersSet,
		sqliteTxPubsubSet,
		sqlitePubsubSet,
	)
	return sqlite.TestAdapters{}, nil
}

var downloaderSet = wire.NewSet(
	app.NewRelayDownloaderFactory,
	app.NewDownloader,

	relays.NewBootstrapRelaySource,
	wire.Bind(new(app.BootstrapRelaySource), new(*relays.BootstrapRelaySource)),

	app.NewDatabaseRelaySource,
	wire.Bind(new(app.RelaySource), new(*app.DatabaseRelaySource)),

	relays.NewRelayConnections,
	wire.Bind(new(app.RelayConnections), new(*relays.RelayConnections)),
)

var domainSet = wire.NewSet(
	domain.NewRelaysExtractor,
	wire.Bind(new(app.RelaysExtractor), new(*domain.RelaysExtractor)),

	domain.NewContactsExtractor,
	wire.Bind(new(app.ContactsExtractor), new(*domain.ContactsExtractor)),
)
