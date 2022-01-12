package grpc

import (
	context "context"
	"testing"
)

func TestServer_SayHello(t *testing.T) {
	s := Server{}

	// set up test cases
	tests := []struct {
		name string
		want string
	}{
		{"world", "Hello world!"},
		{"123", "Hello 123!"},
		{"", "Hello World!"},
	}

	for _, tt := range tests {
		req := &HelloRequest{Name: tt.name}
		resp, err := s.SayHello(context.Background(), req)
		if err != nil {
			t.Errorf("SayHello(%v) got unexpected error", err)
		}
		if resp.Message != tt.want {
			t.Errorf("SayHello(%v) = %v, wanted %v", tt.name, resp.Message, tt.want)
		}
	}
}
