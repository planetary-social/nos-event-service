package di

import (
	"context"

	"github.com/boreq/errors"
	"github.com/hashicorp/go-multierror"
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
	transactionRunner          *sqlite.TransactionRunner
	taskScheduler              *downloader.TaskScheduler
	migrationsRunner           *migrations.Runner
	migrations                 migrations.Migrations
	migrationsProgressCallback migrations.ProgressCallback
}

func NewService(
	app app.Application,
	server http.Server,
	downloader *downloader.Downloader,
	receivedEventSubscriber *memorypubsub.ReceivedEventSubscriber,
	eventSavedEventSubscriber *sqlitepubsub.EventSavedEventSubscriber,
	metricsTimer *timer.Metrics,
	transactionRunner *sqlite.TransactionRunner,
	taskScheduler *downloader.TaskScheduler,
	migrationsRunner *migrations.Runner,
	migrations migrations.Migrations,
	migrationsProgressCallback migrations.ProgressCallback,
) Service {
	return Service{
		app:                        app,
		server:                     server,
		downloader:                 downloader,
		receivedEventSubscriber:    receivedEventSubscriber,
		eventSavedEventSubscriber:  eventSavedEventSubscriber,
		metricsTimer:               metricsTimer,
		transactionRunner:          transactionRunner,
		migrationsRunner:           migrationsRunner,
		taskScheduler:              taskScheduler,
		migrations:                 migrations,
		migrationsProgressCallback: migrationsProgressCallback,
	}
}

func (s Service) App() app.Application {
	return s.app
}

func (s Service) ExecuteMigrations(ctx context.Context) error {
	return s.migrationsRunner.Run(ctx, s.migrations, s.migrationsProgressCallback)
}

func (s Service) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error)
	runners := 0

	runners++
	go func() {
		errCh <- errors.Wrap(s.server.ListenAndServe(ctx), "server error")
	}()

	runners++
	go func() {
		errCh <- errors.Wrap(s.downloader.Run(ctx), "downloader error")
	}()

	runners++
	go func() {
		errCh <- errors.Wrap(s.receivedEventSubscriber.Run(ctx), "received event subscriber error")
	}()

	runners++
	go func() {
		errCh <- errors.Wrap(s.eventSavedEventSubscriber.Run(ctx), "event saved subscriber error")
	}()

	runners++
	go func() {
		errCh <- errors.Wrap(s.metricsTimer.Run(ctx), "metrics timer error")
	}()

	runners++
	go func() {
		errCh <- errors.Wrap(s.transactionRunner.Run(ctx), "transaction runner error")
	}()

	runners++
	go func() {
		errCh <- errors.Wrap(s.taskScheduler.Run(ctx), "task scheduler error")
	}()

	var err error
	for i := 0; i < runners; i++ {
		err = multierror.Append(err, errors.Wrap(<-errCh, "error returned by runner"))
		cancel()
	}

	return err
}
