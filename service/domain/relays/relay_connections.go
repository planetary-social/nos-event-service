package relays

import (
	"context"
	"sync"
	"time"

	"github.com/planetary-social/nos-event-service/internal/logging"
	"github.com/planetary-social/nos-event-service/service/domain"
)

type Metrics interface {
	ReportRelayConnectionsState(m map[domain.RelayAddress]RelayConnectionState)
	ReportNumberOfSubscriptions(address domain.RelayAddress, n int)
	ReportMessageReceived(address domain.RelayAddress, messageType MessageType, err *error)
	ReportRelayDisconnection(address domain.RelayAddress, err error)
}

type MessageType struct {
	s string
}

func (t MessageType) String() string {
	return t.s
}

var (
	MessageTypeNotice  = MessageType{"notice"}
	MessageTypeEOSE    = MessageType{"eose"}
	MessageTypeEvent   = MessageType{"event"}
	MessageTypeAuth    = MessageType{"auth"}
	MessageTypeOK      = MessageType{"ok"}
	MessageTypeUnknown = MessageType{"unknown"}
)

const (
	storeMetricsEvery = 30 * time.Second
)

type RelayConnections struct {
	logger  logging.Logger
	metrics Metrics

	longCtx context.Context

	connections     map[domain.RelayAddress]*RelayConnection
	connectionsLock sync.Mutex
}

func NewRelayConnections(ctx context.Context, logger logging.Logger, metrics Metrics) *RelayConnections {
	v := &RelayConnections{
		logger:          logger.New("relayConnections"),
		metrics:         metrics,
		longCtx:         ctx,
		connections:     make(map[domain.RelayAddress]*RelayConnection),
		connectionsLock: sync.Mutex{},
	}
	go v.storeMetricsLoop(ctx)
	return v
}

func (d *RelayConnections) GetEvents(ctx context.Context, relayAddress domain.RelayAddress, filter domain.Filter) (<-chan EventOrEndOfSavedEvents, error) {
	connection := d.getConnection(relayAddress)
	return connection.GetEvents(ctx, filter)
}

func (d *RelayConnections) SendEvent(ctx context.Context, relayAddress domain.RelayAddress, event domain.Event) error {
	connection := d.getConnection(relayAddress)
	return connection.SendEvent(ctx, event)
}

func (d *RelayConnections) storeMetricsLoop(ctx context.Context) {
	for {
		d.storeMetrics()

		select {
		case <-time.After(storeMetricsEvery):
		case <-ctx.Done():
			return
		}
	}
}

func (d *RelayConnections) storeMetrics() {
	d.connectionsLock.Lock()
	defer d.connectionsLock.Unlock()

	m := make(map[domain.RelayAddress]RelayConnectionState)
	for _, connection := range d.connections {
		m[connection.Address()] = connection.State()
	}
	d.metrics.ReportRelayConnectionsState(m)
}

func (r *RelayConnections) getConnection(relayAddress domain.RelayAddress) *RelayConnection {
	r.connectionsLock.Lock()
	defer r.connectionsLock.Unlock()

	if connection, ok := r.connections[relayAddress]; ok {
		return connection
	}

	factory := NewWebsocketConnectionFactory(relayAddress, r.logger)
	connection := NewRelayConnection(factory, r.logger, r.metrics)
	go connection.Run(r.longCtx)

	r.connections[relayAddress] = connection
	return connection
}
