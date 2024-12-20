package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_UserPurgeCron(t *testing.T) {
	t.Setenv("METRICS_PORT", fmt.Sprint(2022))

	go main()
	time.Sleep(1100 * time.Millisecond)

	expected := 1

	var m int

	mu.Lock()
	m = n
	mu.Unlock()

	assert.Equal(t, expected, m)
}
