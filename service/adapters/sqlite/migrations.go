package sqlite

import (
	"context"
	"database/sql"

	"github.com/boreq/errors"
	"github.com/planetary-social/nos-event-service/migrations"
)

func NewMigrations(fns *MigrationFns) (migrations.Migrations, error) {
	return migrations.NewMigrations([]migrations.Migration{
		migrations.MustNewMigration("initial", fns.Initial),
		migrations.MustNewMigration("create_pubsub_tables", fns.CreatePubsubTables),
	})
}

type MigrationFns struct {
	db     *sql.DB
	pubsub *PubSub
}

func NewMigrationFns(db *sql.DB, pubsub *PubSub) *MigrationFns {
	return &MigrationFns{db: db, pubsub: pubsub}
}

func (m *MigrationFns) Initial(ctx context.Context, state migrations.State, saveStateFunc migrations.SaveStateFunc) error {
	_, err := m.db.Exec(`
		CREATE TABLE IF NOT EXISTS events (
		    id INTEGER PRIMARY KEY,
			event_id TEXT UNIQUE,
			payload BLOB
		);`,
	)
	if err != nil {
		return errors.Wrap(err, "error creating the events table")
	}

	_, err = m.db.Exec(`
		CREATE TABLE IF NOT EXISTS events_to_relays (
			event_id INT,
			relay_id INT,
			PRIMARY KEY(event_id, relay_id),
			FOREIGN KEY(event_id) REFERENCES events(id),
			FOREIGN KEY(relay_id) REFERENCES relays(id)
		);`,
	)
	if err != nil {
		return errors.Wrap(err, "error creating the events_to_relays table")
	}

	_, err = m.db.Exec(`
		CREATE TABLE IF NOT EXISTS relays (
		    id INTEGER PRIMARY KEY,
			address TEXT UNIQUE
		);`,
	)
	if err != nil {
		return errors.Wrap(err, "error creating the relays table")
	}

	return nil
}

func (m *MigrationFns) CreatePubsubTables(ctx context.Context, state migrations.State, saveStateFunc migrations.SaveStateFunc) error {
	for _, query := range m.pubsub.InitializingQueries() {
		if _, err := m.db.Exec(query); err != nil {
			return errors.Wrapf(err, "error initializing pubsub")
		}
	}

	return nil
}
