package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/boreq/errors"
	"github.com/planetary-social/nos-event-service/service/app"
)

type Subscriber struct {
	pubsub *PubSub
	db     *sql.DB
}

func NewSubscriber(
	pubsub *PubSub,
	db *sql.DB,
) *Subscriber {
	return &Subscriber{
		pubsub: pubsub,
		db:     db,
	}
}

func (s *Subscriber) SubscribeToEventSaved(ctx context.Context) <-chan *ReceivedMessage {
	return s.pubsub.Subscribe(ctx, EventSavedTopic)
}

func (s *Subscriber) EventSavedQueueLength(ctx context.Context) (int, error) {
	return s.pubsub.QueueLength(ctx, EventSavedTopic)
}

func (s *Subscriber) EventSavedOldestMessageAge(ctx context.Context) (time.Duration, error) {
	age, err := s.pubsub.OldestMessageAge(ctx, EventSavedTopic)
	if err != nil {
		if errors.Is(err, ErrQueueEmpty) {
			return 0, app.ErrEventSavedQueueEmpty
		}
		return 0, errors.Wrap(err, "error checking queue length")
	}
	return age, nil
}
