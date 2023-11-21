package relays_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/boreq/errors"
	"github.com/planetary-social/nos-event-service/internal/fixtures"
	"github.com/planetary-social/nos-event-service/service/domain/relays"
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
