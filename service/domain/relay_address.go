package domain

import (
	"net"
	"net/url"
	"strings"

	"github.com/boreq/errors"
)

type RelayAddress struct {
	original string
}

func NewRelayAddress(s string) (RelayAddress, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimRight(s, "/")

	u, err := url.Parse(s)
	if err != nil {
		return RelayAddress{}, errors.Wrap(err, "url parse error")
	}

	if u.Scheme != "ws" && u.Scheme != "wss" {
		return RelayAddress{}, errors.New("invalid protocol")
	}

	return RelayAddress{
		original: s,
	}, nil
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
	hostWithoutPort := r.getHostWithoutPort()
	ip := net.ParseIP(hostWithoutPort)
	return ip.IsLoopback() || ip.IsPrivate()
}

func (r RelayAddress) getHostWithoutPort() string {
	u, err := url.Parse(r.original)
	if err != nil {
		panic(err) // checked in constructor
	}

	hostWithoutPort, _, err := net.SplitHostPort(u.Host)
	if err != nil {
		return u.Host
	}
	return hostWithoutPort
}

func (r RelayAddress) String() string {
	return r.original
}
