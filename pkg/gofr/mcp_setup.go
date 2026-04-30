package gofr

import (
	"strings"

	"gofr.dev/pkg/gofr/mcp"
)

// MCP_ENABLED accepts:
//
//	"true"  – expose GET endpoints as MCP tools (safe default).
//	"full"  – also expose POST/PUT/PATCH/DELETE. Read this as
//	          "I'm okay with an LLM mutating data via MCP."
//
// MCP_PERSIST_PATH (optional) controls where learned schemas are
// cached between runs. Defaults to ./.gofr-mcp-schemas.json. Set
// to "-" to disable persistence entirely.
const (
	mcpEnabledKey      = "MCP_ENABLED"
	mcpPersistPathKey  = "MCP_PERSIST_PATH"
	mcpDefaultPersist  = "./.gofr-mcp-schemas.json"
	mcpServerNameKey   = "APP_NAME"
	mcpFallbackName    = "gofr-app"
	mcpFallbackVersion = "dev"
	mcpServerVersion   = "APP_VERSION"
)

// initMCP wires up the learner if MCP_ENABLED is set. It is called
// from New() so that any subsequent app.GET/POST/... calls can pick
// up the learner via the handler struct. Returns nil to mean
// "MCP off" — callers must handle the nil case.
func (a *App) initMCP() {
	mode := strings.ToLower(strings.TrimSpace(a.Config.Get(mcpEnabledKey)))
	if mode != "true" && mode != "full" {
		return
	}

	persistPath := a.Config.GetOrDefault(mcpPersistPathKey, mcpDefaultPersist)
	if persistPath == "-" {
		persistPath = ""
	}

	a.mcpLearner = mcp.NewLearner(persistPath)
	a.mcpAllowMutations = mode == "full"
}

// mcpServerInfo returns name + version reported in the MCP initialize
// handshake. Defaults are sensible if the app didn't bother setting
// APP_NAME / APP_VERSION.
func (a *App) mcpServerInfo() (name, version string) {
	name = a.Config.GetOrDefault(mcpServerNameKey, mcpFallbackName)
	version = a.Config.GetOrDefault(mcpServerVersion, mcpFallbackVersion)

	return name, version
}
