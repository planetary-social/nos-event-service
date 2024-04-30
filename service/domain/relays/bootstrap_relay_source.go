package relays

import (
	"context"

	"github.com/planetary-social/nos-event-service/internal"
	"github.com/planetary-social/nos-event-service/service/domain"
)

var bootstrapRelayAddresses = []domain.RelayAddress{
	domain.MustNewRelayAddress("wss://relay.damus.io"),
	domain.MustNewRelayAddress("wss://nos.lol"),
	domain.MustNewRelayAddress("wss://e.nos.lol"),
	domain.MustNewRelayAddress("wss://purplepag.es"),
	domain.MustNewRelayAddress("wss://relay.current.fyi"),
	domain.MustNewRelayAddress("wss://brb.io"),
	domain.MustNewRelayAddress("wss://relay.snort.social"),
}

type BootstrapRelaySource struct {
}

func NewBootstrapRelaySource() *BootstrapRelaySource {
	return &BootstrapRelaySource{}
}

func (p BootstrapRelaySource) GetRelays(ctx context.Context) ([]domain.RelayAddress, error) {
	return internal.CopySlice(bootstrapRelayAddresses), nil
}
