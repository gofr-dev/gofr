package openai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
)

type RequestPayload struct {
	Model    string         `json:"model"`
	Messages []MessageEntry `json:"messages"`
}

type MessageEntry struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ResponsePayload struct {
	Choices []struct {
		Message MessageEntry `json:"message"`
	} `json:"choices"`
}

func ChatCompletion(prompt string) (string, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("OPENAI_API_KEY is not set")
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
