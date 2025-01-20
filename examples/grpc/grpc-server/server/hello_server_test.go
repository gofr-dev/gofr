package server

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr"
)

func TestServer_SayHello(t *testing.T) {
	s := HelloGoFrServer{}

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

		request := &HelloRequestWrapper{
			context.Background(),
			req,
		}

		ctx := &gofr.Context{
			Request: request,
		}

		resp, err := s.SayHello(ctx)
		grpcResponse, ok := resp.(*HelloResponse)
		require.True(t, ok)

		require.NoError(t, err, "TEST[%d], Failed.\n", i)

		assert.Equal(t, tc.resp, grpcResponse.Message, "TEST[%d], Failed.\n", i)
	}
}
