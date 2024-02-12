package relays_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/boreq/errors"
	"github.com/planetary-social/nos-event-service/internal"
	"github.com/planetary-social/nos-event-service/internal/fixtures"
	"github.com/planetary-social/nos-event-service/internal/logging"
	"github.com/planetary-social/nos-event-service/service/domain"
	"github.com/planetary-social/nos-event-service/service/domain/relays"
	"github.com/planetary-social/nos-event-service/service/domain/relays/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDialErrorIs(t *testing.T) {
	err := relays.NewDialError(fixtures.SomeError())
	require.ErrorIs(t, err, relays.DialError{})
	require.ErrorIs(t, err, &relays.DialError{})
	require.ErrorIs(t, errors.Wrap(err, "wrapped"), relays.DialError{})
	require.ErrorIs(t, errors.Wrap(err, "wrapped"), &relays.DialError{})
}

func TestReadMessageErrorIs(t *testing.T) {
	err := relays.NewReadMessageError(fixtures.SomeError())
	require.ErrorIs(t, err, relays.ReadMessageError{})
	require.ErrorIs(t, err, &relays.ReadMessageError{})
	require.ErrorIs(t, errors.Wrap(err, "wrapped"), relays.ReadMessageError{})
	require.ErrorIs(t, errors.Wrap(err, "wrapped"), &relays.ReadMessageError{})
}

func TestOKResponseErrorErrorIs(t *testing.T) {
	err := relays.NewOKResponseError(fixtures.SomeString())
	require.ErrorIs(t, err, relays.OKResponseError{})
	require.ErrorIs(t, err, &relays.OKResponseError{})
	require.ErrorIs(t, errors.Wrap(err, "wrapped"), relays.OKResponseError{})
	require.ErrorIs(t, errors.Wrap(err, "wrapped"), &relays.OKResponseError{})
}

func TestDefaultBackoffManager_GetReconnectionBackoffReturnsSaneResultsForDialErrors(t *testing.T) {
	m := relays.NewDefaultBackoffManager()
	for i := 0; i < 100; i++ {
		backoff := m.GetReconnectionBackoff(relays.DialError{})
		t.Log(backoff)
		if i == 0 {
			require.Equal(t, backoff, 5*time.Second, "backoff shouldn't start with pow 0 (1 second)")
		}
		require.Positive(t, backoff)
		require.LessOrEqual(t, backoff, relays.MaxDialReconnectionBackoff)
	}
}

func TestDefaultBackoffManager_GetReconnectionBackoffDoesNotScaleIfOtherErrorOccurs(t *testing.T) {
	m := relays.NewDefaultBackoffManager()

	for i := 0; i < 100; i++ {
		backoff := m.GetReconnectionBackoff(fixtures.SomeError())
		require.Equal(t, backoff, relays.ReconnectionBackoff)
	}
}

func TestDefaultBackoffManager_GetReconnectionBackoffDoesNotScaleIfNoErrorOccurs(t *testing.T) {
	m := relays.NewDefaultBackoffManager()

	for i := 0; i < 100; i++ {
		backoff := m.GetReconnectionBackoff(nil)
		require.Equal(t, backoff, relays.ReconnectionBackoff)
	}
}

func TestDefaultBackoffManager_GetReconnectionBackoffResetsDialBackoffIfDifferentErrorOrNoErrorOcccurs(t *testing.T) {
	testCases := []struct {
		Err error
	}{
		{
			Err: nil,
		},
		{
			Err: fixtures.SomeError(),
		},
	}

	for _, testCase := range testCases {
		t.Run(fmt.Sprintf("%s", testCase.Err), func(t *testing.T) {
			m := relays.NewDefaultBackoffManager()

			backoff1 := m.GetReconnectionBackoff(relays.DialError{})
			backoff2 := m.GetReconnectionBackoff(relays.DialError{})
			require.Less(t, backoff1, backoff2)

			backoff3 := m.GetReconnectionBackoff(testCase.Err)
			require.Equal(t, backoff3, relays.ReconnectionBackoff)

			backoff4 := m.GetReconnectionBackoff(relays.DialError{})
			backoff5 := m.GetReconnectionBackoff(relays.DialError{})
			require.Equal(t, backoff1, backoff4)
			require.Equal(t, backoff2, backoff5)

			backoff6 := m.GetReconnectionBackoff(testCase.Err)
			require.Equal(t, backoff6, relays.ReconnectionBackoff)
		})
	}
}

