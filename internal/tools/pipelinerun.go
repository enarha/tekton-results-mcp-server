package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/enarha/tekton-results-mcp-server/internal/tektonresults"
)

const (
	defaultListLimit = 50
	maxListLimit     = 200
)

type listParams struct {
	Namespace     string `json:"namespace"`
	LabelSelector string `json:"labelSelector"`
	Prefix        string `json:"prefix"`
	Limit         int    `json:"limit"`
}

type describeParams struct {
	Namespace     string `json:"namespace"`
	LabelSelector string `json:"labelSelector"`
	Prefix        string `json:"prefix"`
	Name          string `json:"name"`
	Output        string `json:"output"`
}

type logsParams struct {
	Namespace     string `json:"namespace"`
	LabelSelector string `json:"labelSelector"`
	Prefix        string `json:"prefix"`
	Name          string `json:"name"`
}

func pipelineRunTools(deps Dependencies) ([]server.ServerTool, error) {
	return []server.ServerTool{
		newPipelineRunListTool(deps),
		newPipelineRunDescribeTool(deps),
		newPipelineRunLogsTool(deps),
	}, nil
}

func newPipelineRunListTool(deps Dependencies) server.ServerTool {
	namespaceDefault := deps.DefaultNamespace
	if namespaceDefault == "" {
		namespaceDefault = "default"
	}

	tool := mcp.NewTool(
		"pipelinerun_list",
		mcp.WithDescription("List Tekton PipelineRuns stored by the Tekton Results service with optional namespace, label, and name prefix filters."),
		mcp.WithToolAnnotation(readOnlyAnnotations("List PipelineRuns")),
		mcp.WithString("namespace",
			mcp.Description("Kubernetes namespace to query. Use '-' to search across all namespaces."),
			mcp.DefaultString(namespaceDefault),
		),
		mcp.WithString("labelSelector",
			mcp.Description("Comma separated key=value selectors that must match run labels."),
			mcp.DefaultString(""),
		),
		mcp.WithString("prefix",
			mcp.Description("Optional PipelineRun name prefix to match."),
			mcp.DefaultString(""),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of records to return (1-200)."),
			mcp.DefaultNumber(defaultListLimit),
			mcp.Min(1),
			mcp.Max(maxListLimit),
		),
	)

	handler := mcp.NewTypedToolHandler(func(ctx context.Context, _ mcp.CallToolRequest, args listParams) (*mcp.CallToolResult, error) {
		ns := normalizeNamespace(args.Namespace, namespaceDefault)
		opts := tektonresults.ListOptions{
			Namespace:     ns,
			LabelSelector: args.LabelSelector,
			Prefix:        args.Prefix,
			Limit:         sanitizeLimit(args.Limit),
		}

		summaries, err := deps.Service.ListPipelineRuns(ctx, opts)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		payload, err := json.MarshalIndent(summaries, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("format response: %v", err)), nil
		}
		return mcp.NewToolResultText(string(payload)), nil
	})

	return server.ServerTool{
		Tool:    tool,
		Handler: handler,
	}
}

