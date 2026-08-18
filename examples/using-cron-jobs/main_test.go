package main

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/testutil"
)

func TestMain(m *testing.M) {
	os.Setenv("GOFR_TELEMETRY", "false")
	m.Run()
}

func Test_UserPurgeCron(t *testing.T) {
	configs := testutil.NewServerConfigs(t)

	go main()

	// The job is scheduled every second. Waiting on the counter it increments covers both the
	// server's startup and the first tick, neither of which fits a fixed duration: a sleep long
	// enough for a loaded runner also lets the job fire more than once, which the old
	// assert.Equal(1, n) would then fail on.
	require.Eventually(t, func() bool {
		mu.RLock()
		defer mu.RUnlock()

		return n > 0
	}, 30*time.Second, 100*time.Millisecond, "cron job did not run in time")

	t.Logf("Metrics server running at: %s", configs.MetricsHost)
}
