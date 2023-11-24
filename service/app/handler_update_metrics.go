package app

import (
	"context"

	"github.com/boreq/errors"
	"github.com/planetary-social/nos-event-service/internal/logging"
)

type UpdateMetricsHandler struct {
	transactionProvider TransactionProvider
	subscriber          Subscriber
	logger              logging.Logger
	metrics             Metrics
}

func NewUpdateMetricsHandler(
	transactionProvider TransactionProvider,
	subscriber Subscriber,
	logger logging.Logger,
	metrics Metrics,
) *UpdateMetricsHandler {
	return &UpdateMetricsHandler{
		transactionProvider: transactionProvider,
		subscriber:          subscriber,
		logger:              logger.New("updateMetricsHandler"),
		metrics:             metrics,
	}
}

func (h *UpdateMetricsHandler) Handle(ctx context.Context) (err error) {
	defer h.metrics.StartApplicationCall("updateMetrics").End(&err)

	n, err := h.subscriber.EventSavedQueueLength(ctx)
	if err != nil {
		return errors.Wrap(err, "error reading queue length")
	}
	h.metrics.ReportQueueLength("eventSaved", n)

	age, err := h.subscriber.EventSavedOldestMessageAge(ctx)
	if err != nil {
		if errors.Is(err, ErrEventSavedQueueEmpty) {
			h.metrics.ReportQueueOldestMessageAge("eventSaved", 0)
		} else {
			return errors.Wrap(err, "error reading oldest message age")
		}
	} else {
		h.metrics.ReportQueueOldestMessageAge("eventSaved", age)
	}

	if err := h.transactionProvider.Transact(ctx, func(ctx context.Context, adapters Adapters) error {
		n, err := adapters.Relays.Count(ctx)
		if err != nil {
			return errors.Wrap(err, "error counting relay addresses")
		}
		h.metrics.ReportNumberOfStoredRelayAddresses(n)

		n, err = adapters.Events.Count(ctx)
		if err != nil {
			return errors.Wrap(err, "error counting events")
		}
		h.metrics.ReportNumberOfStoredEvents(n)

		return nil
	}); err != nil {
		return errors.Wrap(err, "transaction error")
	}

	return nil
}
