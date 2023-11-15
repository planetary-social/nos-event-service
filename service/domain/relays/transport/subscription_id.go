package transport

import (
	"github.com/boreq/errors"
)

type SubscriptionID struct {
	s string
}

func NewSubscriptionID(s string) (SubscriptionID, error) {
	if s == "" {
		return SubscriptionID{}, errors.New("subscription id can't be an empty string")
	}
	return SubscriptionID{s: s}, nil
}

func (s SubscriptionID) String() string {
	return s.s
}
