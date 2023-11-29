package di

import (
	"github.com/ThreeDotsLabs/watermill"
	"github.com/boreq/errors"
	"github.com/google/wire"
	"github.com/planetary-social/nos-event-service/internal/logging"
	"github.com/sirupsen/logrus"
)

var loggingSet = wire.NewSet(
	newLogger,

	logging.NewWatermillAdapter,
	wire.Bind(new(watermill.LoggerAdapter), new(*logging.WatermillAdapter)),
)

func newLogger(level logging.Level) (logging.Logger, error) {
	if level == logging.LevelDisabled {
		return logging.NewDevNullLogger(), nil
	}

	v := logrus.New()
	switch level {
	case logging.LevelTrace:
		v.SetLevel(logrus.TraceLevel)
	case logging.LevelDebug:
		v.SetLevel(logrus.DebugLevel)
	case logging.LevelError:
		v.SetLevel(logrus.ErrorLevel)
	default:
		return nil, errors.New("unsupported log level")
	}

	return logging.NewSystemLogger(logging.NewLogrusLoggingSystem(v), "root"), nil
}
