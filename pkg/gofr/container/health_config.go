package container

import (
	"strings"
	"time"

	"gofr.dev/pkg/gofr/config"
)

const (
	// healthCacheTTLKey configures how long a health result may be served without re-checking the
	// backends. Unset means no caching, so upgrading GoFr never silently starts serving a stale
	// health body; 5s is the value to set for a service whose probes outpace its backends.
	healthCacheTTLKey = "HEALTH_CACHE_TTL"

	// healthCheckTimeoutKey bounds one round of checks. Unset means the round is bounded only by
	// each backend client's own timeout, which is how GoFr has always behaved.
	healthCheckTimeoutKey = "HEALTH_CHECK_TIMEOUT"
)

// healthDuration reads one of the health duration settings. Both default to disabled, so anything
// that is not a usable positive duration -- unset, "0", "disabled", unparsable, or negative --
// turns the feature off. The two malformed cases are logged, because they are almost always a typo
// in a config file rather than an intent to disable.
func (c *Container) healthDuration(conf config.Config, key string) time.Duration {
	value := strings.TrimSpace(conf.Get(key))

	if value == "" || value == "0" || strings.EqualFold(value, "disabled") {
		return 0
	}

	d, err := time.ParseDuration(value)
	if err != nil {
		c.logHealthConfigWarningf("invalid %s value %q, disabling it", key, value)

		return 0
	}

	if d < 0 {
		c.logHealthConfigWarningf("negative %s value %q, disabling it", key, value)

		return 0
	}

	return d
}

// logHealthConfigWarning tolerates a nil Logger: healthDuration is reachable from a Container built
// in a test without one.
func (c *Container) logHealthConfigWarningf(format string, args ...any) {
	if c.Logger == nil {
		return
	}

	c.Logger.Warnf(format, args...)
}
