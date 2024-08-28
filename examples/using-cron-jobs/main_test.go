package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_UserPurgeCron(t *testing.T) {
	go main()
	time.Sleep(1100 * time.Millisecond)

	expected := 1

	var m int

	mu.Lock()
	m = n
	mu.Unlock()

	assert.Equal(t, expected, m)
}
