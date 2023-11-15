package sqlite

import (
	"context"
	"database/sql"

	"github.com/boreq/errors"
	"github.com/planetary-social/nos-event-service/service/domain"
)

type EventRepository struct {
	tx *sql.Tx
}

func NewEventRepository(tx *sql.Tx) (*EventRepository, error) {
	return &EventRepository{
		tx: tx,
	}, nil
}

func (r *EventRepository) Save(ctx context.Context, event domain.Event) error {
	_, err := r.tx.Exec(`
	INSERT INTO events(event_id, payload)
	VALUES($1, $2)
	ON CONFLICT(event_id) DO UPDATE SET
	  payload=excluded.payload`,
		event.Id().Hex(),
		event.Raw(),
	)
	if err != nil {
		return errors.Wrap(err, "error executing the insert query")
	}

	return nil
}

func (r *EventRepository) Get(ctx context.Context, eventID domain.EventId) (domain.Event, error) {
	result := r.tx.QueryRow(`
	SELECT payload
	FROM events
	WHERE event_id=$1`,
		eventID.Hex(),
	)

	return r.readEvent(result)
}

func (m *EventRepository) readEvent(result *sql.Row) (domain.Event, error) {
	var payload []byte

	if err := result.Scan(&payload); err != nil {
		return domain.Event{}, errors.Wrap(err, "error reading the row")
	}

	event, err := domain.NewEventFromRaw(payload)
	if err != nil {
		return domain.Event{}, errors.Wrap(err, "error creating an event")
	}

	return event, nil
}
