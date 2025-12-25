package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
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
	SelectLast    bool   `json:"selectLast"`
}

type logsParams struct {
	Namespace     string `json:"namespace"`
	LabelSelector string `json:"labelSelector"`
	Prefix        string `json:"prefix"`
	Name          string `json:"name"`
	SelectLast    bool   `json:"selectLast"`
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
		mcp.WithBoolean("selectLast",
			mcp.Description("If true, automatically select the last (most recent) match when multiple PipelineRuns match the filters. Defaults to true."),
			mcp.DefaultBool(true),
		),
	)

	handler := mcp.NewTypedToolHandler(func(ctx context.Context, req mcp.CallToolRequest, args describeParams) (*mcp.CallToolResult, error) {
		if args.Name == "" && args.Prefix == "" && strings.TrimSpace(args.LabelSelector) == "" {
			return mcp.NewToolResultError("provide at least one of name, prefix, or labelSelector to identify a PipelineRun"), nil
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
			SelectLast:    selectLast,
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
		mcp.WithBoolean("selectLast",
			mcp.Description("If true, automatically select the last (most recent) match when multiple PipelineRuns match the filters. Defaults to true."),
			mcp.DefaultBool(true),
		),
	)

	handler := mcp.NewTypedToolHandler(func(ctx context.Context, req mcp.CallToolRequest, args logsParams) (*mcp.CallToolResult, error) {
		if args.Name == "" && args.Prefix == "" && strings.TrimSpace(args.LabelSelector) == "" {
			return mcp.NewToolResultError("provide at least one of name, prefix, or labelSelector to target a PipelineRun"), nil
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
			SelectLast:    selectLast,
		}

		detail, err := deps.Service.GetPipelineRun(ctx, selector)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if !detail.Completed() {
			return mcp.NewToolResultError("logs are only available after the PipelineRun has completed"), nil
		}

		// Fetch all TaskRuns for this PipelineRun using the UID (result ID)
		// This is more reliable than using the name, as names can be reused over time
		taskRunOpts := tektonresults.ListOptions{
			Namespace:     ns,
			LabelSelector: fmt.Sprintf("tekton.dev/pipelineRunUID=%s", detail.Summary.UID),
			Limit:         200, // Maximum allowed
		}

		taskRuns, err := deps.Service.ListTaskRuns(ctx, taskRunOpts)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to list TaskRuns: %v", err)), nil
		}

		if len(taskRuns) == 0 {
			return mcp.NewToolResultText("No TaskRuns found for this PipelineRun"), nil
		}

		// Sort TaskRuns by completion time, then by start time
		sort.Slice(taskRuns, func(i, j int) bool {
			// If both have completion times, sort by completion time
			if taskRuns[i].CompletionTime != nil && taskRuns[j].CompletionTime != nil {
				if !taskRuns[i].CompletionTime.Equal(taskRuns[j].CompletionTime) {
					return taskRuns[i].CompletionTime.Before(taskRuns[j].CompletionTime)
				}
			}
			// Fall back to start time
			if taskRuns[i].StartTime != nil && taskRuns[j].StartTime != nil {
				return taskRuns[i].StartTime.Before(taskRuns[j].StartTime)
			}
			return false
		})

		// Fetch logs for each TaskRun
		var logsBuilder strings.Builder
		for i, tr := range taskRuns {
			if i > 0 {
				logsBuilder.WriteString("\n\n")
			}
			logsBuilder.WriteString("========================================\n")
			logsBuilder.WriteString(fmt.Sprintf("TaskRun: %s\n", tr.Name))
			logsBuilder.WriteString(fmt.Sprintf("Status: %s", tr.Reason))
			if tr.StartTime != nil {
				logsBuilder.WriteString(fmt.Sprintf(" | Started: %s", tr.StartTime.Format("2006-01-02T15:04:05Z")))
			}
			if tr.CompletionTime != nil {
				logsBuilder.WriteString(fmt.Sprintf(" | Completed: %s", tr.CompletionTime.Format("2006-01-02T15:04:05Z")))
			}
			logsBuilder.WriteString("\n========================================\n")

			taskLogs, err := deps.Service.FetchLogs(ctx, tr.RecordName)
			if err != nil {
				logsBuilder.WriteString(fmt.Sprintf("Error fetching logs: %v\n", err))
			} else if taskLogs == "" {
				logsBuilder.WriteString("(no logs available)\n")
			} else {
				logsBuilder.WriteString(taskLogs)
				// Ensure logs end with newline
				if !strings.HasSuffix(taskLogs, "\n") {
					logsBuilder.WriteString("\n")
				}
			}
		}

		return mcp.NewToolResultText(logsBuilder.String()), nil
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
