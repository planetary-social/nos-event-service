package app

import (
	"context"
	"time"

	"github.com/boreq/errors"
	"github.com/planetary-social/nos-event-service/internal/logging"
)

const (
	// Events are retained for 1 hour after processing
	processedEventRetentionPeriod = 1 * time.Hour
)

type CleanupProcessedEvents struct{}

func NewCleanupProcessedEvents() CleanupProcessedEvents {
	return CleanupProcessedEvents{}
}

type CleanupProcessedEventsHandler struct {
	transactionProvider TransactionProvider
	logger              logging.Logger
	metrics             Metrics
}

func NewCleanupProcessedEventsHandler(
	transactionProvider TransactionProvider,
	logger logging.Logger,
	metrics Metrics,
) *CleanupProcessedEventsHandler {
	return &CleanupProcessedEventsHandler{
		transactionProvider: transactionProvider,
		logger:              logger.New("cleanupProcessedEventsHandler"),
		metrics:             metrics,
	}
}

func (h *CleanupProcessedEventsHandler) Handle(ctx context.Context, cmd CleanupProcessedEvents) (err error) {
	defer h.metrics.StartApplicationCall("cleanupProcessedEvents").End(&err)

	ctx, cancel := context.WithTimeout(ctx, applicationHandlerTimeout)
	defer cancel()

	cutoffTime := time.Now().Add(-processedEventRetentionPeriod)

	var deletedCount int
	if err := h.transactionProvider.Transact(ctx, func(ctx context.Context, adapters Adapters) error {
		count, err := adapters.Events.DeleteProcessedEventsBefore(ctx, cutoffTime)
		if err != nil {
			return errors.Wrap(err, "error deleting processed events")
		}
		deletedCount = count
		return nil
	}); err != nil {
		return errors.Wrap(err, "transaction error")
	}

	if deletedCount > 0 {
		h.logger.
			Debug().
			WithField("count", deletedCount).
			WithField("cutoffTime", cutoffTime).
			Message("deleted processed events")
	}

	return nil
}
