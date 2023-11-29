package mocks

import (
	"context"

	"github.com/planetary-social/nos-event-service/service/domain"
)

type RelayRepository struct {
}

func NewRelayRepository() *RelayRepository {
	return &RelayRepository{}
}

func (r RelayRepository) Save(ctx context.Context, eventID domain.EventId, relayAddress domain.MaybeRelayAddress) error {
	panic("implement me")
}

func (r RelayRepository) List(ctx context.Context) ([]domain.MaybeRelayAddress, error) {
	panic("implement me")
}

func (r RelayRepository) Count(ctx context.Context) (int, error) {
	panic("implement me")
}
