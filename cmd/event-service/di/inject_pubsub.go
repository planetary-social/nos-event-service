package di

import (
	"github.com/google/wire"
	"github.com/planetary-social/nos-event-service/service/adapters/memorypubsub"
	"github.com/planetary-social/nos-event-service/service/adapters/sqlite"
	"github.com/planetary-social/nos-event-service/service/app"
	"github.com/planetary-social/nos-event-service/service/ports/sqlitepubsub"
)

var memoryPubsubSet = wire.NewSet(
	memorypubsub.NewReceivedEventPubSub,
	wire.Bind(new(app.ReceivedEventPublisher), new(*memorypubsub.ReceivedEventPubSub)),
	wire.Bind(new(app.ReceivedEventSubscriber), new(*memorypubsub.ReceivedEventPubSub)),
)

var sqlitePubsubSet = wire.NewSet(
	sqlitepubsub.NewEventSavedEventSubscriber,
	sqlite.NewPubSub,

	sqlite.NewSubscriber,
	//wire.Bind(new(app.Subscriber), new(*sqlite.Subscriber)),
)

var sqliteTxPubsubSet = wire.NewSet(
	sqlite.NewPublisher,
	wire.Bind(new(app.Publisher), new(*sqlite.Publisher)),
)