func TestDefaultBackoffManager_NormalBackoffIsSmallerThanMaxDialBackoff(t *testing.T) {
	require.Less(t, relays.ReconnectionBackoff, relays.MaxDialReconnectionBackoff)
}

func TestRelayConnection_SendEventSendsEvents(t *testing.T) {
	ctx := fixtures.TestContext(t)
	ts := newTestConnection(t, ctx)

	event := fixtures.SomeEvent()

	go func() {
		_ = ts.RelayConnection.SendEvent(ctx, event)
	}()

	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		msgs := ts.Connection.OutgoingMessages()
		assert.Contains(collect, msgs, transport.NewMessageEvent(event))
	}, 10*time.Second, 10*time.Millisecond)
}

type testConnection struct {
	RelayConnection *relays.RelayConnection
	Connection      *mockConnection
}

func newTestConnection(tb testing.TB, ctx context.Context) *testConnection {
	connection := newMockConnection()
	factory := newMockConnectionFactory(connection)
	metrics := newMockMetrics()
	logger := logging.NewDevNullLogger()
	relayConnection := relays.NewRelayConnection(factory, logger, metrics)
	go relayConnection.Run(ctx)

	return &testConnection{
		RelayConnection: relayConnection,
		Connection:      connection,
	}
}

type mockConnection struct {
	closed bool
	mutex  sync.Mutex

	incomingMessages [][]byte
	outgoingMessages []relays.NostrMessage
}

func newMockConnection() *mockConnection {
	return &mockConnection{}
}

func (m *mockConnection) AddIncomingMessage(msg relays.NostrMessage) error {
	b, err := msg.MarshalJSON()
	if err != nil {
		return errors.Wrap(err, "error marshaling the incoming message")
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.incomingMessages = append(m.incomingMessages, b)
	return nil
}

func (m *mockConnection) OutgoingMessages() []relays.NostrMessage {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	return internal.CopySlice(m.outgoingMessages)
}

func (m *mockConnection) SendMessage(msg relays.NostrMessage) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.closed {
		return errors.New("mock connection is closed")
	}

	m.outgoingMessages = append(m.outgoingMessages, msg)
	return nil
}

func (m *mockConnection) ReadMessage() ([]byte, error) {
	for {
		m.mutex.Lock()

		if m.closed {
			return nil, errors.New("mock connection is closed")
		}

		if len(m.incomingMessages) > 0 {
			msg := m.incomingMessages[0]
			m.incomingMessages = m.incomingMessages[1:]
			m.mutex.Unlock()
			return msg, nil
		}

		m.mutex.Unlock()

		<-time.After(100 * time.Millisecond)
	}
}

func (m *mockConnection) Close() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.closed = true
	return nil
}

type mockConnectionFactory struct {
	conn    relays.Connection
	address domain.RelayAddress
}

func newMockConnectionFactory(conn relays.Connection) *mockConnectionFactory {
	return &mockConnectionFactory{
		conn:    conn,
		address: fixtures.SomeRelayAddress(),
	}
}

func (m mockConnectionFactory) Create(ctx context.Context) (relays.Connection, error) {
	return m.conn, nil
}

func (m mockConnectionFactory) Address() domain.RelayAddress {
	return m.address
}

type mockMetrics struct {
}

func newMockMetrics() *mockMetrics {
	return &mockMetrics{}
}

func (m2 mockMetrics) ReportRelayConnectionsState(m map[domain.RelayAddress]relays.RelayConnectionState) {
}

func (m2 mockMetrics) ReportNumberOfSubscriptions(address domain.RelayAddress, n int) {
}

func (m2 mockMetrics) ReportMessageReceived(address domain.RelayAddress, messageType relays.MessageType, err *error) {
}

func (m2 mockMetrics) ReportRelayDisconnection(address domain.RelayAddress, err error) {
}

func (m2 mockMetrics) ReportNotice(address domain.RelayAddress, noticeType relays.NoticeType) {
}
