package openai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

// RequestPayload is the structure sent to OpenAI API
type RequestPayload struct {
	Model    string         `json:"model"`
	Messages []MessageEntry `json:"messages"`
}

// MessageEntry represents each message in chat
type MessageEntry struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ResponsePayload is the structure received from OpenAI
type ResponsePayload struct {
	Choices []struct {
		Message MessageEntry `json:"message"`
	} `json:"choices"`
}

// ChatCompletion calls OpenAI's Chat API
func ChatCompletion(prompt string, apiKey string) (string, error) {
	if apiKey == "" {
		return "", fmt.Errorf("API key is required")
	}

	payload := RequestPayload{
		Model: "gpt-3.5-turbo",
		Messages: []MessageEntry{
			{Role: "user", Content: prompt},
		},
	}

	body, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := ioutil.ReadAll(resp.Body)

	var result ResponsePayload
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", err
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no response from OpenAI")
	}

	return result.Choices[0].Message.Content, nil
}
