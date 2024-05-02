package app

import (
	"context"

	"github.com/boreq/errors"
	"github.com/planetary-social/nos-event-service/internal/logging"
	"github.com/planetary-social/nos-event-service/service/domain"
)

const getEventsLimit = 100

type GetEvents struct {
	after *domain.EventId
}

func NewGetEvents(after *domain.EventId) GetEvents {
	return GetEvents{after: after}
}

type GetEventsResult struct {
	events            []domain.Event
	thereIsMoreEvents bool
}

func NewGetEventsResult(events []domain.Event, thereIsMoreEvents bool) GetEventsResult {
	return GetEventsResult{
		events:            events,
		thereIsMoreEvents: thereIsMoreEvents,
	}
}

func (g GetEventsResult) Events() []domain.Event {
	return g.events
}

func (g GetEventsResult) ThereIsMoreEvents() bool {
	return g.thereIsMoreEvents
}

type GetEventsHandler struct {
	transactionProvider TransactionProvider
	logger              logging.Logger
	metrics             Metrics
}

func NewGetEventsHandler(
	transactionProvider TransactionProvider,
	logger logging.Logger,
	metrics Metrics,
) *GetEventsHandler {
	return &GetEventsHandler{
		transactionProvider: transactionProvider,
		logger:              logger.New("getEventsHandler"),
		metrics:             metrics,
	}
}

func (h *GetEventsHandler) Handle(ctx context.Context, cmd GetEvents) (result GetEventsResult, err error) {
	defer h.metrics.StartApplicationCall("getEvents").End(&err)

	ctx, cancel := context.WithTimeout(ctx, applicationHandlerTimeout)
	defer cancel()

	var events []domain.Event
	if err := h.transactionProvider.ReadOnly(ctx, func(ctx context.Context, adapters Adapters) error {
		tmp, err := adapters.Events.List(ctx, cmd.after, getEventsLimit+1)
		if err != nil {
			return errors.Wrap(err, "error getting the event")
		}
		events = tmp
		return nil
	}); err != nil {
		return GetEventsResult{}, errors.Wrap(err, "transaction error")
	}

	if len(events) > getEventsLimit {
		return NewGetEventsResult(events[:len(events)-1], true), nil
	}
	return NewGetEventsResult(events, false), nil
}
