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

	return nil
}
