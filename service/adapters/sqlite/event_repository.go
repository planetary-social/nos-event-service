package sqlite

import (
	"context"
	"database/sql"

	"github.com/boreq/errors"
	"github.com/planetary-social/nos-event-service/service/app"
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

func (r *EventRepository) Count(ctx context.Context) (int, error) {
	row := r.tx.QueryRow(`SELECT COUNT(*) FROM events`)

	var count int
	if err := row.Scan(&count); err != nil {
		return 0, errors.Wrap(err, "error scanning")
	}

	return count, nil
}

func (r *EventRepository) Exists(ctx context.Context, eventID domain.EventId) (bool, error) {
	result := r.tx.QueryRow(`
	SELECT event_id
	FROM events
	WHERE event_id=$1`,
		eventID.Hex(),
	)

	var tmp string
	if err := result.Scan(&tmp); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, errors.Wrap(err, "error scanning")
	}
	return true, nil
}

func (r *EventRepository) List(ctx context.Context, after *domain.EventId, limit int) ([]domain.Event, error) {
	rows, err := r.listQuery(after, limit)
	if err != nil {
		return nil, errors.Wrap(err, "error querying")
	}

	return r.readEvents(rows)
}

func (r *EventRepository) listQuery(after *domain.EventId, limit int) (*sql.Rows, error) {
	if after != nil {
		return r.tx.Query(`
	SELECT payload
	FROM events
	WHERE event_id > $1
	ORDER BY event_id
	LIMIT $2`,
			after.Hex(), limit,
		)
	}

	return r.tx.Query(`
	SELECT payload
	FROM events
	ORDER BY event_id
	LIMIT $1`,
		limit,
	)
}

func (r *EventRepository) readEvents(rows *sql.Rows) ([]domain.Event, error) {
	var events []domain.Event
	for rows.Next() {
		event, err := r.readEvent(rows)
		if err != nil {
			return nil, errors.Wrap(err, "error reading an event")
		}
		events = append(events, event)
	}
	return events, nil
}

func (m *EventRepository) readEvent(scanner scanner) (domain.Event, error) {
	var payload []byte

	if err := scanner.Scan(&payload); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Event{}, app.ErrEventNotFound
		}
		return domain.Event{}, errors.Wrap(err, "error reading the row")
	}

	event, err := domain.NewEventFromRaw(payload)
	if err != nil {
		return domain.Event{}, errors.Wrap(err, "error creating an event")
	}

	return event, nil
}

func (r *EventRepository) Delete(ctx context.Context, eventID domain.EventId) error {
	_, err := r.tx.ExecContext(ctx, `
	DELETE FROM events
	WHERE event_id=$1`,
		eventID.Hex(),
	)
	if err != nil {
		return errors.Wrap(err, "error executing the delete query")
	}
	return nil
}
