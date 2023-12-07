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
	"github.com/planetary-social/nos-event-service/service/adapters/mocks"
	"github.com/planetary-social/nos-event-service/service/adapters/sqlite"
	"github.com/planetary-social/nos-event-service/service/app"
	"github.com/planetary-social/nos-event-service/service/config"
	"github.com/planetary-social/nos-event-service/service/domain"
	"github.com/planetary-social/nos-event-service/service/domain/downloader"
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
		extractConfigSet,
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
		extractConfigSet,
	)
	return sqlite.TestedItems{}, nil, nil
}

type TestApplication struct {
	GetEventsHandler *app.GetEventsHandler

	EventRepository *mocks.EventRepository
}

func BuildTestApplication(context.Context, testing.TB) (TestApplication, error) {
	wire.Build(
		wire.Struct(new(TestApplication), "*"),

		wire.Struct(new(app.Adapters), "*"),

		mocks.NewTransactionProvider,
		wire.Bind(new(app.TransactionProvider), new(*mocks.TransactionProvider)),

		mocks.NewEventRepository,
		wire.Bind(new(app.EventRepository), new(*mocks.EventRepository)),

		mocks.NewRelayRepository,
		wire.Bind(new(app.RelayRepository), new(*mocks.RelayRepository)),

		mocks.NewContactRepository,
		wire.Bind(new(app.ContactRepository), new(*mocks.ContactRepository)),

		mocks.NewPublicKeysToMonitorRepository,
		wire.Bind(new(app.PublicKeysToMonitorRepository), new(*mocks.PublicKeysToMonitorRepository)),

		mocks.NewPublisher,
		wire.Bind(new(app.Publisher), new(*mocks.Publisher)),

		mocks.NewMetrics,
		wire.Bind(new(app.Metrics), new(*mocks.Metrics)),

		applicationSet,
		loggingSet,

		wire.Value(logging.LevelError),
	)
	return TestApplication{}, nil
}

func newTestAdaptersConfig(tb testing.TB) (config.Config, error) {
	return config.NewConfig(
		fixtures.SomeString(),
		config.EnvironmentDevelopment,
		logging.LevelDebug,
		fixtures.SomeString(),
		nil,
		fixtures.SomeFile(tb),
		fixtures.SomeString(),
		fixtures.SomeString(),
		fixtures.SomeString(),
		fixtures.SomeString(),
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
	)
	return app.Adapters{}, nil
}

func buildTestTransactionSqliteAdapters(*sql.DB, *sql.Tx, buildTransactionSqliteAdaptersDependencies) (sqlite.TestAdapters, error) {
	wire.Build(
		wire.Struct(new(sqlite.TestAdapters), "*"),
		wire.FieldsOf(new(buildTransactionSqliteAdaptersDependencies), "Logger"),

		sqliteTxAdaptersSet,
		sqliteTxPubsubSet,
	)
	return sqlite.TestAdapters{}, nil
}

var downloaderSet = wire.NewSet(
	downloader.NewRelayDownloaderFactory,
	downloader.NewDownloader,

	relays.NewBootstrapRelaySource,
	wire.Bind(new(downloader.BootstrapRelaySource), new(*relays.BootstrapRelaySource)),

	app.NewDatabaseRelaySource,
	wire.Bind(new(downloader.RelaySource), new(*app.DatabaseRelaySource)),

	relays.NewRelayConnections,
	wire.Bind(new(downloader.RelayConnections), new(*relays.RelayConnections)),

	app.NewDatabasePublicKeySource,
	wire.Bind(new(downloader.PublicKeySource), new(*app.DatabasePublicKeySource)),

	relays.NewEventSender,
	wire.Bind(new(app.EventSender), new(*relays.EventSender)),
)

var domainSet = wire.NewSet(
	domain.NewRelaysExtractor,
	wire.Bind(new(app.RelaysExtractor), new(*domain.RelaysExtractor)),

	domain.NewContactsExtractor,
	wire.Bind(new(app.ContactsExtractor), new(*domain.ContactsExtractor)),
)
