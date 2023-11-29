package domain

import (
	"encoding/json"
	"fmt"

	"github.com/boreq/errors"
	"github.com/planetary-social/nos-event-service/internal"
)

type Registration struct {
	publicKey PublicKey
	relays    []RelayAddress
}

func NewRegistrationFromEvent(event Event) (Registration, error) {
	if event.Kind() != EventKindRegistration {
		return Registration{}, fmt.Errorf("invalid event kind '%d'", event.Kind().Int())
	}

	var v registrationContent
	if err := json.Unmarshal([]byte(event.Content()), &v); err != nil {
		return Registration{}, errors.Wrap(err, "error unmarshaling content")
	}

	relays, err := newRelays(v)
	if err != nil {
		return Registration{}, errors.Wrap(err, "error creating relay addresses")
	}

	return Registration{
		publicKey: event.PubKey(),
		relays:    relays,
	}, nil
}

func (p Registration) PublicKey() PublicKey {
	return p.publicKey
}

func (p Registration) Relays() []RelayAddress {
	return internal.CopySlice(p.relays)
}

type registrationContent struct {
	Relays []relayTransport `json:"relays"`
}

type relayTransport struct {
	Address string `json:"address"`
}

func newRelays(v registrationContent) ([]RelayAddress, error) {
	var relays []RelayAddress
	for _, relayTransport := range v.Relays {
		address, err := NewRelayAddress(relayTransport.Address)
		if err != nil {
			return nil, errors.Wrap(err, "error creating relay address")
		}
		relays = append(relays, address)
	}
	return relays, nil
}
