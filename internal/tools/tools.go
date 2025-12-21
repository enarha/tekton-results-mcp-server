package tools

import (
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/enarha/tekton-results-mcp-server/internal/tektonresults"
)

// Dependencies bundles the shared objects every tool relies on.
type Dependencies struct {
	Service          *tektonresults.Service
	DefaultNamespace string
}

// Add registers all Tekton Results tools with the MCP server.
func Add(s *server.MCPServer, deps Dependencies) error {
	if deps.Service == nil {
		return fmt.Errorf("tekton results service dependency is required")
	}

	tools, err := pipelineRunTools(deps)
	if err != nil {
		return err
	}
	taskTools, err := taskRunTools(deps)
	if err != nil {
		return err
	}

	s.AddTools(append(tools, taskTools...)...)
	return nil
}

func readOnlyAnnotations(title string) mcp.ToolAnnotation {
	return mcp.ToolAnnotation{
		Title:           title,
		ReadOnlyHint:    mcp.ToBoolPtr(true),
		DestructiveHint: mcp.ToBoolPtr(false),
		IdempotentHint:  mcp.ToBoolPtr(true),
		OpenWorldHint:   mcp.ToBoolPtr(true),
	}
}

func normalizeNamespace(input, def string) string {
	ns := strings.TrimSpace(input)
	switch strings.ToLower(ns) {
	case "":
		return def
	case "-", "all", "*":
		return "-"
	default:
		return ns
	}
}
