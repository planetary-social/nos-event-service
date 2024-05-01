package sqlitepubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/boreq/errors"
	"github.com/planetary-social/nos-event-service/internal/logging"
	"github.com/planetary-social/nos-event-service/service/adapters/sqlite"
	"github.com/planetary-social/nos-event-service/service/app"
	"github.com/planetary-social/nos-event-service/service/domain"
)

const backPressureThreshold = 20000

type ProcessSavedEventHandler interface {
	Handle(ctx context.Context, cmd app.ProcessSavedEvent) (err error)
	NotifyBackPressure()
	ResolveBackPressure()
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

	//Periodically check for backpressure, if detected, send signal to sql queue
	//publisher to disconnect from relays for a while
	go func() {
		for {
			queueSize, err := s.subscriber.EventSavedQueueLength(ctx)
			if err != nil {
				s.logger.Error().WithError(err).Message("error getting queue length")
			}

			if queueSize > backPressureThreshold {
				s.logger.Debug().Message(
					fmt.Sprintf("Queue size %d > %d. Sending backpressure signal to slow down", queueSize, backPressureThreshold),
				)
				s.handler.NotifyBackPressure()
			} else if queueSize < backPressureThreshold/2 {
				s.handler.ResolveBackPressure()
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(30 * time.Second):
				continue
			}
		}
	}()

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

	eventID, err := domain.NewEventIdFromHex(transport.EventID)
	if err != nil {
		return errors.Wrap(err, "error creating an account id")
	}

	cmd := app.NewProcessSavedEvent(eventID)

	if err := s.handler.Handle(ctx, cmd); err != nil {
		return errors.Wrap(err, "error calling the handler")
	}

	return nil
}
