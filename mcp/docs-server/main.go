// Command docs-server is a Model Context Protocol server that exposes the
// GoFr documentation to AI agents as callable tools.
//
// It speaks MCP over stdio, which is what every current MCP client
// (Claude Code, Cursor, Codex, Continue) launches by default:
//
//	go run gofr.dev/mcp/docs-server@latest
//
// Content comes from https://gofr.dev/llms-full.txt — the concatenated
// dump the website publishes on every deploy — fetched once per process
// and cached in memory. Fetching rather than embedding means the server
// serves current documentation without being re-released alongside it.
//
// Deliberately stdlib-only: this lives in the root gofr.dev module, and
// a docs tool is not a good reason to add a dependency that every
// downstream `go get gofr.dev` would then carry.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"
)

const (
	docsURL        = "https://gofr.dev/llms-full.txt"
	siteURL        = "https://gofr.dev"
	fetchTimeout   = 30 * time.Second
	protocolVer    = "2024-11-05"
	serverName     = "gofr-docs"
	serverVersion  = "1.0.0"
	defaultResults = 10
	excerptRunes   = 400
)

func main() {
	srv := newServer(&http.Client{Timeout: fetchTimeout}, docsURL)

	if err := srv.serve(context.Background(), os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "docs-server: %v\n", err)
		os.Exit(1)
	}
}
