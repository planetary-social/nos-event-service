package domain

import (
	"net"
	"net/url"
	"strings"

	"github.com/boreq/errors"
)

type RelayAddress struct {
	s string
}

func NewRelayAddress(s string) (RelayAddress, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimRight(s, "/")

	if !strings.HasPrefix(s, "ws://") && !strings.HasPrefix(s, "wss://") {
		return RelayAddress{}, errors.New("invalid protocol")
	}
	return RelayAddress{s: s}, nil
}

func MustNewRelayAddress(s string) RelayAddress {
	v, err := NewRelayAddress(s)
	if err != nil {
		panic(err)
	}
	return v
}

func NewRelayAddressFromMaybeAddress(maybe MaybeRelayAddress) (RelayAddress, error) {
	return NewRelayAddress(maybe.String())
}

func (r RelayAddress) IsLoopbackOrPrivate() bool {
	u, err := url.Parse(r.s)
	if err != nil {
		return false
	}
	ip := net.ParseIP(u.Host)
	return ip.IsLoopback() || ip.IsPrivate()
}

func (r RelayAddress) String() string {
	return r.s
}
