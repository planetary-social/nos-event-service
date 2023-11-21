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
	hostWithoutPort, err := r.getHostWithoutPort()
	if err != nil {
		return false
	}
	ip := net.ParseIP(hostWithoutPort)
	return ip.IsLoopback() || ip.IsPrivate()
}

func (r RelayAddress) getHostWithoutPort() (string, error) {
	u, err := url.Parse(r.s)
	if err != nil {
		return "", errors.Wrap(err, "url parse error")
	}

	hostWithoutPort, _, err := net.SplitHostPort(u.Host)
	if err != nil {
		return u.Host, nil
	}
	return hostWithoutPort, nil
}

func (r RelayAddress) String() string {
	return r.s
}
