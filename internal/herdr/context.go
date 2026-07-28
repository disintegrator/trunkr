package herdr

import (
	"encoding/json"
	"fmt"
	"os"
)

// ContextEnvVar carries the JSON-serialized invocation context herdr passes
// to every runtime plugin command.
const ContextEnvVar = "HERDR_PLUGIN_CONTEXT_JSON"

// InvocationContext is the HERDR_PLUGIN_CONTEXT_JSON payload, trimmed to the
// fields trunkr consumes. Every field is nullable in herdr's schema; nulls
// decode to empty strings.
type InvocationContext struct {
	WorkspaceID    string `json:"workspace_id"`
	WorkspaceCwd   string `json:"workspace_cwd"`
	TabID          string `json:"tab_id"`
	FocusedPaneID  string `json:"focused_pane_id"`
	FocusedPaneCwd string `json:"focused_pane_cwd"`
}

// ParseContext decodes an invocation context payload. An empty payload is a
// valid, empty context — herdr always sets the variable, but actions must
// degrade cleanly when run outside herdr.
func ParseContext(raw string) (InvocationContext, error) {
	var ic InvocationContext
	if raw == "" {
		return ic, nil
	}
	if err := json.Unmarshal([]byte(raw), &ic); err != nil {
		return InvocationContext{}, fmt.Errorf("parsing %s: %w", ContextEnvVar, err)
	}
	return ic, nil
}

// ContextFromEnv parses the invocation context from the environment.
func ContextFromEnv() (InvocationContext, error) {
	return ParseContext(os.Getenv(ContextEnvVar))
}

// TargetDir is the repo/workspace directory an action should operate on: the
// focused pane's cwd when known, else the workspace cwd. Empty when the
// invocation has no directory context.
func (ic InvocationContext) TargetDir() string {
	if ic.FocusedPaneCwd != "" {
		return ic.FocusedPaneCwd
	}
	return ic.WorkspaceCwd
}
