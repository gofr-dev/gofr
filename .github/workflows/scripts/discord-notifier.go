package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	discordLimit = 2000
	chunkSize    = 1900 // Leave buffer for "Part X" headers
)

type DiscordPayload struct {
	Content string `json:"content"`
}

func main() {
	webhookURL := os.Getenv("DISCORD_WEBHOOK")
	if webhookURL == "" {
		fmt.Println("Error: DISCORD_WEBHOOK environment variable is not set")
		os.Exit(1)
	}

	releaseBody := os.Getenv("RELEASE_BODY")
	releaseName := os.Getenv("RELEASE_NAME")
	releaseTag := os.Getenv("RELEASE_TAG")
	releaseURL := os.Getenv("RELEASE_URL")

	if releaseBody == "" {
		releaseBody = "No release notes provided."
	}

	// 1. Construct Header
	header := fmt.Sprintf("🚀 **New Release: %s**\n\n", releaseName)
	header += fmt.Sprintf("📦 **Tag:** `%s`\n", releaseTag)
	header += fmt.Sprintf("🔗 **Link:** %s\n\n", releaseURL)
	header += "---\n\n"

	fullMessage := header + releaseBody

	// 2. Split Message
	chunks := splitMessage(fullMessage, chunkSize)

	// 3. Send Chunks
	for i, chunk := range chunks {
		prefix := ""
		if len(chunks) > 1 {
			prefix = fmt.Sprintf("📝 **Release Notes (Part %d/%d):**\n\n", i+1, len(chunks))
			// If it's the first chunk, we don't need the "Part 1" prefix if the header is already there,
			// but for consistency in multi-part messages, it's often good.
			// However, the header itself is distinct. Let's only add "Part X" if i > 0 or if we want to be explicit.
			// Given the user's request for "generalizing", let's keep it simple:
			// If > 1 chunk, prepend Part info to ALL chunks to avoid confusion.
		}

		// Special handling: The first chunk already has the nice header.
		// If we prepend "Part 1/X" before "🚀 New Release", it looks ugly.
		// Logic:
		// Chunk 0: [Header] + [Body Part 1]
		// Chunk 1: [Part 2/X] + [Body Part 2]
		finalContent := chunk
		if i > 0 {
			finalContent = prefix + chunk
		}

		err := sendToDiscord(webhookURL, finalContent)
		if err != nil {
			fmt.Printf("Failed to send chunk %d: %v\n", i+1, err)
			os.Exit(1)
		}
		
		// Rate limit prevention
		time.Sleep(500 * time.Millisecond)
	}

	fmt.Println("Successfully sent release notes to Discord!")
}

// splitMessage splits the text into chunks respecting UTF-8 and Markdown code blocks
func splitMessage(text string, limit int) []string {
	var chunks []string
	runes := []rune(text)
	totalLen := len(runes)

	if totalLen <= limit {
		return []string{text}
	}

	currentChunk := ""
	inCodeBlock := false

	// We'll iterate line by line to keep formatting clean
	lines := strings.Split(text, "\n")
	
	for _, line := range lines {
		// Check for code block toggles
		// Note: This is a simple check. It handles ```go, ``` etc.
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inCodeBlock = !inCodeBlock
		}

		// Calculate potential length
		// +1 for newline we stripped
		potentialLen := utf8.RuneCountInString(currentChunk) + utf8.RuneCountInString(line) + 1

		if potentialLen > limit {
			// Must split here
			if inCodeBlock {
				// Close block in current chunk
				currentChunk += "\n```"
				chunks = append(chunks, currentChunk)
				
				// Re-open block in next chunk (assuming generic block for simplicity, 
				// ideally we'd track the language)
				currentChunk = "```\n" + line
			} else {
				chunks = append(chunks, currentChunk)
				currentChunk = line
			}
		} else {
			if currentChunk == "" {
				currentChunk = line
			} else {
				currentChunk += "\n" + line
			}
		}
	}

	if currentChunk != "" {
		chunks = append(chunks, currentChunk)
	}

	return chunks
}

func sendToDiscord(webhookURL, content string) error {
	payload := DiscordPayload{Content: content}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// Retry logic
	for i := 0; i < 3; i++ {
		resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(jsonPayload))
		if err == nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
			resp.Body.Close()
			return nil
		}
		
		if resp != nil {
			fmt.Printf("Attempt %d failed: Status %d\n", i+1, resp.StatusCode)
			resp.Body.Close()
		} else {
			fmt.Printf("Attempt %d failed: %v\n", i+1, err)
		}
		
		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("failed after 3 attempts")
}
