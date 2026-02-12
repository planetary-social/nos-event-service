package di

import (
	"context"
	"os"
	"time"

	"github.com/boreq/errors"
	"github.com/hashicorp/go-multierror"
	"github.com/planetary-social/nos-event-service/internal/goroutine"
	"github.com/planetary-social/nos-event-service/internal/logging"
	"github.com/planetary-social/nos-event-service/internal/migrations"
	"github.com/planetary-social/nos-event-service/service/adapters/sqlite"
	"github.com/planetary-social/nos-event-service/service/app"
	"github.com/planetary-social/nos-event-service/service/domain/downloader"
	"github.com/planetary-social/nos-event-service/service/ports/http"
	"github.com/planetary-social/nos-event-service/service/ports/memorypubsub"
	"github.com/planetary-social/nos-event-service/service/ports/sqlitepubsub"
	"github.com/planetary-social/nos-event-service/service/ports/timer"
)

type Service struct {
	app                        app.Application
	server                     http.Server
	downloader                 *downloader.Downloader
	receivedEventSubscriber    *memorypubsub.ReceivedEventSubscriber
	eventSavedEventSubscriber  *sqlitepubsub.EventSavedEventSubscriber
	metricsTimer               *timer.Metrics
	cleanupTimer               *timer.Cleanup
	transactionRunner          *sqlite.TransactionRunner
	taskScheduler              *downloader.TaskScheduler
	migrationsRunner           *migrations.Runner
	migrations                 migrations.Migrations
	migrationsProgressCallback migrations.ProgressCallback
	logger                     logging.Logger
}

func NewService(
	app app.Application,
	server http.Server,
	downloader *downloader.Downloader,
	receivedEventSubscriber *memorypubsub.ReceivedEventSubscriber,
	eventSavedEventSubscriber *sqlitepubsub.EventSavedEventSubscriber,
	metricsTimer *timer.Metrics,
	cleanupTimer *timer.Cleanup,
	transactionRunner *sqlite.TransactionRunner,
	taskScheduler *downloader.TaskScheduler,
	migrationsRunner *migrations.Runner,
	migrations migrations.Migrations,
	migrationsProgressCallback migrations.ProgressCallback,
	logger logging.Logger,
) Service {
	return Service{
		app:                        app,
		server:                     server,
		downloader:                 downloader,
		receivedEventSubscriber:    receivedEventSubscriber,
		eventSavedEventSubscriber:  eventSavedEventSubscriber,
		metricsTimer:               metricsTimer,
		cleanupTimer:               cleanupTimer,
		transactionRunner:          transactionRunner,
		migrationsRunner:           migrationsRunner,
		taskScheduler:              taskScheduler,
		migrations:                 migrations,
		migrationsProgressCallback: migrationsProgressCallback,
		logger:                     logger.New("service"),
	}
}

func (s Service) App() app.Application {
	return s.app
}

func (s Service) ExecuteMigrations(ctx context.Context) error {
	return s.migrationsRunner.Run(ctx, s.migrations, s.migrationsProgressCallback)
}

const shutdownTimeout = 30 * time.Second

func (s Service) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error)
	runners := 0

	runners++
	// Serve http
	goroutine.Run(s.logger, errCh, func() error {
		return errors.Wrap(s.server.ListenAndServe(ctx), "server error")
	})

	// Fetch events from the database relays and send them to the in memory pubsub
	runners++
	goroutine.Run(s.logger, errCh, func() error {
		return errors.Wrap(s.downloader.Run(ctx), "downloader error")
	})

	// Subscribe to the in memory pubsub of events, emit a NewSaveReceivedEvent
	// command that will make some checks on the event, save it if the check
	// passes and emit a EventSavedEvent to the sqlite pubsub.
	runners++
	goroutine.Run(s.logger, errCh, func() error {
		return errors.Wrap(s.receivedEventSubscriber.Run(ctx), "received event subscriber error")
	})

	// Subscribe to saved events in the database. This uses the sqlite pubsub. This triggers:
	// - analysis to extract new relays and store them in db. They will be used by the downloader.
	// - analysis to store pubkeys and store them in the db (contacts_followees, pubkeys, contacts_events). This will be used by the downloader.
	// - publish to watermill pubsub
	// - may publish the event in wss://relay.nos.social if they are metadata related
	runners++
	goroutine.Run(s.logger, errCh, func() error {
		return errors.Wrap(s.eventSavedEventSubscriber.Run(ctx), "event saved subscriber error")
	})

	// The metrics timer collects metrics from the app.
	runners++
	goroutine.Run(s.logger, errCh, func() error {
		return errors.Wrap(s.metricsTimer.Run(ctx), "metrics timer error")
	})

	// Sqlite transaction runner
	runners++
	goroutine.Run(s.logger, errCh, func() error {
		return errors.Wrap(s.transactionRunner.Run(ctx), "transaction runner error")
	})

	// The task scheduler creates sequential time window based tasks that
	// contain filters to be applied to each relay to fetch the events we want.
	// Event downloaders subscribe to this.
	runners++
	goroutine.Run(s.logger, errCh, func() error {
		return errors.Wrap(s.taskScheduler.Run(ctx), "task scheduler error")
	})

	// The cleanup timer periodically removes processed events older than retention period
	runners++
	goroutine.Run(s.logger, errCh, func() error {
		return errors.Wrap(s.cleanupTimer.Run(ctx), "cleanup timer error")
	})

	// Wait for the first runner to exit.
	firstErr := <-errCh
	s.logger.Error().WithError(firstErr).Message("first runner terminated, shutting down")
	cancel()

	// Collect remaining errors with a timeout to prevent zombie processes.
	var compoundErr error
	compoundErr = multierror.Append(compoundErr, errors.Wrap(firstErr, "error returned by runner"))

	timer := time.NewTimer(shutdownTimeout)
	defer timer.Stop()

	for i := 1; i < runners; i++ {
		select {
		case err := <-errCh:
			err = errors.Wrap(err, "error returned by runner")
			s.logger.Error().WithError(err).Message("runner terminated")
			compoundErr = multierror.Append(compoundErr, err)
		case <-timer.C:
			s.logger.Error().Message("shutdown timeout exceeded, forcing exit")
			os.Exit(1)
		}
	}

	return compoundErr
}
