package app

import (
	"context"

	"github.com/boreq/errors"
	"github.com/planetary-social/nos-event-service/internal/logging"
	"github.com/planetary-social/nos-event-service/service/domain"
)

type ProcessSavedEvent struct {
	id domain.EventId
}

func NewProcessSavedEvent(id domain.EventId) ProcessSavedEvent {
	return ProcessSavedEvent{id: id}
}

type ProcessSavedEventHandler struct {
	transactionProvider    TransactionProvider
	relaysExtractor        RelaysExtractor
	ExternalEventPublisher ExternalEventPublisher
	logger                 logging.Logger
	metrics                Metrics
}

func NewProcessSavedEventHandler(
	transactionProvider TransactionProvider,
	relaysExtractor RelaysExtractor,
	externalEventPublisher ExternalEventPublisher,
	logger logging.Logger,
	metrics Metrics,
) *ProcessSavedEventHandler {
	return &ProcessSavedEventHandler{
		transactionProvider:    transactionProvider,
		relaysExtractor:        relaysExtractor,
		ExternalEventPublisher: externalEventPublisher,
		logger:                 logger.New("processSavedEventHandler"),
		metrics:                metrics,
	}
}

func (h *ProcessSavedEventHandler) Handle(ctx context.Context, cmd ProcessSavedEvent) (err error) {
	defer h.metrics.StartApplicationCall("processSavedEvent").End(&err)

	var event domain.Event
	if err := h.transactionProvider.Transact(ctx, func(ctx context.Context, adapters Adapters) error {
		tmp, err := adapters.Events.Get(ctx, cmd.id)
		if err != nil {
			return errors.Wrap(err, "error loading the event")
		}
		event = tmp
		return nil
	}); err != nil {
		return errors.Wrap(err, "transaction error")
	}

	maybeRelayAddresses, err := h.relaysExtractor.Extract(event)
	if err != nil {
		return errors.Wrap(err, "error extracting relay addresses from event")
	}

	if len(maybeRelayAddresses) == 0 && !(event.Kind() == domain.EventKindContacts && event.Content() == "") {
		h.logger.Debug().WithField("event", event.String()).WithField("addresses", maybeRelayAddresses).Message("addresses")
	}

	if err := h.transactionProvider.Transact(ctx, func(ctx context.Context, adapters Adapters) error {
		for _, maybeRelayAddress := range maybeRelayAddresses {
			if err := adapters.Relays.Save(ctx, cmd.id, maybeRelayAddress); err != nil {
				return errors.Wrap(err, "error saving a relay address")
			}
		}

		return nil
	}); err != nil {
		return errors.Wrap(err, "transaction error")
	}

	if err := h.ExternalEventPublisher.PublishNewEventReceived(ctx, event); err != nil {
		return errors.Wrap(err, "error publishing the external event")
	}

	return nil
}
