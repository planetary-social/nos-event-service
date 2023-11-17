package sqlite

import (
	"context"
	"database/sql"

	"github.com/boreq/errors"
	"github.com/planetary-social/nos-event-service/service/app"
	"github.com/planetary-social/nos-event-service/service/domain"
)

type ContactRepository struct {
	tx *sql.Tx
}

func NewContactRepository(tx *sql.Tx) *ContactRepository {
	return &ContactRepository{tx: tx}
}

func (r *ContactRepository) SetContacts(ctx context.Context, event domain.Event, contacts []domain.PublicKey) error {
	_, err := r.tx.Exec(`
	INSERT OR IGNORE INTO public_keys(public_key)
	VALUES($1)`,
		event.PubKey().Hex(),
	)
	if err != nil {
		return errors.Wrap(err, "error inserting the public key")
	}

	row := r.tx.QueryRow(`
	SELECT id FROM public_keys
	WHERE public_key=$1`,
		event.PubKey().Hex(),
	)
	var dbFollowerId int
	if err := row.Scan(&dbFollowerId); err != nil {
		return errors.Wrap(err, "error getting the public key id")

	}

	row = r.tx.QueryRow(`
	SELECT id FROM events
	WHERE event_id=$1`,
		event.Id().Hex(),
	)
	var dbEventId int
	if err := row.Scan(&dbEventId); err != nil {
		return errors.Wrap(err, "error getting the event id")
	}

	_, err = r.tx.Exec(`
	INSERT INTO contacts_events(follower_id, event_id)
	VALUES ($1, $2)
	ON CONFLICT(follower_id) DO UPDATE SET
	  event_id=excluded.event_id`,
		dbFollowerId,
		dbEventId,
	)
	if err != nil {
		return errors.Wrap(err, "error inserting the contacts event")
	}

	_, err = r.tx.Exec(`
	DELETE FROM contacts_followees
	WHERE follower_id=$1`,
		dbFollowerId,
	)
	if err != nil {
		return errors.Wrap(err, "error removing current followees")
	}

	for _, contact := range contacts {
		_, err := r.tx.Exec(`
	INSERT OR IGNORE INTO public_keys(public_key)
	VALUES($1)`,
			contact.Hex(),
		)
		if err != nil {
			return errors.Wrap(err, "error inserting the contact's public key")
		}

		row := r.tx.QueryRow(`
	SELECT id FROM public_keys
	WHERE public_key=$1`,
			contact.Hex(),
		)
		var dbFolloweeId int
		if err := row.Scan(&dbFolloweeId); err != nil {
			return errors.Wrap(err, "error getting the contact's public key id")

		}

		_, err = r.tx.Exec(`
	INSERT INTO contacts_followees(follower_id, followee_id)
	VALUES ($1, $2)`,
			dbFollowerId,
			dbFolloweeId,
		)
		if err != nil {
			return errors.Wrap(err, "error inserting the followee")
		}
	}

	return nil
}

func (r *ContactRepository) GetCurrentContactsEvent(ctx context.Context, author domain.PublicKey) (domain.Event, error) {
	row := r.tx.QueryRow(`
		SELECT payload
		FROM events E
		LEFT JOIN  contacts_events CE ON CE.event_id=E.id
		LEFT JOIN  public_keys PK ON PK.id=CE.follower_id
		WHERE PK.public_key=$1`,
		author.Hex(),
	)

	var payload []byte
	if err := row.Scan(&payload); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Event{}, app.ErrNoContactsEvent
		}
		return domain.Event{}, errors.Wrap(err, "error querying")
	}

	return domain.NewEventFromRaw(payload)
}

func (r *ContactRepository) GetFollowees(ctx context.Context, publicKey domain.PublicKey) ([]domain.PublicKey, error) {
	rows, err := r.tx.Query(`
		SELECT PK2.public_key
		FROM contacts_followees CF
		LEFT JOIN  public_keys PK1 ON PK1.id=CF.follower_id
		LEFT JOIN  public_keys PK2 ON PK2.id=CF.followee_id
		WHERE PK1.public_key=$1`,
		publicKey.Hex(),
	)
	if err != nil {
		return nil, errors.Wrap(err, "error querying")
	}

	var result []domain.PublicKey
	for rows.Next() {
		var publicKeyTmp string
		if err := rows.Scan(&publicKeyTmp); err != nil {
			return nil, errors.Wrap(err, "scan err")
		}

		publicKey, err := domain.NewPublicKeyFromHex(publicKeyTmp)
		if err != nil {
			return nil, errors.Wrap(err, "error creating a public key")
		}
		result = append(result, publicKey)
	}

	return result, nil
}
