package domain_test

import (
	"testing"

	"github.com/planetary-social/nos-event-service/internal/fixtures"
	"github.com/planetary-social/nos-event-service/service/domain"
	"github.com/stretchr/testify/require"
)

func TestNewRegistrationFromEvent(t *testing.T) {
	event := fixtures.Event(domain.EventKindRegistration, nil, `{"relays": [ {"address": "wss://example.com"} ]}`)

	registration, err := domain.NewRegistrationFromEvent(event)
	require.NoError(t, err)
	require.Equal(t, []domain.RelayAddress{domain.MustNewRelayAddress("wss://example.com")}, registration.Relays())
	require.Equal(t, event.PubKey(), registration.PublicKey())
}
