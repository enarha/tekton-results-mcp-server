package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/enarha/tekton-results-mcp-server/internal/tektonresults"
	"github.com/mark3labs/mcp-go/mcp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// mockTaskRunService is a mock implementation of Service interface for testing TaskRun tools
type mockTaskRunService struct {
	listPipelineRunsFunc func(ctx context.Context, opts tektonresults.ListOptions) ([]tektonresults.RunSummary, error)
	listTaskRunsFunc     func(ctx context.Context, opts tektonresults.ListOptions) ([]tektonresults.RunSummary, error)
	getPipelineRunFunc   func(ctx context.Context, selector tektonresults.RunSelector) (*tektonresults.RunDetail, error)
	getTaskRunFunc       func(ctx context.Context, selector tektonresults.RunSelector) (*tektonresults.RunDetail, error)
	fetchLogsFunc        func(ctx context.Context, recordName string) (string, error)
}

func (m *mockTaskRunService) ListPipelineRuns(ctx context.Context, opts tektonresults.ListOptions) ([]tektonresults.RunSummary, error) {
	if m.listPipelineRunsFunc != nil {
		return m.listPipelineRunsFunc(ctx, opts)
	}
	return nil, nil
}

func (m *mockTaskRunService) ListTaskRuns(ctx context.Context, opts tektonresults.ListOptions) ([]tektonresults.RunSummary, error) {
	if m.listTaskRunsFunc != nil {
		return m.listTaskRunsFunc(ctx, opts)
	}
	return nil, nil
}

func (m *mockTaskRunService) GetPipelineRun(ctx context.Context, selector tektonresults.RunSelector) (*tektonresults.RunDetail, error) {
	if m.getPipelineRunFunc != nil {
		return m.getPipelineRunFunc(ctx, selector)
	}
	return nil, nil
}

func (m *mockTaskRunService) GetTaskRun(ctx context.Context, selector tektonresults.RunSelector) (*tektonresults.RunDetail, error) {
	if m.getTaskRunFunc != nil {
		return m.getTaskRunFunc(ctx, selector)
	}
	return nil, nil
}

func (m *mockTaskRunService) FetchLogs(ctx context.Context, recordName string) (string, error) {
	if m.fetchLogsFunc != nil {
		return m.fetchLogsFunc(ctx, recordName)
	}
	return "", nil
}

func TestTaskRunList_DefaultParameters(t *testing.T) {
	mock := &mockTaskRunService{
		listTaskRunsFunc: func(ctx context.Context, opts tektonresults.ListOptions) ([]tektonresults.RunSummary, error) {
			if opts.Namespace != "test-ns" {
				t.Errorf("Expected namespace 'test-ns', got %s", opts.Namespace)
			}
			if opts.Limit != 50 {
				t.Errorf("Expected limit 50, got %d", opts.Limit)
			}
			return []tektonresults.RunSummary{
				{Name: "tr-1", Namespace: "test-ns"},
			}, nil
		},
	}

	deps := Dependencies{Service: mock, DefaultNamespace: "test-ns"}
	tool := newTaskRunListTool(deps)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{}

	result, err := tool.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("Handler failed: %v", err)
	}

	if result.IsError {
		t.Fatalf("Result is error: %s", getTextFromResult(result))
	}

	text := getTextFromResult(result)
	if !strings.Contains(text, "tr-1") {
		t.Errorf("Response doesn't contain expected run: %s", text)
	}
}

func TestTaskRunList_CustomParameters(t *testing.T) {
	mock := &mockTaskRunService{
		listTaskRunsFunc: func(ctx context.Context, opts tektonresults.ListOptions) ([]tektonresults.RunSummary, error) {
			if opts.Namespace != "-" {
				t.Errorf("Expected namespace '-', got %s", opts.Namespace)
			}
			if opts.LabelSelector != "type=test" {
				t.Errorf("Expected labelSelector 'type=test', got %s", opts.LabelSelector)
			}
			if opts.Prefix != "my-task" {
				t.Errorf("Expected prefix 'my-task', got %s", opts.Prefix)
			}
			if opts.Limit != 15 {
				t.Errorf("Expected limit 15, got %d", opts.Limit)
			}
			return []tektonresults.RunSummary{}, nil
		},
	}

	deps := Dependencies{Service: mock, DefaultNamespace: "default"}
	tool := newTaskRunListTool(deps)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"namespace":     "-",
		"labelSelector": "type=test",
		"prefix":        "my-task",
		"limit":         float64(15),
	}

	_, err := tool.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("Handler failed: %v", err)
	}
}

