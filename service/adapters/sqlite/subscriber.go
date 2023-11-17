package sqlite

import (
	"context"
	"database/sql"
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
