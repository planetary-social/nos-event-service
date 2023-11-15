package app

import (
	"context"

	"github.com/planetary-social/nos-event-service/service/domain"
)

type MockRelaySource struct {
}

func NewMockRelaySource() *MockRelaySource {
	return &MockRelaySource{}
}

func (m MockRelaySource) GetRelays(ctx context.Context) ([]domain.RelayAddress, error) {
	return nil, nil
}
