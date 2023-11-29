package di

import (
	"github.com/google/wire"
	"github.com/planetary-social/nos-event-service/internal/logging"
	"github.com/planetary-social/nos-event-service/service/config"
)

var extractConfigSet = wire.NewSet(
	logLevelFromConfig,
)

func logLevelFromConfig(conf config.Config) logging.Level {
	return conf.LogLevel()
}
