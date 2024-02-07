package relays

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/boreq/errors"
	"github.com/planetary-social/nos-event-service/service/domain"
)

var ErrEventReplaced = errors.New("relay has a newer event which replaced this event")
var ErrEventInvalid = errors.New("invalid event from relay")

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

func say(text string) {
	cmd := exec.Command("say", text)
	if err := cmd.Run(); err != nil {
		// Handle the error if the command fails
		fmt.Println("Failed to execute say command:", err)
	}
}
func (s *EventSender) maybeConvertError(err error) error {
	var okResponseErr OKResponseError
	if !errors.As(err, &okResponseErr) {
		return err
	}

	reason := okResponseErr.Reason()

	switch {
	case reason == "replaced: have newer event":
		return ErrEventReplaced
	case strings.HasPrefix(reason, "invalid: "):
		return ErrEventInvalid
	default:
		return err
	}
}
