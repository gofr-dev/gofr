package store

import "testing"

func TestDummy(t *testing.T) {
	if 1 != 1 {
		t.Error("This should never fail")
	}
}
