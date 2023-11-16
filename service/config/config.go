package config

import (
	"fmt"

	"github.com/boreq/errors"
	"github.com/planetary-social/nos-event-service/internal/logging"
)

type Environment struct {
	s string
}

var (
	EnvironmentProduction  = Environment{"production"}
	EnvironmentDevelopment = Environment{"development"}
)

type Config struct {
	listenAddress string

	environment Environment
	logLevel    logging.Level

	googlePubSubProjectID       string
	googlePubSubCredentialsJSON []byte

	databasePath string
}

func NewConfig(
	listenAddress string,
	environment Environment,
	logLevel logging.Level,
	googlePubSubProjectID string,
	googlePubSubCredentialsJSON []byte,
	databasePath string,
) (Config, error) {
	c := Config{
		listenAddress:               listenAddress,
		environment:                 environment,
		logLevel:                    logLevel,
		googlePubSubProjectID:       googlePubSubProjectID,
		googlePubSubCredentialsJSON: googlePubSubCredentialsJSON,
		databasePath:                databasePath,
	}

	c.setDefaults()
	if err := c.validate(); err != nil {
		return Config{}, errors.Wrap(err, "invalid config")
	}

	return c, nil
}

func (c *Config) ListenAddress() string {
	return c.listenAddress
}

func (c *Config) Environment() Environment {
	return c.environment
}

func (c *Config) LogLevel() logging.Level {
	return c.logLevel
}

func (c *Config) GooglePubSubProjectID() string {
	return c.googlePubSubProjectID
}

func (c *Config) GooglePubSubCredentialsJSON() []byte {
	return c.googlePubSubCredentialsJSON
}

func (c *Config) DatabasePath() string {
	return c.databasePath
}

func (c *Config) setDefaults() {
	if c.listenAddress == "" {
		c.listenAddress = ":8008"
	}
}

func (c *Config) validate() error {
	if c.listenAddress == "" {
		return errors.New("missing listen address")
	}

	switch c.environment {
	case EnvironmentProduction:
	case EnvironmentDevelopment:
	default:
		return fmt.Errorf("unknown environment '%+v'", c.environment)
	}

	if c.environment != EnvironmentDevelopment {
		if c.googlePubSubProjectID == "" {
			return errors.New("missing google pub sub project id")
		}

		if len(c.googlePubSubCredentialsJSON) == 0 {
			return errors.New("missing google pub sub credentials json")
		}
	}

	if c.databasePath == "" {
		return errors.New("missing database path")
	}

	return nil
}
