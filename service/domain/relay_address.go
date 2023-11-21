package domain

import (
	"net"
	"net/url"
	"strings"

	"github.com/boreq/errors"
)

type RelayAddress struct {
	original string
	parsed   *url.URL
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
		parsed:   u,
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
	hostWithoutPort, err := r.getHostWithoutPort()
	if err != nil {
		return false
	}
	ip := net.ParseIP(hostWithoutPort)
	return ip.IsLoopback() || ip.IsPrivate()
}

func (r RelayAddress) getHostWithoutPort() (string, error) {
	hostWithoutPort, _, err := net.SplitHostPort(r.parsed.Host)
	if err != nil {
		return r.parsed.Host, nil
	}
	return hostWithoutPort, nil
}

func (r RelayAddress) String() string {
	return r.original
}
