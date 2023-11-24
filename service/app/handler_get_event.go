package app

import (
	"context"

	"github.com/boreq/errors"
	"github.com/planetary-social/nos-event-service/internal/logging"
	"github.com/planetary-social/nos-event-service/service/domain"
)

type GetEvent struct {
	id domain.EventId
}

func NewGetEvent(id domain.EventId) GetEvent {
	return GetEvent{id: id}
}

type GetEventHandler struct {
	transactionProvider TransactionProvider
	logger              logging.Logger
	metrics             Metrics
}

func NewGetEventHandler(
	transactionProvider TransactionProvider,
	logger logging.Logger,
	metrics Metrics,
) *GetEventHandler {
	return &GetEventHandler{
		transactionProvider: transactionProvider,
		logger:              logger.New("getEventHandler"),
		metrics:             metrics,
	}
}

func (h *GetEventHandler) Handle(ctx context.Context, cmd GetEvent) (event domain.Event, err error) {
	defer h.metrics.StartApplicationCall("getEvent").End(&err)

	if err := h.transactionProvider.Transact(ctx, func(ctx context.Context, adapters Adapters) error {
		tmp, err := adapters.Events.Get(ctx, cmd.id)
		if err != nil {
			return errors.Wrap(err, "error getting the event")
		}
		event = tmp
		return nil
	}); err != nil {
		return domain.Event{}, errors.Wrap(err, "transaction error")
	}

	return event, nil
}
