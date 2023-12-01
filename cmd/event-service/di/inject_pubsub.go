package di

import (
	"github.com/ThreeDotsLabs/watermill"
	"github.com/boreq/errors"
	"github.com/google/wire"
	"github.com/planetary-social/nos-event-service/service/adapters/gcp"
	"github.com/planetary-social/nos-event-service/service/adapters/memorypubsub"
	"github.com/planetary-social/nos-event-service/service/adapters/sqlite"
	"github.com/planetary-social/nos-event-service/service/app"
	"github.com/planetary-social/nos-event-service/service/config"
	"github.com/planetary-social/nos-event-service/service/domain/downloader"
	"github.com/planetary-social/nos-event-service/service/ports/sqlitepubsub"
)

var memoryPubsubSet = wire.NewSet(
	memorypubsub.NewReceivedEventPubSub,
	wire.Bind(new(downloader.ReceivedEventPublisher), new(*memorypubsub.ReceivedEventPubSub)),
	wire.Bind(new(app.ReceivedEventSubscriber), new(*memorypubsub.ReceivedEventPubSub)),
)

var sqlitePubsubSet = wire.NewSet(
	sqlitepubsub.NewEventSavedEventSubscriber,
	sqlite.NewPubSub,

	sqlite.NewSubscriber,
	wire.Bind(new(app.Subscriber), new(*sqlite.Subscriber)),
)

var sqliteTxPubsubSet = wire.NewSet(
	sqlite.NewPublisher,
	wire.Bind(new(app.Publisher), new(*sqlite.Publisher)),
)

var externalPubsubSet = wire.NewSet(
	gcp.NewNoopPublisher,
	selectExternalPublisher,
)

func selectExternalPublisher(conf config.Config, logger watermill.LoggerAdapter, noop *gcp.NoopPublisher) (app.ExternalEventPublisher, error) {
	if conf.Environment() == config.EnvironmentDevelopment {
		return noop, nil
	}

	watermillPublisher, err := gcp.NewWatermillPublisher(conf, logger)
	if err != nil {
		return nil, errors.Wrap(err, "error creating watermill publisher")
	}

	return gcp.NewPublisher(watermillPublisher), nil
}
