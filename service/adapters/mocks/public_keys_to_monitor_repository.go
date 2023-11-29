package mocks

import (
	"context"

	"github.com/planetary-social/nos-event-service/service/domain"
)

type PublicKeysToMonitorRepository struct {
}

func NewPublicKeysToMonitorRepository() *PublicKeysToMonitorRepository {
	return &PublicKeysToMonitorRepository{}
}

func (p PublicKeysToMonitorRepository) Save(ctx context.Context, publicKeyToMonitor domain.PublicKeyToMonitor) error {
	panic("implement me")
}

func (p PublicKeysToMonitorRepository) List(ctx context.Context) ([]domain.PublicKeyToMonitor, error) {
	panic("implement me")
}

func (p PublicKeysToMonitorRepository) Get(ctx context.Context, publicKey domain.PublicKey) (domain.PublicKeyToMonitor, error) {
	panic("implement me")
}
