package gofr

const (
	defaultHTTPPort   = 8000
	defaultGRPCPort   = 9000
	defaultMetricPort = 2121
	defaultMCPPort    = 8200

	// minTCPPort and maxTCPPort bound a port number that will actually be handed to net.Listen.
	// A value outside this range is not a busy port, it is an impossible one, and it fails the bind
	// permanently rather than transiently - so it is worth rejecting where it is read.
	minTCPPort = 1
	maxTCPPort = 65535
)
