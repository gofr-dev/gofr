package openai

import (
	"os"
	"testing"
)

func TestChatCompletion(t *testing.T) {
	os.Setenv("OPENAI_API_KEY", "sk-test") // Dummy for test. Use real one when testing

	response, err := ChatCompletion("Hello!")
	if err == nil {
		t.Log("API responded:", response)
	} else {
		t.Log("Expected error (since test key):", err)
	}
}