func newPipelineRunDescribeTool(deps Dependencies) server.ServerTool {
	namespaceDefault := deps.DefaultNamespace
	if namespaceDefault == "" {
		namespaceDefault = "default"
	}

	tool := mcp.NewTool(
		"pipelinerun_describe",
		mcp.WithDescription("Describe a Tekton PipelineRun stored in Tekton Results. Provide a name for exact match or combine labelSelector/prefix to narrow results."),
		mcp.WithToolAnnotation(readOnlyAnnotations("Describe PipelineRun")),
		mcp.WithString("name",
			mcp.Description("Exact PipelineRun name. Optional if labelSelector/prefix uniquely identify a run."),
			mcp.DefaultString(""),
		),
		mcp.WithString("namespace",
			mcp.Description("Kubernetes namespace that owns the PipelineRun. Use '-' to search across namespaces."),
			mcp.DefaultString(namespaceDefault),
		),
		mcp.WithString("labelSelector",
			mcp.Description("Comma separated key=value selectors that must match run labels."),
			mcp.DefaultString(""),
		),
		mcp.WithString("prefix",
			mcp.Description("Optional PipelineRun name prefix to disambiguate."),
			mcp.DefaultString(""),
		),
		mcp.WithString("output",
			mcp.Description("Return format: 'yaml' (default) or 'json'."),
			mcp.DefaultString("yaml"),
		),
	)

	handler := mcp.NewTypedToolHandler(func(ctx context.Context, _ mcp.CallToolRequest, args describeParams) (*mcp.CallToolResult, error) {
		if args.Name == "" && args.Prefix == "" && strings.TrimSpace(args.LabelSelector) == "" {
			return mcp.NewToolResultError("provide at least one of name, prefix, or labelSelector to identify a PipelineRun"), nil
		}

		ns := normalizeNamespace(args.Namespace, namespaceDefault)
		selector := tektonresults.RunSelector{
			Namespace:     ns,
			LabelSelector: args.LabelSelector,
			Prefix:        args.Prefix,
			Name:          args.Name,
		}

		detail, err := deps.Service.GetPipelineRun(ctx, selector)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		output := strings.ToLower(strings.TrimSpace(args.Output))
		if output == "" {
			output = "yaml"
		}

		formatted, err := detail.Format(output)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(formatted), nil
	})

	return server.ServerTool{
		Tool:    tool,
		Handler: handler,
	}
}

func newPipelineRunLogsTool(deps Dependencies) server.ServerTool {
	namespaceDefault := deps.DefaultNamespace
	if namespaceDefault == "" {
		namespaceDefault = "default"
	}

	tool := mcp.NewTool(
		"pipelinerun_logs",
		mcp.WithDescription("Retrieve stored logs for a completed Tekton PipelineRun."),
		mcp.WithToolAnnotation(readOnlyAnnotations("PipelineRun Logs")),
		mcp.WithString("name",
			mcp.Description("Exact PipelineRun name. Optional if labelSelector/prefix uniquely identify a run."),
			mcp.DefaultString(""),
		),
		mcp.WithString("namespace",
			mcp.Description("Kubernetes namespace for the PipelineRun. Use '-' to search all namespaces."),
			mcp.DefaultString(namespaceDefault),
		),
		mcp.WithString("labelSelector",
			mcp.Description("Comma separated key=value selectors that must match run labels."),
			mcp.DefaultString(""),
		),
		mcp.WithString("prefix",
			mcp.Description("Optional PipelineRun name prefix when multiple runs share similar names."),
			mcp.DefaultString(""),
		),
	)

	handler := mcp.NewTypedToolHandler(func(ctx context.Context, _ mcp.CallToolRequest, args logsParams) (*mcp.CallToolResult, error) {
		if args.Name == "" && args.Prefix == "" && strings.TrimSpace(args.LabelSelector) == "" {
			return mcp.NewToolResultError("provide at least one of name, prefix, or labelSelector to target a PipelineRun"), nil
		}

		ns := normalizeNamespace(args.Namespace, namespaceDefault)
		selector := tektonresults.RunSelector{
			Namespace:     ns,
			LabelSelector: args.LabelSelector,
			Prefix:        args.Prefix,
			Name:          args.Name,
		}

		detail, err := deps.Service.GetPipelineRun(ctx, selector)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if !detail.Completed() {
			return mcp.NewToolResultError("logs are only available after the PipelineRun has completed"), nil
		}

		logs, err := deps.Service.FetchLogs(ctx, detail.RecordName)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(logs), nil
	})

	return server.ServerTool{
		Tool:    tool,
		Handler: handler,
	}
}

func sanitizeLimit(limit int) int {
	if limit <= 0 {
		return defaultListLimit
	}
	if limit > maxListLimit {
		return maxListLimit
	}
	return limit
}
