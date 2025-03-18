package main

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gofr.dev/pkg/gofr/testutil"
)

func TestMain(m *testing.M) {
	os.Setenv("GOFR_TELEMETRY", "false")
	m.Run()
}

func Test_UserPurgeCron(t *testing.T) {
	configs := testutil.NewServerConfigs(t)

	go main()
	time.Sleep(1100 * time.Millisecond)

	expected := 1

	var m int

	mu.Lock()
	m = n
	mu.Unlock()

	assert.Equal(t, expected, m)
	t.Logf("Metrics server running at: %s", configs.MetricsHost)
}
