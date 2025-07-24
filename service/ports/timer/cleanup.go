package timer

import (
	"context"
	"time"

	"github.com/planetary-social/nos-event-service/internal/logging"
	"github.com/planetary-social/nos-event-service/service/app"
)

const cleanupEvery = 1 * time.Hour

type Cleanup struct {
	app    app.Application
	logger logging.Logger
}

func NewCleanup(app app.Application, logger logging.Logger) *Cleanup {
	return &Cleanup{
		app:    app,
		logger: logger.New("cleanup"),
	}
}

func (c *Cleanup) Run(ctx context.Context) error {
	// Run immediately on startup
	if err := c.cleanup(ctx); err != nil {
		c.logger.Error().WithError(err).Message("error running initial cleanup")
	}

	for {
		select {
		case <-time.After(cleanupEvery):
			if err := c.cleanup(ctx); err != nil {
				c.logger.Error().WithError(err).Message("error triggering cleanup handler")
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (c *Cleanup) cleanup(ctx context.Context) error {
	return c.app.CleanupProcessedEvents.Handle(ctx, app.NewCleanupProcessedEvents())
}
