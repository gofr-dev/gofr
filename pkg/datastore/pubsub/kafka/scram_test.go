package kafka

import (
	"strings"
	"testing"
)

func Test_BeginSuccess(t *testing.T) {
	client := XDGSCRAMClient{HashGeneratorFcn: SHA512}

	err := client.Begin("test-user", "password", "")
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}
}

func Test_BeginError(t *testing.T) {
	client := XDGSCRAMClient{HashGeneratorFcn: SHA512}

	errStr := "Error SASLprepping username"

	err := client.Begin("\u0627\u0031", "password", "")
	if err == nil || !strings.Contains(err.Error(), errStr) {
		t.Errorf("expected error string %v, got %v", errStr, err)
	}
}

func Test_Step(t *testing.T) {
	client := XDGSCRAMClient{HashGeneratorFcn: SHA512}

	err := client.Begin("test-user", "password", "")
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}

	_, err = client.Step("")
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}
}

func Test_Done(t *testing.T) {
	client := XDGSCRAMClient{HashGeneratorFcn: SHA512}

	err := client.Begin("test-user", "password", "")
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}

	if client.Done() {
		t.Errorf("unexpected done from the client")
	}
}
