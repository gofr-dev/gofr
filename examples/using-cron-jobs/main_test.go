package main

import (
	"gofr.dev/pkg/gofr/testutil"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

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
