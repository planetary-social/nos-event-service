package relays

var (
	RelayConnectionStateInitializing  = RelayConnectionState{"initializing"}
	RelayConnectionStateConnected     = RelayConnectionState{"connected"}
	RelayConnectionStateDisconnected  = RelayConnectionState{"disconnected"}
	RelayConnectionStateBackPressured = RelayConnectionState{"backpressured"}
)

type RelayConnectionState struct {
	s string
}

func (r RelayConnectionState) String() string {
	return r.s
}
