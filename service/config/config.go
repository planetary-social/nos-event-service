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

	pyroscopeApplicationName   string
	pyroscopeServerAddress     string
	pyroscopeBasicAuthUser     string
	pyroscopeBasicAuthPassword string
}

func NewConfig(
	listenAddress string,
	environment Environment,
	logLevel logging.Level,
	googlePubSubProjectID string,
	googlePubSubCredentialsJSON []byte,
	databasePath string,
	pyroscopeApplicationName string,
	pyroscopeServerAddress string,
	pyroscopeBasicAuthUser string,
	pyroscopeBasicAuthPassword string,
) (Config, error) {
	c := Config{
		listenAddress:               listenAddress,
		environment:                 environment,
		logLevel:                    logLevel,
		googlePubSubProjectID:       googlePubSubProjectID,
		googlePubSubCredentialsJSON: googlePubSubCredentialsJSON,
		databasePath:                databasePath,
		pyroscopeApplicationName:    pyroscopeApplicationName,
		pyroscopeServerAddress:      pyroscopeServerAddress,
		pyroscopeBasicAuthUser:      pyroscopeBasicAuthUser,
		pyroscopeBasicAuthPassword:  pyroscopeBasicAuthPassword,
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

func (c *Config) PyroscopeApplicationName() string {
	return c.pyroscopeApplicationName
}

func (c *Config) PyroscopeServerAddress() string {
	return c.pyroscopeServerAddress
}

func (c *Config) PyroscopeBasicAuthUser() string {
	return c.pyroscopeBasicAuthUser
}

func (c *Config) PyroscopeBasicAuthPassword() string {
	return c.pyroscopeBasicAuthPassword
}

func (c *Config) setDefaults() {
	if c.listenAddress == "" {
		c.listenAddress = ":8008"
	}

	if c.pyroscopeApplicationName == "" {
		c.pyroscopeApplicationName = "events.nos.social"
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

	if c.pyroscopeApplicationName == "" {
		return errors.New("missing pyroscope application name")
	}

	return nil
}
