package sqlite

import (
	"context"
	"encoding/json"

	"github.com/boreq/errors"
	"github.com/oklog/ulid/v2"
	"github.com/planetary-social/nos-event-service/service/domain"
)

const EventSavedTopic = "event_saved"

type Publisher struct {
	pubsub *TxPubSub
}

func NewPublisher(pubsub *TxPubSub) *Publisher {
	return &Publisher{pubsub: pubsub}
}

func (p *Publisher) PublishEventSaved(ctx context.Context, id domain.EventId) error {
	transport := EventSavedEventTransport{
		EventID: id.Hex(),
	}

	payload, err := json.Marshal(transport)
	if err != nil {
		return errors.Wrap(err, "error marshaling the transport type")
	}

	msg, err := NewMessage(ulid.Make().String(), payload)
	if err != nil {
		return errors.Wrap(err, "error creating a message")
	}

	return p.pubsub.PublishTx(EventSavedTopic, msg)
}

type EventSavedEventTransport struct {
	EventID string `json:"eventID"`
}
