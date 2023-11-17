// Code generated by Wire. DO NOT EDIT.

//go:generate go run github.com/google/wire/cmd/wire
//go:build !wireinject
// +build !wireinject

package di

import (
	"context"
	"database/sql"
	"testing"

	"github.com/google/wire"
	"github.com/planetary-social/nos-event-service/internal/fixtures"
	"github.com/planetary-social/nos-event-service/internal/logging"
	"github.com/planetary-social/nos-event-service/internal/migrations"
	"github.com/planetary-social/nos-event-service/service/adapters"
	"github.com/planetary-social/nos-event-service/service/adapters/gcp"
	"github.com/planetary-social/nos-event-service/service/adapters/memorypubsub"
	"github.com/planetary-social/nos-event-service/service/adapters/prometheus"
	"github.com/planetary-social/nos-event-service/service/adapters/sqlite"
	"github.com/planetary-social/nos-event-service/service/app"
	"github.com/planetary-social/nos-event-service/service/config"
	"github.com/planetary-social/nos-event-service/service/domain"
	"github.com/planetary-social/nos-event-service/service/domain/relays"
	"github.com/planetary-social/nos-event-service/service/ports/http"
	memorypubsub2 "github.com/planetary-social/nos-event-service/service/ports/memorypubsub"
	"github.com/planetary-social/nos-event-service/service/ports/sqlitepubsub"
	"github.com/planetary-social/nos-event-service/service/ports/timer"
)

// Injectors from wire.go:

func BuildService(contextContext context.Context, configConfig config.Config) (Service, func(), error) {
	logger, err := newLogger(configConfig)
	if err != nil {
		return Service{}, nil, err
	}
	db, cleanup, err := newSqliteDB(configConfig, logger)
	if err != nil {
		return Service{}, nil, err
	}
	diBuildTransactionSqliteAdaptersDependencies := buildTransactionSqliteAdaptersDependencies{
		Logger: logger,
	}
	genericAdaptersFactoryFn := newAdaptersFactoryFn(diBuildTransactionSqliteAdaptersDependencies)
	databaseMutex := sqlite.NewDatabaseMutex()
	genericTransactionProvider := sqlite.NewTransactionProvider(db, genericAdaptersFactoryFn, databaseMutex)
	prometheusPrometheus, err := prometheus.NewPrometheus(logger)
	if err != nil {
		cleanup()
		return Service{}, nil, err
	}
	saveReceivedEventHandler := app.NewSaveReceivedEventHandler(genericTransactionProvider, logger, prometheusPrometheus)
	relaysExtractor := domain.NewRelaysExtractor(logger)
	contactsExtractor := domain.NewContactsExtractor(logger)
	watermillAdapter := logging.NewWatermillAdapter(logger)
	noopPublisher := gcp.NewNoopPublisher()
	externalEventPublisher, err := selectExternalPublisher(configConfig, watermillAdapter, noopPublisher)
	if err != nil {
		cleanup()
		return Service{}, nil, err
	}
	processSavedEventHandler := app.NewProcessSavedEventHandler(genericTransactionProvider, relaysExtractor, contactsExtractor, externalEventPublisher, logger, prometheusPrometheus)
	sqliteGenericTransactionProvider := sqlite.NewPubSubTxTransactionProvider(db, databaseMutex)
	pubSub := sqlite.NewPubSub(sqliteGenericTransactionProvider, logger)
	subscriber := sqlite.NewSubscriber(pubSub, db)
	updateMetricsHandler := app.NewUpdateMetricsHandler(genericTransactionProvider, subscriber, logger, prometheusPrometheus)
	addPublicKeyToMonitorHandler := app.NewAddPublicKeyToMonitorHandler(genericTransactionProvider, logger, prometheusPrometheus)
	application := app.Application{
		SaveReceivedEvent:     saveReceivedEventHandler,
		ProcessSavedEvent:     processSavedEventHandler,
		UpdateMetrics:         updateMetricsHandler,
		AddPublicKeyToMonitor: addPublicKeyToMonitorHandler,
	}
	server := http.NewServer(configConfig, logger, application, prometheusPrometheus)
	bootstrapRelaySource := relays.NewBootstrapRelaySource()
	databaseRelaySource := app.NewDatabaseRelaySource(genericTransactionProvider, logger)
	databasePublicKeySource := app.NewDatabasePublicKeySource(genericTransactionProvider, logger)
	relayConnections := relays.NewRelayConnections(contextContext, logger, prometheusPrometheus)
	receivedEventPubSub := memorypubsub.NewReceivedEventPubSub()
	relayDownloaderFactory := app.NewRelayDownloaderFactory(databasePublicKeySource, relayConnections, receivedEventPubSub, logger, prometheusPrometheus)
	downloader := app.NewDownloader(bootstrapRelaySource, databaseRelaySource, logger, prometheusPrometheus, relayDownloaderFactory)
	receivedEventSubscriber := memorypubsub2.NewReceivedEventSubscriber(receivedEventPubSub, saveReceivedEventHandler, logger)
	eventSavedEventSubscriber := sqlitepubsub.NewEventSavedEventSubscriber(processSavedEventHandler, subscriber, logger, prometheusPrometheus)
	metrics := timer.NewMetrics(application, logger)
	migrationsStorage, err := sqlite.NewMigrationsStorage(db)
	if err != nil {
		cleanup()
		return Service{}, nil, err
	}
	runner := migrations.NewRunner(migrationsStorage, logger)
	migrationFns := sqlite.NewMigrationFns(db, pubSub)
	migrationsMigrations, err := sqlite.NewMigrations(migrationFns)
	if err != nil {
		cleanup()
		return Service{}, nil, err
	}
	loggingMigrationsProgressCallback := adapters.NewLoggingMigrationsProgressCallback(logger)
	service := NewService(application, server, downloader, receivedEventSubscriber, eventSavedEventSubscriber, metrics, runner, migrationsMigrations, loggingMigrationsProgressCallback)
	return service, func() {
		cleanup()
	}, nil
}

