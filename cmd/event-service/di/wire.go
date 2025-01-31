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
	"github.com/planetary-social/nos-event-service/service/ports/memorypubsub"
	"github.com/planetary-social/nos-event-service/service/ports/sqlitepubsub"
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
		bloomSet,
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
	EventRepository *mocks.EventRepository
}

func BuildTestApplication(context.Context, testing.TB) (TestApplication, error) {
	wire.Build(
		wire.Struct(new(TestApplication), "*"),
		mocks.NewEventRepository,
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
	newCachedPublicKeySource,
	wire.Bind(new(downloader.PublicKeySource), new(*app.CachedDatabasePublicKeySource)),

	relays.NewEventSender,
	wire.Bind(new(app.EventSender), new(*relays.EventSender)),

	downloader.NewTaskScheduler,
	wire.Bind(new(downloader.Scheduler), new(*downloader.TaskScheduler)),
)

func newCachedPublicKeySource(underlying *app.DatabasePublicKeySource) *app.CachedDatabasePublicKeySource {
	return app.NewCachedDatabasePublicKeySource(underlying)
}

var domainSet = wire.NewSet(
	domain.NewRelaysExtractor,
	wire.Bind(new(app.RelaysExtractor), new(*domain.RelaysExtractor)),

	domain.NewContactsExtractor,
	wire.Bind(new(app.ContactsExtractor), new(*domain.ContactsExtractor)),
)

var applicationSet = wire.NewSet(
	wire.Struct(new(app.Application), "*"),

	app.NewSaveReceivedEventHandler,
	wire.Bind(new(memorypubsub.SaveReceivedEventHandler), new(*app.SaveReceivedEventHandler)),

	app.NewProcessSavedEventHandler,
	wire.Bind(new(sqlitepubsub.ProcessSavedEventHandler), new(*app.ProcessSavedEventHandler)),

	app.NewUpdateMetricsHandler,
	app.NewAddPublicKeyToMonitorHandler,
	app.NewGetPublicKeyInfoHandler,
)