func TestTaskRunList_NamespaceNormalization(t *testing.T) {
	tests := []struct {
		name              string
		inputNamespace    string
		expectedNamespace string
	}{
		{"empty string", "", "default"},
		{"all keyword", "all", "-"},
		{"asterisk", "*", "-"},
		{"dash", "-", "-"},
		{"whitespace", "  prod  ", "prod"},
		{"normal namespace", "test-ns", "test-ns"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockTaskRunService{
				listTaskRunsFunc: func(ctx context.Context, opts tektonresults.ListOptions) ([]tektonresults.RunSummary, error) {
					if opts.Namespace != tt.expectedNamespace {
						t.Errorf("Expected namespace %s, got %s", tt.expectedNamespace, opts.Namespace)
					}
					return []tektonresults.RunSummary{}, nil
				},
			}

			deps := Dependencies{Service: mock, DefaultNamespace: "default"}
			tool := newTaskRunListTool(deps)

			req := mcp.CallToolRequest{}
			req.Params.Arguments = map[string]any{"namespace": tt.inputNamespace}

			_, err := tool.Handler(context.Background(), req)
			if err != nil {
				t.Fatalf("Handler failed: %v", err)
			}
		})
	}
}

func TestTaskRunList_ServiceError(t *testing.T) {
	mock := &mockTaskRunService{
		listTaskRunsFunc: func(ctx context.Context, opts tektonresults.ListOptions) ([]tektonresults.RunSummary, error) {
			return nil, &testError{msg: "connection refused"}
		},
	}

	deps := Dependencies{Service: mock, DefaultNamespace: "default"}
	tool := newTaskRunListTool(deps)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{}

	result, err := tool.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("Handler failed: %v", err)
	}

	if !result.IsError {
		t.Fatal("Expected error result")
	}

	if !strings.Contains(getTextFromResult(result), "connection refused") {
		t.Errorf("Error message doesn't contain expected text: %s", getTextFromResult(result))
	}
}

func TestTaskRunGet_ByName(t *testing.T) {
	mock := &mockTaskRunService{
		getTaskRunFunc: func(ctx context.Context, selector tektonresults.RunSelector) (*tektonresults.RunDetail, error) {
			if selector.Name != "my-task" {
				t.Errorf("Expected name 'my-task', got %s", selector.Name)
			}
			if selector.Namespace != "test-ns" {
				t.Errorf("Expected namespace 'test-ns', got %s", selector.Namespace)
			}
			if !selector.SelectLast {
				t.Error("Expected SelectLast to be true by default")
			}
			return &tektonresults.RunDetail{
				Raw: json.RawMessage(`{"metadata":{"name":"my-task"}}`),
			}, nil
		},
	}

	deps := Dependencies{Service: mock, DefaultNamespace: "test-ns"}
	tool := newTaskRunGetTool(deps)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"name": "my-task",
	}

	result, err := tool.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("Handler failed: %v", err)
	}

	if result.IsError {
		t.Fatalf("Result is error: %s", getTextFromResult(result))
	}

	text := getTextFromResult(result)
	if !strings.Contains(text, "my-task") {
		t.Errorf("Response doesn't contain expected name: %s", text)
	}
}

func TestTaskRunGet_ByUID(t *testing.T) {
	mock := &mockTaskRunService{
		getTaskRunFunc: func(ctx context.Context, selector tektonresults.RunSelector) (*tektonresults.RunDetail, error) {
			if selector.UID != "task-uid-789" {
				t.Errorf("Expected UID 'task-uid-789', got %s", selector.UID)
			}
			if selector.Name != "" {
				t.Error("Expected Name to be empty when querying by UID")
			}
			return &tektonresults.RunDetail{
				Raw: json.RawMessage(`{"metadata":{"uid":"task-uid-789"}}`),
			}, nil
		},
	}

	deps := Dependencies{Service: mock, DefaultNamespace: "default"}
	tool := newTaskRunGetTool(deps)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"uid": "task-uid-789",
	}

	result, err := tool.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("Handler failed: %v", err)
	}

	if result.IsError {
		t.Fatalf("Result is error: %s", getTextFromResult(result))
	}
}

func TestTaskRunGet_WithLabelSelectorAndPrefix(t *testing.T) {
	mock := &mockTaskRunService{
		getTaskRunFunc: func(ctx context.Context, selector tektonresults.RunSelector) (*tektonresults.RunDetail, error) {
			if selector.LabelSelector != "app=web,tier=frontend" {
				t.Errorf("Expected labelSelector 'app=web,tier=frontend', got %s", selector.LabelSelector)
			}
			if selector.Prefix != "web-task" {
				t.Errorf("Expected prefix 'web-task', got %s", selector.Prefix)
			}
			return &tektonresults.RunDetail{
				Raw: json.RawMessage(`{"metadata":{"name":"web-task-1"}}`),
			}, nil
		},
	}

	deps := Dependencies{Service: mock, DefaultNamespace: "default"}
	tool := newTaskRunGetTool(deps)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"labelSelector": "app=web,tier=frontend",
		"prefix":        "web-task",
	}

	_, err := tool.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("Handler failed: %v", err)
	}
}

