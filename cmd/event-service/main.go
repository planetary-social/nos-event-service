package main

import (
	"context"
	"fmt"
	"os"
	"runtime"

	"github.com/boreq/errors"
	"github.com/grafana/pyroscope-go"
	"github.com/planetary-social/nos-event-service/cmd/event-service/di"
	configadapters "github.com/planetary-social/nos-event-service/service/adapters/config"
	"github.com/planetary-social/nos-event-service/service/config"
)

func main() {
	if err := run(); err != nil {
		fmt.Printf("error: %s\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()

	conf, err := configadapters.NewEnvironmentConfigLoader().Load()
	if err != nil {
		return errors.Wrap(err, "error creating a config")
	}

	if err := setupPyroscope(conf); err != nil {
		return errors.Wrap(err, "error setting up pyroscope")
	}

	service, cleanup, err := di.BuildService(ctx, conf)
	if err != nil {
		return errors.Wrap(err, "error building a service")
	}
	defer cleanup()

	if err := service.ExecuteMigrations(ctx); err != nil {
		return errors.Wrap(err, "error executing migrations")
	}

	return service.Run(ctx)
}

func setupPyroscope(conf config.Config) error {
	if conf.PyroscopeServerAddress() == "" {
		return nil
	}

	runtime.SetMutexProfileFraction(5)
	runtime.SetBlockProfileRate(5)

	_, err := pyroscope.Start(pyroscope.Config{
		ApplicationName:   conf.PyroscopeApplicationName(),
		ServerAddress:     conf.PyroscopeServerAddress(),
		Logger:            nil,
		BasicAuthUser:     conf.PyroscopeBasicAuthUser(),
		BasicAuthPassword: conf.PyroscopeBasicAuthPassword(),
		ProfileTypes: []pyroscope.ProfileType{
			pyroscope.ProfileCPU,
			pyroscope.ProfileAllocObjects,
			pyroscope.ProfileAllocSpace,
			pyroscope.ProfileInuseObjects,
			pyroscope.ProfileInuseSpace,
			pyroscope.ProfileGoroutines,
			pyroscope.ProfileMutexCount,
			pyroscope.ProfileMutexDuration,
			pyroscope.ProfileBlockCount,
			pyroscope.ProfileBlockDuration,
		},
	})
	if err != nil {
		return errors.Wrap(err, "error starting profiling")
	}

	return nil
}
