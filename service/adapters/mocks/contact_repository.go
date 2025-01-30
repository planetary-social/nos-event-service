package mocks

import (
	"context"

	"github.com/planetary-social/nos-event-service/service/app"
	"github.com/planetary-social/nos-event-service/service/domain"
)

type ContactRepository struct {
}

func NewContactRepository() *ContactRepository {
	return &ContactRepository{}
}

func (c ContactRepository) GetCurrentContactsEvent(ctx context.Context, author domain.PublicKey) (domain.Event, error) {
	return domain.Event{}, app.ErrNoContactsEvent
}

func (c ContactRepository) SetContacts(ctx context.Context, event domain.Event, contacts []domain.PublicKey) error {
	return nil
}

func (c ContactRepository) GetFollowees(ctx context.Context, publicKey domain.PublicKey) ([]domain.PublicKey, error) {
	panic("implement me")
}

func (c ContactRepository) IsFolloweeOfMonitoredPublicKey(ctx context.Context, publicKey domain.PublicKey) (bool, error) {
	panic("implement me")
}

func (c ContactRepository) CountFollowers(ctx context.Context, publicKey domain.PublicKey) (int, error) {
	panic("implement me")
}

func (c ContactRepository) CountFollowees(ctx context.Context, publicKey domain.PublicKey) (int, error) {
	panic("implement me")
}
