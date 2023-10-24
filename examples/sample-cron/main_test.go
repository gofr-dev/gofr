package main

import (
	"testing"
	"time"
)

func Test_main(t *testing.T) {
	go main()
	time.Sleep(1*time.Minute + 30*time.Second)

	expected := 1

	var m int

	mu.Lock()
	m = n
	mu.Unlock()

	if m != expected {
		t.Errorf("FAILED, Expected: %v, Got: %v", expected, m)
	}
}
