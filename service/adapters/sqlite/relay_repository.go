package sqlite

import (
	"context"
	"database/sql"

	"github.com/boreq/errors"
	"github.com/planetary-social/nos-event-service/service/domain"
)

type RelayRepository struct {
	tx *sql.Tx
}

func NewRelayRepository(tx *sql.Tx) *RelayRepository {
	return &RelayRepository{tx: tx}
}

func (r *RelayRepository) Save(ctx context.Context, eventID domain.EventId, relayAddress domain.MaybeRelayAddress) error {
	_, err := r.tx.Exec(`
	INSERT OR IGNORE INTO relays(address)
	VALUES($1)`,
		relayAddress.String(),
	)
	if err != nil {
		return errors.Wrap(err, "error inserting the relay address")
	}

	row := r.tx.QueryRow(`
	SELECT id FROM relays
	WHERE address=$1`,
		relayAddress.String(),
	)
	var dbRelayId int
	if err := row.Scan(&dbRelayId); err != nil {
		return errors.Wrap(err, "error getting the relay id")

	}

	row = r.tx.QueryRow(`
	SELECT id FROM events
	WHERE event_id=$1`,
		eventID.Hex(),
	)
	var dbEventId int
	if err := row.Scan(&dbEventId); err != nil {
		return errors.Wrap(err, "error getting the event id")
	}

	_, err = r.tx.Exec(`
	INSERT OR IGNORE INTO events_to_relays(event_id, relay_id)
	VALUES($1, $2)`,
		dbEventId, dbRelayId,
	)
	if err != nil {
		return errors.Wrap(err, "error inserting the relationship")
	}

	return nil
}

func (r *RelayRepository) List(ctx context.Context) ([]domain.MaybeRelayAddress, error) {
	rows, err := r.tx.Query(`SELECT address FROM relays`)
	if err != nil {
		return nil, errors.Wrap(err, "error selecting addresses")
	}

	var result []domain.MaybeRelayAddress
	for rows.Next() {
		var addressString string
		if err := rows.Scan(&addressString); err != nil {
			return nil, errors.Wrap(err, "error scanning")
		}
		result = append(result, domain.NewMaybeRelayAddress(addressString))
	}

	return result, nil
}

func (r *RelayRepository) Count(ctx context.Context) (int, error) {
	row := r.tx.QueryRow(`SELECT COUNT(*) FROM relays`)

	var count int
	if err := row.Scan(&count); err != nil {
		return 0, errors.Wrap(err, "error scanning")
	}

	return count, nil
}
