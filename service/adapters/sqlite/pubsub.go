package sqlite

import (
	"context"
	"database/sql"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/boreq/errors"
	"github.com/planetary-social/nos-event-service/internal/logging"
)

type BackoffManager interface {
	// GetMessageErrorBackoff is used to backoff reprocessing of a single
	// specific message if its processing fails. The first time message
	// processing fails 1 is passed to this function, the second time 2 etc.
	GetMessageErrorBackoff(nackCount int) time.Duration

	// GetNoMessagesBackoff is used to backoff querying for new messages on the
	// queue. The first time in a row where the query returns no messages 1 is
	// passed to this function, then 2 is passed etc.
	GetNoMessagesBackoff(tick int) time.Duration
}

var ErrQueueEmpty = errors.New("queue is empty")

type PubsubTransactionProvider interface {
	Transact(context.Context, func(context.Context, *sql.Tx) error) error
}

type Message struct {
	uuid    string
	payload []byte
}

func NewMessage(uuid string, payload []byte) (Message, error) {
	if uuid == "" {
		return Message{}, errors.New("uuid can't be empty")
	}
	return Message{uuid: uuid, payload: payload}, nil
}

func (m Message) UUID() string {
	return m.uuid
}

func (m Message) Payload() []byte {
	return m.payload
}

type ReceivedMessage struct {
	Message

	lock   sync.Mutex
	state  receivedMessageState
	chAck  chan struct{}
	chNack chan struct{}
}

func NewReceivedMessage(message Message) *ReceivedMessage {
	return &ReceivedMessage{
		Message: message,
		state:   receivedMessageStateFresh,
		chAck:   make(chan struct{}),
		chNack:  make(chan struct{}),
	}
}

func (m *ReceivedMessage) Ack() error {
	m.lock.Lock()
	defer m.lock.Unlock()

	if m.state != receivedMessageStateFresh {
		return errors.New("message was already acked or nacked")
	}

	m.state = receivedMessageStateAcked
	close(m.chAck)
	return nil
}

func (m *ReceivedMessage) Nack() error {
	m.lock.Lock()
	defer m.lock.Unlock()

	if m.state != receivedMessageStateFresh {
		return errors.New("message was already acked or nacked")
	}

	m.state = receivedMessageStateNacked
	close(m.chNack)
	return nil
}

type receivedMessageState struct {
	s string
}

var (
	receivedMessageStateFresh  = receivedMessageState{"fresh"}
	receivedMessageStateAcked  = receivedMessageState{"acked"}
	receivedMessageStateNacked = receivedMessageState{"nacked"}
)

type PubSub struct {
	backoffManager      BackoffManager
	transactionProvider PubsubTransactionProvider
	logger              logging.Logger
}

func NewPubSub(
	transactionProvider PubsubTransactionProvider,
	logger logging.Logger,
) *PubSub {
	return &PubSub{
		backoffManager:      NewDefaultBackoffManager(),
		transactionProvider: transactionProvider,
		logger:              logger,
	}
}

func (p *PubSub) InitializingQueries() []string {
	return []string{`
		CREATE TABLE IF NOT EXISTS pubsub (
		topic TEXT NOT NULL,
		uuid VARCHAR(36) NOT NULL PRIMARY KEY,
		payload BLOB,
		created_at INTEGER NOT NULL,
		nack_count INTEGER NOT NULL,
		backoff_until INTEGER
		)`,
	}
}

func (p *PubSub) Publish(ctx context.Context, topic string, msg Message) error {
	if err := p.transactionProvider.Transact(ctx, func(ctx context.Context, tx *sql.Tx) error {
		txPubSub := NewTxPubSub(tx, p.logger)
		return txPubSub.PublishTx(topic, msg)
	}); err != nil {
		return errors.Wrap(err, "transaction error")
	}
	return nil
}

func (p *PubSub) Subscribe(ctx context.Context, topic string) <-chan *ReceivedMessage {
	ch := make(chan *ReceivedMessage)
	go p.subscribe(ctx, topic, ch)
	return ch
}

func (p *PubSub) QueueLength(ctx context.Context, topic string) (int, error) {
	var count int
	if err := p.transactionProvider.Transact(ctx, func(ctx context.Context, tx *sql.Tx) error {
		row := tx.QueryRow(
			"SELECT COUNT(*) FROM pubsub WHERE topic = ?",
			topic,
		)

		var tmp int
		if err := row.Scan(&tmp); err != nil {
			return errors.Wrap(err, "row scan error")
		}

		count = tmp
		return nil
	}); err != nil {
		return 0, errors.Wrap(err, "transaction error")
	}

	return count, nil
}

// OldestMessageAge returns ErrQueueEmpty if the queue is empty.
func (p *PubSub) OldestMessageAge(ctx context.Context, topic string) (time.Duration, error) {
	var age time.Duration
	if err := p.transactionProvider.Transact(ctx, func(ctx context.Context, tx *sql.Tx) error {
		row := tx.QueryRow(
			"SELECT created_at FROM pubsub WHERE topic = ? ORDER BY created_at ASC LIMIT 1",
			topic,
		)

		var tmp int64
		if err := row.Scan(&tmp); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrQueueEmpty
			}
			return errors.Wrap(err, "row scan error")
		}

		age = time.Since(time.Unix(tmp, 0))
		return nil
	}); err != nil {
		return 0, errors.Wrap(err, "transaction error")
	}

	return age, nil
}

