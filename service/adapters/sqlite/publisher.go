package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/boreq/errors"
	"github.com/oklog/ulid/v2"
	"github.com/planetary-social/nos-event-service/service/domain"
)

const EventSavedTopic = "event_saved"

type Publisher struct {
	pubsub *PubSub
	tx     *sql.Tx
}

func NewPublisher(pubsub *PubSub, tx *sql.Tx) *Publisher {
	return &Publisher{pubsub: pubsub, tx: tx}
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

	return p.pubsub.PublishTx(p.tx, EventSavedTopic, msg)
}

type EventSavedEventTransport struct {
	EventID string `json:"eventID"`
}