func BuildTestAdapters(contextContext context.Context, tb testing.TB) (sqlite.TestedItems, func(), error) {
	configConfig, err := newTestAdaptersConfig(tb)
	if err != nil {
		return sqlite.TestedItems{}, nil, err
	}
	logger, err := newLogger(configConfig)
	if err != nil {
		return sqlite.TestedItems{}, nil, err
	}
	db, cleanup, err := newSqliteDB(configConfig, logger)
	if err != nil {
		return sqlite.TestedItems{}, nil, err
	}
	diBuildTransactionSqliteAdaptersDependencies := buildTransactionSqliteAdaptersDependencies{
		Logger: logger,
	}
	genericAdaptersFactoryFn := newTestAdaptersFactoryFn(diBuildTransactionSqliteAdaptersDependencies)
	databaseMutex := sqlite.NewDatabaseMutex()
	genericTransactionProvider := sqlite.NewTestTransactionProvider(db, genericAdaptersFactoryFn, databaseMutex)
	sqliteGenericTransactionProvider := sqlite.NewPubSubTxTransactionProvider(db, databaseMutex)
	pubSub := sqlite.NewPubSub(sqliteGenericTransactionProvider, logger)
	subscriber := sqlite.NewSubscriber(pubSub, db)
	migrationsStorage, err := sqlite.NewMigrationsStorage(db)
	if err != nil {
		cleanup()
		return sqlite.TestedItems{}, nil, err
	}
	runner := migrations.NewRunner(migrationsStorage, logger)
	migrationFns := sqlite.NewMigrationFns(db, pubSub)
	migrationsMigrations, err := sqlite.NewMigrations(migrationFns)
	if err != nil {
		cleanup()
		return sqlite.TestedItems{}, nil, err
	}
	loggingMigrationsProgressCallback := adapters.NewLoggingMigrationsProgressCallback(logger)
	testedItems := sqlite.TestedItems{
		TransactionProvider:        genericTransactionProvider,
		Subscriber:                 subscriber,
		MigrationsStorage:          migrationsStorage,
		PubSub:                     pubSub,
		MigrationsRunner:           runner,
		Migrations:                 migrationsMigrations,
		MigrationsProgressCallback: loggingMigrationsProgressCallback,
	}
	return testedItems, func() {
		cleanup()
	}, nil
}

