package relays

import (
	"context"

	"github.com/boreq/errors"
	"github.com/planetary-social/nos-event-service/service/domain"
)

var ErrEventReplaced = errors.New("relay has a newer event which replaced this event")

type EventSender struct {
	connections *RelayConnections
}

func NewEventSender(connections *RelayConnections) *EventSender {
	return &EventSender{connections: connections}
}

func (s *EventSender) SendEvent(ctx context.Context, address domain.RelayAddress, event domain.Event) error {
	if err := s.connections.SendEvent(ctx, address, event); err != nil {
		err = s.maybeConvertError(err)
		return errors.Wrap(err, "error sending event to relay")
	}
	return nil
}

func (s *EventSender) maybeConvertError(err error) error {
	var okResponseErr OKResponseError
	if errors.As(err, &okResponseErr) {
		switch okResponseErr.Reason() {
		case "replaced: have newer event":
			return ErrEventReplaced
		default:
			return err
		}
	}
	return err
}
