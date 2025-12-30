package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/enarha/tekton-results-mcp-server/internal/tektonresults"
)

// Service interface defines the methods that tools use from tektonresults.Service
type Service interface {
	ListPipelineRuns(ctx context.Context, opts tektonresults.ListOptions) ([]tektonresults.RunSummary, error)
	ListTaskRuns(ctx context.Context, opts tektonresults.ListOptions) ([]tektonresults.RunSummary, error)
	GetPipelineRun(ctx context.Context, selector tektonresults.RunSelector) (*tektonresults.RunDetail, error)
	GetTaskRun(ctx context.Context, selector tektonresults.RunSelector) (*tektonresults.RunDetail, error)
	FetchLogs(ctx context.Context, recordName string) (string, error)
}

// Dependencies bundles the shared objects every tool relies on.
type Dependencies struct {
	Service          Service
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
