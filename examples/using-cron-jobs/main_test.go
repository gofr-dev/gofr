package main

import (
	"fmt"
	"gofr.dev/pkg/gofr/testutil"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_UserPurgeCron(t *testing.T) {
	port := testutil.GetFreePort(t)
	t.Setenv("METRICS_PORT", fmt.Sprint(port))

	go main()
	time.Sleep(1100 * time.Millisecond)

	expected := 1

	var m int

	mu.Lock()
	m = n
	mu.Unlock()

	assert.Equal(t, expected, m)
}
