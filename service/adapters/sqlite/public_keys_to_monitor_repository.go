package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/boreq/errors"
	"github.com/planetary-social/nos-event-service/service/domain"
)

type PublicKeysToMonitorRepository struct {
	tx *sql.Tx
}

func NewPublicKeysToMonitorRepository(tx *sql.Tx) (*PublicKeysToMonitorRepository, error) {
	return &PublicKeysToMonitorRepository{
		tx: tx,
	}, nil
}

func (r *PublicKeysToMonitorRepository) Save(ctx context.Context, publicKeyToMonitor domain.PublicKeyToMonitor) error {
	_, err := r.tx.Exec(`
	INSERT INTO public_keys_to_monitor(public_key, created_at, updated_at)
	VALUES($1, $2, $3)
	ON CONFLICT(public_key) DO UPDATE SET
	  updated_at=excluded.updated_at`,
		publicKeyToMonitor.PublicKey().Hex(),
		publicKeyToMonitor.CreatedAt().Unix(),
		publicKeyToMonitor.UpdatedAt().Unix(),
	)
	if err != nil {
		return errors.Wrap(err, "error executing the insert query")
	}

	return nil
}

func (r *PublicKeysToMonitorRepository) List(ctx context.Context) ([]domain.PublicKeyToMonitor, error) {
	rows, err := r.tx.Query(`
		SELECT public_key, created_at, updated_at
		FROM public_keys_to_monitor`,
	)
	if err != nil {
		return nil, errors.Wrap(err, "query error")
	}

	var result []domain.PublicKeyToMonitor
	for rows.Next() {
		var publicKeyTmp string
		var createdAtTmp int64
		var updatedAtTmp int64
		if err := rows.Scan(&publicKeyTmp, &createdAtTmp, &updatedAtTmp); err != nil {
			return nil, errors.Wrap(err, "scan error")
		}

		publicKey, err := domain.NewPublicKeyFromHex(publicKeyTmp)
		if err != nil {
			return nil, errors.Wrap(err, "error creating a public key")
		}

		createdAt := time.Unix(createdAtTmp, 0)
		updatedAt := time.Unix(updatedAtTmp, 0)

		publicKeyToMonitor, err := domain.NewPublicKeyToMonitor(publicKey, createdAt, updatedAt)
		if err != nil {
			return nil, errors.Wrap(err, "error creating a public key to monitor")
		}

		result = append(result, publicKeyToMonitor)
	}

	return result, nil
}
