package domain

import (
	"net"
	"net/url"
	"strings"

	"github.com/boreq/errors"
)

type RelayAddress struct {
	original        string
	hostWithoutPort string
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

	u.Host = strings.ToLower(u.Host)
	hostWithoutPort, _, err := net.SplitHostPort(u.Host)
	if err != nil {
		hostWithoutPort = u.Host
	}
	normalizedURI := u.String()

	return RelayAddress{
		original:        normalizedURI,
		hostWithoutPort: hostWithoutPort,
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
	ip := net.ParseIP(r.hostWithoutPort)
	return ip.IsLoopback() || ip.IsPrivate()
}

func (r RelayAddress) HostWithoutPort() string {
	return r.hostWithoutPort
}

func (r RelayAddress) String() string {
	return r.original
}
