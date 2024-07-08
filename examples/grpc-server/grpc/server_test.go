package grpc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServer_SayHello(t *testing.T) {
	s := Server{}

	tests := []struct {
		input string
		resp  string
	}{
		{"world", "Hello world!"},
		{"123", "Hello 123!"},
		{"", "Hello World!"},
	}

	for i, tc := range tests {
		req := &HelloRequest{Name: tc.input}
		resp, err := s.SayHello(context.Background(), req)

		assert.NoError(t, err, "TEST[%d], Failed.\n", i)

		assert.Equal(t, tc.resp, resp.Message, "TEST[%d], Failed.\n", i)
	}
}
