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

func taskRunTools(deps Dependencies) ([]server.ServerTool, error) {
	return []server.ServerTool{
		newTaskRunListTool(deps),
		newTaskRunGetTool(deps),
		newTaskRunLogsTool(deps),
	}, nil
}

func newTaskRunListTool(deps Dependencies) server.ServerTool {
	namespaceDefault := deps.DefaultNamespace
	if namespaceDefault == "" {
		namespaceDefault = "default"
	}

	tool := mcp.NewTool(
		"taskrun_list",
		mcp.WithDescription("List Tekton TaskRuns stored by the Tekton Results service with optional namespace, label, and name prefix filters."),
		mcp.WithToolAnnotation(readOnlyAnnotations("List TaskRuns")),
		mcp.WithString("namespace",
			mcp.Description("Kubernetes namespace to query. Use '-' to search across all namespaces."),
			mcp.DefaultString(namespaceDefault),
		),
		mcp.WithString("labelSelector",
			mcp.Description("Comma separated key=value selectors that must match run labels."),
			mcp.DefaultString(""),
		),
		mcp.WithString("prefix",
			mcp.Description("Optional TaskRun name prefix to match."),
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

		summaries, err := deps.Service.ListTaskRuns(ctx, opts)
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

func newTaskRunGetTool(deps Dependencies) server.ServerTool {
	namespaceDefault := deps.DefaultNamespace
	if namespaceDefault == "" {
		namespaceDefault = "default"
	}

	tool := mcp.NewTool(
		"taskrun_get",
		mcp.WithDescription("Get a Tekton TaskRun stored in Tekton Results. Provide a name for exact match or combine labelSelector/prefix to narrow results. Returns the full resource in YAML (default) or JSON format."),
		mcp.WithToolAnnotation(readOnlyAnnotations("Get TaskRun")),
		mcp.WithString("name",
			mcp.Description("Exact TaskRun name. Optional if labelSelector/prefix uniquely identify a run."),
			mcp.DefaultString(""),
		),
		mcp.WithString("namespace",
			mcp.Description("Kubernetes namespace that owns the TaskRun. Use '-' to search across namespaces."),
			mcp.DefaultString(namespaceDefault),
		),
		mcp.WithString("labelSelector",
			mcp.Description("Comma separated key=value selectors that must match run labels."),
			mcp.DefaultString(""),
		),
		mcp.WithString("prefix",
			mcp.Description("Optional TaskRun name prefix to disambiguate."),
			mcp.DefaultString(""),
		),
		mcp.WithString("uid",
			mcp.Description("Exact TaskRun UID (unique identifier in Tekton Results database). This is the most efficient way to find a specific run."),
			mcp.DefaultString(""),
		),
		mcp.WithString("output",
			mcp.Description("Return format: 'yaml' (default) or 'json'."),
			mcp.DefaultString("yaml"),
		),
		mcp.WithBoolean("selectLast",
			mcp.Description("If true, automatically select the last (most recent) match when multiple TaskRuns match the filters. Defaults to true."),
			mcp.DefaultBool(true),
		),
	)

	handler := mcp.NewTypedToolHandler(func(ctx context.Context, req mcp.CallToolRequest, args getParams) (*mcp.CallToolResult, error) {
		if args.Name == "" && args.Prefix == "" && args.UID == "" && strings.TrimSpace(args.LabelSelector) == "" {
			return mcp.NewToolResultError("provide at least one of name, prefix, uid, or labelSelector to identify a TaskRun"), nil
		}

		// Default selectLast to true if not explicitly provided
		selectLast := true
		if params, ok := req.Params.Arguments.(map[string]interface{}); ok {
			if val, exists := params["selectLast"]; exists {
				if boolVal, ok := val.(bool); ok {
					selectLast = boolVal
				}
			}
		}

		ns := normalizeNamespace(args.Namespace, namespaceDefault)
		selector := tektonresults.RunSelector{
			Namespace:     ns,
			LabelSelector: args.LabelSelector,
			Prefix:        args.Prefix,
			Name:          args.Name,
			UID:           args.UID,
			SelectLast:    selectLast,
		}

		detail, err := deps.Service.GetTaskRun(ctx, selector)
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

func newTaskRunLogsTool(deps Dependencies) server.ServerTool {
	namespaceDefault := deps.DefaultNamespace
	if namespaceDefault == "" {
		namespaceDefault = "default"
	}

	tool := mcp.NewTool(
		"taskrun_logs",
		mcp.WithDescription("Retrieve stored logs for a completed Tekton TaskRun."),
		mcp.WithToolAnnotation(readOnlyAnnotations("TaskRun Logs")),
		mcp.WithString("name",
			mcp.Description("Exact TaskRun name. Optional if labelSelector/prefix uniquely identify a run."),
			mcp.DefaultString(""),
		),
		mcp.WithString("namespace",
			mcp.Description("Kubernetes namespace for the TaskRun. Use '-' to search all namespaces."),
			mcp.DefaultString(namespaceDefault),
		),
		mcp.WithString("labelSelector",
			mcp.Description("Comma separated key=value selectors that must match run labels."),
			mcp.DefaultString(""),
		),
		mcp.WithString("prefix",
			mcp.Description("Optional TaskRun name prefix when multiple runs share similar names."),
			mcp.DefaultString(""),
		),
		mcp.WithString("uid",
			mcp.Description("Exact TaskRun UID (unique identifier in Tekton Results database). This is the most efficient way to find a specific run."),
			mcp.DefaultString(""),
		),
		mcp.WithBoolean("selectLast",
			mcp.Description("If true, automatically select the last (most recent) match when multiple TaskRuns match the filters. Defaults to true."),
			mcp.DefaultBool(true),
		),
	)

	handler := mcp.NewTypedToolHandler(func(ctx context.Context, req mcp.CallToolRequest, args logsParams) (*mcp.CallToolResult, error) {
		if args.Name == "" && args.Prefix == "" && strings.TrimSpace(args.LabelSelector) == "" {
			return mcp.NewToolResultError("provide at least one of name, prefix, uid, or labelSelector to target a TaskRun"), nil
		}

		// Default selectLast to true if not explicitly provided
		selectLast := true
		if params, ok := req.Params.Arguments.(map[string]interface{}); ok {
			if val, exists := params["selectLast"]; exists {
				if boolVal, ok := val.(bool); ok {
					selectLast = boolVal
				}
			}
		}

		ns := normalizeNamespace(args.Namespace, namespaceDefault)
		selector := tektonresults.RunSelector{
			Namespace:     ns,
			LabelSelector: args.LabelSelector,
			Prefix:        args.Prefix,
			Name:          args.Name,
			UID:           args.UID,
			SelectLast:    selectLast,
		}

		detail, err := deps.Service.GetTaskRun(ctx, selector)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if !detail.Completed() {
			return mcp.NewToolResultError("logs are only available after the TaskRun has completed"), nil
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
