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
		lineLen := utf8.RuneCountInString(line)
		chunkLen := utf8.RuneCountInString(currentChunk)
		
		// Check for code block toggles *before* processing to know if we are entering/exiting
		// Actually, standard markdown: if line starts with ```, it toggles.
		// We need to know if we are *currently* in a block to decide overhead.
		// But the toggle happens *on* this line.
		// If line is "```go", we enter. If line is "```", we exit.
		isToggle := strings.HasPrefix(strings.TrimSpace(line), "```")
		
		// Overhead calculation
		// If we split now:
		// 1. We need to add `\n` separator (1 char)
		// 2. If inCodeBlock (and not toggling out), we need `\n` + ` ``` ` (4 chars) to close current.
		overhead := 1
		if inCodeBlock && !isToggle {
			overhead += 4 // "\n```"
		}

		// Check if adding this line exceeds limit
		if chunkLen+lineLen+overhead > limit {
			// Must split
			if currentChunk != "" {
				if inCodeBlock && !isToggle {
					// Close block in current chunk
					currentChunk += "\n```"
					chunks = append(chunks, currentChunk)
					
					// Re-open in next chunk
					currentChunk = "```\n"
				} else {
					chunks = append(chunks, currentChunk)
					currentChunk = ""
				}
			}

			// Now handle the line itself. If it's still too big (even for a new chunk), we must split it.
			// Note: If we just re-opened a code block, currentChunk is "```\n" (4 chars).
			// We need to fit `line` into `limit - len(currentChunk)`.
			
			// If the line fits in a fresh chunk (plus potential code block header), just add it.
			if utf8.RuneCountInString(currentChunk)+lineLen <= limit {
				if currentChunk == "" {
					currentChunk = line
				} else {
					currentChunk += line // No newline needed if we just reset, but wait...
					// If we reset currentChunk to "", we don't need newline.
					// If we reset to "```\n", we already have newline.
				}
			} else {
				// Line is too long. Split it.
				// We will split by characters for simplicity, or spaces if we want to be fancy.
				// Given the constraints and "generalize" request, let's do simple char split for now 
				// to ensure correctness, as word splitting is complex with Markdown.
				
				remaining := line
				for len(remaining) > 0 {
					// How much space do we have?
					space := limit - utf8.RuneCountInString(currentChunk)
					if space <= 0 {
						// Should not happen if logic is correct, but safety:
						chunks = append(chunks, currentChunk)
						currentChunk = ""
						space = limit
					}
					
					if utf8.RuneCountInString(remaining) <= space {
						currentChunk += remaining
						remaining = ""
					} else {
						// Take what fits
						// Need to be careful with multi-byte runes
						runes := []rune(remaining)
						take := runes[:space]
						currentChunk += string(take)
						remaining = string(runes[space:])
						
						// Chunk is full
						chunks = append(chunks, currentChunk)
						currentChunk = ""
					}
				}
			}
		} else {
			// Fits
			if currentChunk == "" {
				currentChunk = line
			} else {
				currentChunk += "\n" + line
			}
		}

		// Update state for next line
		if isToggle {
			inCodeBlock = !inCodeBlock
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
