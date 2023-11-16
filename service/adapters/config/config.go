package config

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/boreq/errors"
	"github.com/planetary-social/nos-event-service/internal/logging"
	"github.com/planetary-social/nos-event-service/service/config"
)

const (
	envPrefix = "EVENTS"

	envListenAddress                   = "LISTEN_ADDRESS"
	envEnvironment                     = "ENVIRONMENT"
	envLogLevel                        = "LOG_LEVEL"
	envGooglePubsubProjectID           = "GOOGLE_PUBSUB_PROJECT_ID"
	envGooglePubsubCredentialsJSONPath = "GOOGLE_PUBSUB_CREDENTIALS_JSON_PATH"
	envDatabasePath                    = "DATABASE_PATH"
)

type EnvironmentConfigLoader struct {
}

func NewEnvironmentConfigLoader() *EnvironmentConfigLoader {
	return &EnvironmentConfigLoader{}
}

func (c *EnvironmentConfigLoader) Load() (config.Config, error) {
	environment, err := c.loadEnvironment()
	if err != nil {
		return config.Config{}, errors.Wrap(err, "error loading the environment setting")
	}

	logLevel, err := c.loadLogLevel()
	if err != nil {
		return config.Config{}, errors.Wrap(err, "error loading the log level")
	}

	var googlePubSubCredentialsJSON []byte
	if p := c.getenv(envGooglePubsubCredentialsJSONPath); p != "" {
		f, err := os.Open(p)
		if err != nil {
			return config.Config{}, errors.Wrap(err, "error opening the pubsub credentials file")
		}

		b, err := io.ReadAll(f)
		if err != nil {
			return config.Config{}, errors.Wrap(err, "error reading the pubsub credentials file")
		}

		googlePubSubCredentialsJSON = b
	}

	return config.NewConfig(
		c.getenv(envListenAddress),
		environment,
		logLevel,
		c.getenv(envGooglePubsubProjectID),
		googlePubSubCredentialsJSON,
		c.getenv(envDatabasePath),
	)
}

func (c *EnvironmentConfigLoader) loadEnvironment() (config.Environment, error) {
	v := strings.ToUpper(c.getenv(envEnvironment))
	switch v {
	case "PRODUCTION":
		return config.EnvironmentProduction, nil
	case "DEVELOPMENT":
		return config.EnvironmentDevelopment, nil
	case "":
		return config.EnvironmentProduction, nil
	default:
		return config.Environment{}, fmt.Errorf("invalid environment requested '%s'", v)
	}
}

func (c *EnvironmentConfigLoader) loadLogLevel() (logging.Level, error) {
	v := strings.ToUpper(c.getenv(envLogLevel))
	switch v {
	case "TRACE":
		return logging.LevelTrace, nil
	case "DEBUG":
		return logging.LevelDebug, nil
	case "ERROR":
		return logging.LevelError, nil
	case "DISABLED":
		return logging.LevelDisabled, nil
	case "":
		return logging.LevelDebug, nil
	default:
		return 0, fmt.Errorf("invalid log level requested '%s'", v)
	}
}

func (c *EnvironmentConfigLoader) getenv(key string) string {
	return os.Getenv(fmt.Sprintf("%s_%s", envPrefix, key))
}