func buildTransactionSqliteAdapters(db *sql.DB, tx *sql.Tx, diBuildTransactionSqliteAdaptersDependencies buildTransactionSqliteAdaptersDependencies) (app.Adapters, error) {
	eventRepository, err := sqlite.NewEventRepository(tx)
	if err != nil {
		return app.Adapters{}, err
	}
	relayRepository := sqlite.NewRelayRepository(tx)
	contactRepository := sqlite.NewContactRepository(tx)
	publicKeysToMonitorRepository, err := sqlite.NewPublicKeysToMonitorRepository(tx)
	if err != nil {
		return app.Adapters{}, err
	}
	logger := diBuildTransactionSqliteAdaptersDependencies.Logger
	txPubSub := sqlite.NewTxPubSub(tx, logger)
	publisher := sqlite.NewPublisher(txPubSub)
	appAdapters := app.Adapters{
		Events:              eventRepository,
		Relays:              relayRepository,
		Contacts:            contactRepository,
		PublicKeysToMonitor: publicKeysToMonitorRepository,
		Publisher:           publisher,
	}
	return appAdapters, nil
}

func buildTestTransactionSqliteAdapters(db *sql.DB, tx *sql.Tx, diBuildTransactionSqliteAdaptersDependencies buildTransactionSqliteAdaptersDependencies) (sqlite.TestAdapters, error) {
	eventRepository, err := sqlite.NewEventRepository(tx)
	if err != nil {
		return sqlite.TestAdapters{}, err
	}
	relayRepository := sqlite.NewRelayRepository(tx)
	contactRepository := sqlite.NewContactRepository(tx)
	publicKeysToMonitorRepository, err := sqlite.NewPublicKeysToMonitorRepository(tx)
	if err != nil {
		return sqlite.TestAdapters{}, err
	}
	logger := diBuildTransactionSqliteAdaptersDependencies.Logger
	txPubSub := sqlite.NewTxPubSub(tx, logger)
	publisher := sqlite.NewPublisher(txPubSub)
	testAdapters := sqlite.TestAdapters{
		EventRepository:               eventRepository,
		RelayRepository:               relayRepository,
		ContactRepository:             contactRepository,
		PublicKeysToMonitorRepository: publicKeysToMonitorRepository,
		Publisher:                     publisher,
	}
	return testAdapters, nil
}

// wire.go:

func newTestAdaptersConfig(tb testing.TB) (config.Config, error) {
	return config.NewConfig(fixtures.SomeString(), config.EnvironmentDevelopment, logging.LevelDebug, fixtures.SomeString(), nil, fixtures.SomeFile(tb))
}

type buildTransactionSqliteAdaptersDependencies struct {
	Logger logging.Logger
}

var downloaderSet = wire.NewSet(app.NewRelayDownloaderFactory, app.NewDownloader, relays.NewBootstrapRelaySource, wire.Bind(new(app.BootstrapRelaySource), new(*relays.BootstrapRelaySource)), app.NewDatabaseRelaySource, wire.Bind(new(app.RelaySource), new(*app.DatabaseRelaySource)), relays.NewRelayConnections, wire.Bind(new(app.RelayConnections), new(*relays.RelayConnections)), app.NewDatabasePublicKeySource, wire.Bind(new(app.PublicKeySource), new(*app.DatabasePublicKeySource)))

var domainSet = wire.NewSet(domain.NewRelaysExtractor, wire.Bind(new(app.RelaysExtractor), new(*domain.RelaysExtractor)), domain.NewContactsExtractor, wire.Bind(new(app.ContactsExtractor), new(*domain.ContactsExtractor)))
