package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_UserPurgeCron(t *testing.T) {
	// TODO find a way to do not have to wait for more than a minute
	go main()
	time.Sleep(1*time.Minute + 30*time.Second)

	expected := 1

	var m int

	mu.Lock()
	m = n
	mu.Unlock()

	assert.Equal(t, expected, m)
}
