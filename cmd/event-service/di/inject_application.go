package di

import (
	"github.com/google/wire"
	"github.com/planetary-social/nos-event-service/service/app"
	"github.com/planetary-social/nos-event-service/service/ports/memorypubsub"
	"github.com/planetary-social/nos-event-service/service/ports/sqlitepubsub"
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
