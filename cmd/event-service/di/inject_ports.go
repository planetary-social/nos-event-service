package di

import (
	"github.com/google/wire"
	"github.com/planetary-social/nos-event-service/service/ports/http"
	"github.com/planetary-social/nos-event-service/service/ports/memorypubsub"
)

var portsSet = wire.NewSet(
	http.NewServer,

	memorypubsub.NewReceivedEventSubscriber,
)
