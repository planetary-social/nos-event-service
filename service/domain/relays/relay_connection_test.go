package relays_test

import (
	"testing"

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
