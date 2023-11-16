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
	contactsExtractor      ContactsExtractor
	ExternalEventPublisher ExternalEventPublisher
	logger                 logging.Logger
	metrics                Metrics
}

func NewProcessSavedEventHandler(
	transactionProvider TransactionProvider,
	relaysExtractor RelaysExtractor,
	contactsExtractor ContactsExtractor,
	externalEventPublisher ExternalEventPublisher,
	logger logging.Logger,
	metrics Metrics,
) *ProcessSavedEventHandler {
	return &ProcessSavedEventHandler{
		transactionProvider:    transactionProvider,
		relaysExtractor:        relaysExtractor,
		contactsExtractor:      contactsExtractor,
		ExternalEventPublisher: externalEventPublisher,
		logger:                 logger.New("processSavedEventHandler"),
		metrics:                metrics,
	}
}

func (h *ProcessSavedEventHandler) Handle(ctx context.Context, cmd ProcessSavedEvent) (err error) {
	defer h.metrics.StartApplicationCall("processSavedEvent").End(&err)

	event, err := h.loadEvent(ctx, cmd.id)
	if err != nil {
		return errors.Wrap(err, "error loading the event")
	}

	if err := h.saveRelaysAndContacts(ctx, event); err != nil {
		return errors.Wrap(err, "error saving relays and contacts")
	}

	if err := h.ExternalEventPublisher.PublishNewEventReceived(ctx, event); err != nil {
		return errors.Wrap(err, "error publishing the external event")
	}

	return nil
}

func (h *ProcessSavedEventHandler) loadEvent(ctx context.Context, eventId domain.EventId) (domain.Event, error) {
	var event domain.Event
	if err := h.transactionProvider.Transact(ctx, func(ctx context.Context, adapters Adapters) error {
		tmp, err := adapters.Events.Get(ctx, eventId)
		if err != nil {
			return errors.Wrap(err, "error loading the event")
		}
		event = tmp
		return nil
	}); err != nil {
		return domain.Event{}, errors.Wrap(err, "transaction error")
	}
	return event, nil
}

func (h *ProcessSavedEventHandler) saveRelaysAndContacts(ctx context.Context, event domain.Event) error {
	maybeRelayAddresses, err := h.relaysExtractor.Extract(event)
	if err != nil {
		return errors.Wrap(err, "error extracting relay addresses")
	}

	contacts, contactsFound, err := h.extractContacts(event)
	if err != nil {
		return errors.Wrap(err, "error extracting contacts")
	}

	// todo remove logging
	if len(maybeRelayAddresses) == 0 && !(event.Kind() == domain.EventKindContacts && event.Content() == "") {
		h.logger.Debug().WithField("event", event.String()).WithField("addresses", maybeRelayAddresses).Message("addresses")
	}

	if err := h.transactionProvider.Transact(ctx, func(ctx context.Context, adapters Adapters) error {
		for _, maybeRelayAddress := range maybeRelayAddresses {
			if err := adapters.Relays.Save(ctx, event.Id(), maybeRelayAddress); err != nil {
				return errors.Wrap(err, "error saving a relay address")
			}
		}

		if contactsFound {
			shouldReplaceContacts, err := h.shouldReplaceContacts(ctx, adapters, event)
			if err != nil {
				return errors.Wrap(err, "error checking if contacts should be replaced")
			}

			if shouldReplaceContacts {
				if err := adapters.Contacts.SetContacts(ctx, event, contacts); err != nil {
					return errors.Wrap(err, "error setting new contacts")
				}
			}
		}

		return nil
	}); err != nil {
		return errors.Wrap(err, "transaction error")
	}

	return nil
}

func (h *ProcessSavedEventHandler) extractContacts(event domain.Event) ([]domain.PublicKey, bool, error) {
	contacts, err := h.contactsExtractor.Extract(event)
	if err != nil {
		if errors.Is(err, domain.ErrNotContactsEvent) {
			return nil, false, nil
		}
		return nil, false, errors.Wrap(err, "error calling contacts extactor")
	}
	return contacts, true, nil
}

func (h *ProcessSavedEventHandler) shouldReplaceContacts(ctx context.Context, adapters Adapters, newEvent domain.Event) (bool, error) {
	oldEvent, err := adapters.Contacts.GetCurrentContactsEvent(ctx, newEvent.PubKey())
	if err != nil {
		if errors.Is(err, ErrNoContactsEvent) {
			return true, nil
		}
		return false, errors.Wrap(err, "error getting current contacts event")
	}

	return domain.ShouldReplaceContactsEvent(oldEvent, newEvent)
}