func (p *PubSub) subscribe(ctx context.Context, topic string, ch chan *ReceivedMessage) {
	noMessagesCounter := 0

	for {
		msg, err := p.readMsg(ctx, topic)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				noMessagesCounter++
				backoff := p.backoffManager.GetNoMessagesBackoff(noMessagesCounter)

				p.logger.Trace().
					WithField("duration", backoff).
					Message("backing off reading messages")

				select {
				case <-time.After(backoff):
					continue
				case <-ctx.Done():
					return
				}
			}

			p.logger.Error().WithError(err).Message("error reading message")

			select {
			case <-time.After(1 * time.Second):
				continue
			case <-ctx.Done():
				return
			}
		}

		noMessagesCounter = 0
		receivedMsg := NewReceivedMessage(msg)

		select {
		case ch <- receivedMsg:
		case <-ctx.Done():
			return
		}

		select {
		case <-receivedMsg.chAck:
			if err := p.ack(ctx, receivedMsg.Message); err != nil {
				p.logger.Error().WithError(err).Message("error acking a message")
			}
		case <-receivedMsg.chNack:
			if err := p.nack(ctx, receivedMsg.Message); err != nil {
				p.logger.Error().WithError(err).Message("error nacking a message")
			}
		case <-ctx.Done():
			return
		}
	}
}

func (p *PubSub) readMsg(ctx context.Context, topic string) (Message, error) {
	var msg Message
	if err := p.transactionProvider.Transact(ctx, func(ctx context.Context, tx *sql.Tx) error {
		row := tx.QueryRow(
			"SELECT uuid, payload FROM pubsub WHERE topic = ? AND (backoff_until IS NULL OR backoff_until <= ?) ORDER BY RANDOM() LIMIT 1",
			topic,
			time.Now().Unix(),
		)

		var uuid string
		var payload []byte
		if err := row.Scan(&uuid, &payload); err != nil {
			return errors.Wrap(err, "row scan error")
		}

		tmp, err := NewMessage(uuid, payload)
		if err != nil {
			return errors.Wrap(err, "error creating a message")
		}

		msg = tmp
		return nil
	}); err != nil {
		return Message{}, errors.Wrap(err, "transaction error")
	}

	return msg, nil
}

func (p *PubSub) ack(ctx context.Context, msg Message) error {
	return p.transactionProvider.Transact(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.Exec(
			"DELETE FROM pubsub WHERE uuid = ?",
			msg.uuid,
		)
		return err
	})
}

func (p *PubSub) nack(ctx context.Context, msg Message) error {
	return p.transactionProvider.Transact(ctx, func(ctx context.Context, tx *sql.Tx) error {
		row := tx.QueryRow(
			"SELECT nack_count FROM pubsub WHERE uuid = ? LIMIT 1",
			msg.uuid,
		)

		var nackCount int
		if err := row.Scan(&nackCount); err != nil {
			return errors.Wrap(err, "error calling scan")
		}

		nackCount = nackCount + 1
		backoffDuration := p.backoffManager.GetMessageErrorBackoff(nackCount)
		backoffUntil := time.Now().Add(backoffDuration)

		p.logger.Trace().
			WithField("until", backoffUntil).
			WithField("duration", backoffDuration).
			Message("backing off a message")

		if _, err := tx.Exec(
			"UPDATE pubsub SET nack_count = ?, backoff_until = ? WHERE uuid = ?",
			nackCount,
			backoffUntil.Unix(),
			msg.uuid,
		); err != nil {
			return errors.Wrap(err, "error updating the message")
		}

		return nil

	})
}

type TxPubSub struct {
	tx     *sql.Tx
	logger logging.Logger
}

func NewTxPubSub(
	tx *sql.Tx,
	logger logging.Logger,
) *TxPubSub {
	return &TxPubSub{
		tx:     tx,
		logger: logger,
	}
}

func (p *TxPubSub) PublishTx(topic string, msg Message) error {
	_, err := p.tx.Exec(
		"INSERT INTO pubsub VALUES (?, ?, ?, ?, ?, ?)",
		topic,
		msg.uuid,
		msg.payload,
		time.Now().Unix(),
		0,
		nil,
	)
	return err
}

const (
	maxDefaultMessageErrorBackoff = 1 * time.Hour
	maxDefaultNoMessagesBackoff   = 30 * time.Second

	randomizeMessageErrorBackoffByFraction = 0.1
)

type DefaultBackoffManager struct {
}

func NewDefaultBackoffManager() DefaultBackoffManager {
	return DefaultBackoffManager{}
}

func (d DefaultBackoffManager) GetMessageErrorBackoff(nackCount int) time.Duration {
	a := time.Duration(math.Pow(4, float64(nackCount-1))) * time.Second
	value := min(a, maxDefaultMessageErrorBackoff)
	if value <= 0 {
		value = maxDefaultMessageErrorBackoff
	}
	randFraction := 1 - randomizeMessageErrorBackoffByFraction + 2*randomizeMessageErrorBackoffByFraction*rand.Float64()
	return time.Duration(float64(value) * randFraction)
}

func (d DefaultBackoffManager) GetNoMessagesBackoff(tick int) time.Duration {
	a := time.Duration(math.Pow(2, float64(tick-1))) * time.Second
	if a <= 0 {
		return maxDefaultNoMessagesBackoff
	}
	return min(a, maxDefaultNoMessagesBackoff)
}
