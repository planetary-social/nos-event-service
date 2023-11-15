package sqlitepubsub

import (
	"context"
	"encoding/json"

	"github.com/boreq/errors"
	"github.com/planetary-social/nos-event-service/internal/logging"
	"github.com/planetary-social/nos-event-service/service/adapters/sqlite"
	"github.com/planetary-social/nos-event-service/service/app"
	"github.com/planetary-social/nos-event-service/service/domain"
)

type ProcessSavedEventHandler interface {
	Handle(ctx context.Context, cmd app.ProcessSavedEvent) (err error)
}

type EventSavedEventSubscriber struct {
	handler    ProcessSavedEventHandler
	subscriber *sqlite.Subscriber
	logger     logging.Logger
	metrics    app.Metrics
}

func NewEventSavedEventSubscriber(
	handler ProcessSavedEventHandler,
	subscriber *sqlite.Subscriber,
	logger logging.Logger,
	metrics app.Metrics,
) *EventSavedEventSubscriber {
	return &EventSavedEventSubscriber{
		handler:    handler,
		subscriber: subscriber,
		logger:     logger.New("eventSavedEventSubscriber"),
		metrics:    metrics,
	}
}
func (s *EventSavedEventSubscriber) Run(ctx context.Context) error {
	for msg := range s.subscriber.SubscribeToEventSaved(ctx) {
		if err := s.handleMessage(ctx, msg); err != nil {
			s.logger.Error().WithError(err).Message("error handling a message")
			if err := msg.Nack(); err != nil {
				return errors.Wrap(err, "error nacking a message")
			}
		} else {
			if err := msg.Ack(); err != nil {
				return errors.Wrap(err, "error acking a message")
			}
		}
	}

	return errors.New("channel closed")
}

func (s *EventSavedEventSubscriber) handleMessage(ctx context.Context, msg *sqlite.ReceivedMessage) error {
	var transport sqlite.EventSavedEventTransport
	if err := json.Unmarshal(msg.Payload(), &transport); err != nil {
		return errors.Wrap(err, "error unmarshaling")
	}

	eventID, err := domain.NewEventId(transport.EventID)
	if err != nil {
		return errors.Wrap(err, "error creating an account id")
	}

	cmd := app.NewProcessSavedEvent(eventID)

	if err := s.handler.Handle(ctx, cmd); err != nil {
		return errors.Wrap(err, "error calling the handler")
	}

	return nil
}
