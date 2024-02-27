package main

import (
	"strings"
	"testing"
	"time"

	"gofr.dev/pkg/gofr/testutil"
)

func TestExampleSubscriber(t *testing.T) {
	log := testutil.StdoutOutputForFunc(func() {
		const host = "http://localhost:8200"
		go main()
		time.Sleep(time.Minute * 2)
	})

	testCases := []struct {
		desc        string
		expectedLog string
	}{
		{
			desc:        "valid order",
			expectedLog: "Received order",
		},
		{
			desc:        "valid  product",
			expectedLog: "Received product",
		},
	}

	for i, tc := range testCases {
		if !strings.Contains(log, tc.expectedLog) {
			t.Errorf("TEST[%d], Failed.\n%s", i, tc.desc)
		}
	}
}
