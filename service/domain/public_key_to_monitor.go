package domain

import (
	"time"

	"github.com/boreq/errors"
)

type PublicKeyToMonitor struct {
	publicKey PublicKey
	createdAt time.Time
	updatedAt time.Time
}

func NewPublicKeyToMonitor(publicKey PublicKey, createdAt time.Time, updatedAt time.Time) (PublicKeyToMonitor, error) {
	if createdAt.IsZero() {
		return PublicKeyToMonitor{}, errors.New("zero value of created at")
	}
	if updatedAt.IsZero() {
		return PublicKeyToMonitor{}, errors.New("zero value of updated at")
	}
	if updatedAt.Before(createdAt) {
		return PublicKeyToMonitor{}, errors.New("updated at is before created at")
	}
	return PublicKeyToMonitor{
		publicKey: publicKey,
		createdAt: createdAt,
		updatedAt: updatedAt,
	}, nil
}

func MustNewPublicKeyToMonitor(publicKey PublicKey, createdAt time.Time, updatedAt time.Time) PublicKeyToMonitor {
	v, err := NewPublicKeyToMonitor(publicKey, createdAt, updatedAt)
	if err != nil {
		panic(err)
	}
	return v
}

func (p PublicKeyToMonitor) PublicKey() PublicKey {
	return p.publicKey
}

func (p PublicKeyToMonitor) CreatedAt() time.Time {
	return p.createdAt
}

func (p PublicKeyToMonitor) UpdatedAt() time.Time {
	return p.updatedAt
}