func TestTaskRunGet_OutputFormats(t *testing.T) {
	tests := []struct {
		name           string
		outputFormat   string
		expectedInText string
	}{
		{"default yaml", "", "metadata:"},
		{"explicit yaml", "yaml", "metadata:"},
		{"json format", "json", `"metadata"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockTaskRunService{
				getTaskRunFunc: func(ctx context.Context, selector tektonresults.RunSelector) (*tektonresults.RunDetail, error) {
					return &tektonresults.RunDetail{
						Raw: json.RawMessage(`{"metadata":{"name":"test"}}`),
					}, nil
				},
			}

			deps := Dependencies{Service: mock, DefaultNamespace: "default"}
			tool := newTaskRunGetTool(deps)

			args := map[string]any{"name": "test"}
			if tt.outputFormat != "" {
				args["output"] = tt.outputFormat
			}

			req := mcp.CallToolRequest{}
			req.Params.Arguments = args

			result, err := tool.Handler(context.Background(), req)
			if err != nil {
				t.Fatalf("Handler failed: %v", err)
			}

			text := getTextFromResult(result)
			if !strings.Contains(text, tt.expectedInText) {
				t.Errorf("Expected output to contain %s, got: %s", tt.expectedInText, text)
			}
		})
	}
}

func TestTaskRunGet_SelectLastParameter(t *testing.T) {
	tests := []struct {
		name               string
		selectLastValue    any
		expectedSelectLast bool
	}{
		{"default (not provided)", nil, true},
		{"explicitly true", true, true},
		{"explicitly false", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockTaskRunService{
				getTaskRunFunc: func(ctx context.Context, selector tektonresults.RunSelector) (*tektonresults.RunDetail, error) {
					if selector.SelectLast != tt.expectedSelectLast {
						t.Errorf("Expected SelectLast to be %v, got %v", tt.expectedSelectLast, selector.SelectLast)
					}
					return &tektonresults.RunDetail{
						Raw: json.RawMessage(`{"metadata":{"name":"test"}}`),
					}, nil
				},
			}

			deps := Dependencies{Service: mock, DefaultNamespace: "default"}
			tool := newTaskRunGetTool(deps)

			args := map[string]any{"name": "test"}
			if tt.selectLastValue != nil {
				args["selectLast"] = tt.selectLastValue
			}

			req := mcp.CallToolRequest{}
			req.Params.Arguments = args

			_, err := tool.Handler(context.Background(), req)
			if err != nil {
				t.Fatalf("Handler failed: %v", err)
			}
		})
	}
}

func TestTaskRunGet_ServiceError(t *testing.T) {
	mock := &mockTaskRunService{
		getTaskRunFunc: func(ctx context.Context, selector tektonresults.RunSelector) (*tektonresults.RunDetail, error) {
			return nil, &testError{msg: "taskrun not found"}
		},
	}

	deps := Dependencies{Service: mock, DefaultNamespace: "default"}
	tool := newTaskRunGetTool(deps)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"name": "missing"}

	result, err := tool.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("Handler failed: %v", err)
	}

	if !result.IsError {
		t.Fatal("Expected error result")
	}

	if !strings.Contains(getTextFromResult(result), "taskrun not found") {
		t.Errorf("Error message doesn't contain expected text: %s", getTextFromResult(result))
	}
}

func TestTaskRunLogs_ByName(t *testing.T) {
	completionTime := metav1.NewTime(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	
	var getTaskRunCalled bool
	mock := &mockTaskRunService{
		getTaskRunFunc: func(ctx context.Context, selector tektonresults.RunSelector) (*tektonresults.RunDetail, error) {
			getTaskRunCalled = true
			if selector.Name != "my-task" {
				t.Errorf("Expected name 'my-task', got %s", selector.Name)
			}
			return &tektonresults.RunDetail{
				Summary: tektonresults.RunSummary{
					RecordName:     "test-ns/results/tr-uid/records/tr-uid",
					CompletionTime: &completionTime,
				},
				RecordName: "test-ns/results/tr-uid/records/tr-uid",
			}, nil
		},
		fetchLogsFunc: func(ctx context.Context, recordName string) (string, error) {
			if !getTaskRunCalled {
				t.Fatal("FetchLogs called before GetTaskRun")
			}
			if recordName != "test-ns/results/tr-uid/records/tr-uid" {
				t.Errorf("Expected record name 'test-ns/results/tr-uid/records/tr-uid', got: '%s'", recordName)
			}
			return "task execution logs", nil
		},
	}

	deps := Dependencies{Service: mock, DefaultNamespace: "test-ns"}
	tool := newTaskRunLogsTool(deps)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"name": "my-task",
	}

	result, err := tool.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("Handler failed: %v", err)
	}

	if result.IsError {
		t.Fatalf("Result is error: %s", getTextFromResult(result))
	}

	text := getTextFromResult(result)
	if !strings.Contains(text, "task execution logs") {
		t.Errorf("Response doesn't contain expected logs: %s", text)
	}
}

func TestTaskRunLogs_ByUID(t *testing.T) {
	// Note: The validation logic in taskrun_logs doesn't check for UID,
	// so we need to provide at least one of name/prefix/labelSelector as well.
	// This is a known limitation in the current implementation.
	completionTime := metav1.NewTime(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	mock := &mockTaskRunService{
		getTaskRunFunc: func(ctx context.Context, selector tektonresults.RunSelector) (*tektonresults.RunDetail, error) {
			if selector.UID != "task-uid-abc" {
				t.Errorf("Expected UID 'task-uid-abc', got %s", selector.UID)
			}
			// Name can be anything since UID takes precedence
			return &tektonresults.RunDetail{
				Summary: tektonresults.RunSummary{
					RecordName:     "test-ns/results/task-uid-abc/records/task-uid-abc",
					CompletionTime: &completionTime,
				},
				RecordName: "test-ns/results/task-uid-abc/records/task-uid-abc",
			}, nil
		},
		fetchLogsFunc: func(ctx context.Context, recordName string) (string, error) {
			return "logs content", nil
		},
	}

	deps := Dependencies{Service: mock, DefaultNamespace: "test-ns"}
	tool := newTaskRunLogsTool(deps)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"uid":  "task-uid-abc",
		"name": "placeholder", // Required due to validation logic
	}

	result, err := tool.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("Handler failed: %v", err)
	}

	if result.IsError {
		t.Fatalf("Result is error: %s", getTextFromResult(result))
	}
}

func TestTaskRunLogs_EmptyLogs(t *testing.T) {
	completionTime := metav1.NewTime(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	mock := &mockTaskRunService{
		getTaskRunFunc: func(ctx context.Context, selector tektonresults.RunSelector) (*tektonresults.RunDetail, error) {
			return &tektonresults.RunDetail{
				Summary: tektonresults.RunSummary{
					RecordName:     "test-ns/results/tr-uid/records/tr-uid",
					CompletionTime: &completionTime,
				},
				RecordName: "test-ns/results/tr-uid/records/tr-uid",
			}, nil
		},
		fetchLogsFunc: func(ctx context.Context, recordName string) (string, error) {
			return "", nil
		},
	}

	deps := Dependencies{Service: mock, DefaultNamespace: "default"}
	tool := newTaskRunLogsTool(deps)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"name": "test"}

	result, err := tool.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("Handler failed: %v", err)
	}

	// When logs are empty, FetchLogs returns empty string, which is returned as-is
	text := getTextFromResult(result)
	if text != "" {
		t.Errorf("Expected empty logs, got: %s", text)
	}
}

func TestTaskRunLogs_ServiceError(t *testing.T) {
	mock := &mockTaskRunService{
		getTaskRunFunc: func(ctx context.Context, selector tektonresults.RunSelector) (*tektonresults.RunDetail, error) {
			return nil, &testError{msg: "taskrun not found"}
		},
	}

	deps := Dependencies{Service: mock, DefaultNamespace: "default"}
	tool := newTaskRunLogsTool(deps)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"name": "missing"}

	result, err := tool.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("Handler failed: %v", err)
	}

	if !result.IsError {
		t.Fatal("Expected error result")
	}
}

func TestTaskRunLogs_FetchLogsError(t *testing.T) {
	completionTime := metav1.NewTime(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	mock := &mockTaskRunService{
		getTaskRunFunc: func(ctx context.Context, selector tektonresults.RunSelector) (*tektonresults.RunDetail, error) {
			return &tektonresults.RunDetail{
				Summary: tektonresults.RunSummary{
					RecordName:     "test-ns/results/tr-uid/records/tr-uid",
					CompletionTime: &completionTime,
				},
				RecordName: "test-ns/results/tr-uid/records/tr-uid",
			}, nil
		},
		fetchLogsFunc: func(ctx context.Context, recordName string) (string, error) {
			return "", &testError{msg: "logs not available"}
		},
	}

	deps := Dependencies{Service: mock, DefaultNamespace: "default"}
	tool := newTaskRunLogsTool(deps)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"name": "test"}

	result, err := tool.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("Handler failed: %v", err)
	}

	if !result.IsError {
		t.Fatal("Expected error result")
	}

	if !strings.Contains(getTextFromResult(result), "logs not available") {
		t.Errorf("Error message doesn't contain expected text: %s", getTextFromResult(result))
	}
}
