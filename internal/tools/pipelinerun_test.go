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

// mockService is a mock implementation of Service interface for testing
type mockPipelineRunService struct {
	listPipelineRunsFunc func(ctx context.Context, opts tektonresults.ListOptions) ([]tektonresults.RunSummary, error)
	listTaskRunsFunc     func(ctx context.Context, opts tektonresults.ListOptions) ([]tektonresults.RunSummary, error)
	getPipelineRunFunc   func(ctx context.Context, selector tektonresults.RunSelector) (*tektonresults.RunDetail, error)
	getTaskRunFunc       func(ctx context.Context, selector tektonresults.RunSelector) (*tektonresults.RunDetail, error)
	fetchLogsFunc        func(ctx context.Context, recordName string) (string, error)
}

func (m *mockPipelineRunService) ListPipelineRuns(ctx context.Context, opts tektonresults.ListOptions) ([]tektonresults.RunSummary, error) {
	if m.listPipelineRunsFunc != nil {
		return m.listPipelineRunsFunc(ctx, opts)
	}
	return nil, nil
}

func (m *mockPipelineRunService) ListTaskRuns(ctx context.Context, opts tektonresults.ListOptions) ([]tektonresults.RunSummary, error) {
	if m.listTaskRunsFunc != nil {
		return m.listTaskRunsFunc(ctx, opts)
	}
	return nil, nil
}

func (m *mockPipelineRunService) GetPipelineRun(ctx context.Context, selector tektonresults.RunSelector) (*tektonresults.RunDetail, error) {
	if m.getPipelineRunFunc != nil {
		return m.getPipelineRunFunc(ctx, selector)
	}
	return nil, nil
}

func (m *mockPipelineRunService) GetTaskRun(ctx context.Context, selector tektonresults.RunSelector) (*tektonresults.RunDetail, error) {
	if m.getTaskRunFunc != nil {
		return m.getTaskRunFunc(ctx, selector)
	}
	return nil, nil
}

func (m *mockPipelineRunService) FetchLogs(ctx context.Context, recordName string) (string, error) {
	if m.fetchLogsFunc != nil {
		return m.fetchLogsFunc(ctx, recordName)
	}
	return "", nil
}

// getTextFromResult extracts text from CallToolResult for testing
func getTextFromResult(result *mcp.CallToolResult) string {
	if len(result.Content) == 0 {
		return ""
	}
	if textContent, ok := mcp.AsTextContent(result.Content[0]); ok {
		return textContent.Text
	}
	return ""
}

func TestPipelineRunList_DefaultParameters(t *testing.T) {
	mock := &mockPipelineRunService{
		listPipelineRunsFunc: func(ctx context.Context, opts tektonresults.ListOptions) ([]tektonresults.RunSummary, error) {
			if opts.Namespace != "test-ns" {
				t.Errorf("Expected namespace 'test-ns', got %s", opts.Namespace)
			}
			if opts.Limit != 50 {
				t.Errorf("Expected limit 50, got %d", opts.Limit)
			}
			return []tektonresults.RunSummary{
				{Name: "pr-1", Namespace: "test-ns"},
			}, nil
		},
	}

	deps := Dependencies{Service: mock, DefaultNamespace: "test-ns"}
	tool := newPipelineRunListTool(deps)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{} // No args, use defaults

	result, err := tool.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("Handler failed: %v", err)
	}

	if result.IsError {
		t.Fatalf("Result is error: %s", getTextFromResult(result))
	}

	// Verify JSON response contains the run
	text := getTextFromResult(result)
	if !strings.Contains(text, "pr-1") {
		t.Errorf("Response doesn't contain expected run: %s", text)
	}
}

func TestPipelineRunList_CustomParameters(t *testing.T) {
	mock := &mockPipelineRunService{
		listPipelineRunsFunc: func(ctx context.Context, opts tektonresults.ListOptions) ([]tektonresults.RunSummary, error) {
			if opts.Namespace != "-" {
				t.Errorf("Expected namespace '-', got %s", opts.Namespace)
			}
			if opts.LabelSelector != "app=test" {
				t.Errorf("Expected labelSelector 'app=test', got %s", opts.LabelSelector)
			}
			if opts.Prefix != "my-pr" {
				t.Errorf("Expected prefix 'my-pr', got %s", opts.Prefix)
			}
			if opts.Limit != 10 {
				t.Errorf("Expected limit 10, got %d", opts.Limit)
			}
			return []tektonresults.RunSummary{}, nil
		},
	}

	deps := Dependencies{Service: mock, DefaultNamespace: "default"}
	tool := newPipelineRunListTool(deps)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"namespace":     "all",
		"labelSelector": "app=test",
		"prefix":        "my-pr",
		"limit":         float64(10), // JSON numbers are float64
	}

	_, err := tool.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("Handler failed: %v", err)
	}
}

func TestPipelineRunList_LimitSanitization(t *testing.T) {
	tests := []struct {
		name          string
		inputLimit    float64
		expectedLimit int
	}{
		{"zero limit", 0, 50},
		{"negative limit", -10, 50},
		{"above max", 300, 200},
		{"valid limit", 25, 25},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockPipelineRunService{
				listPipelineRunsFunc: func(ctx context.Context, opts tektonresults.ListOptions) ([]tektonresults.RunSummary, error) {
					if opts.Limit != tt.expectedLimit {
						t.Errorf("Expected limit %d, got %d", tt.expectedLimit, opts.Limit)
					}
					return []tektonresults.RunSummary{}, nil
				},
			}

			deps := Dependencies{Service: mock, DefaultNamespace: "default"}
			tool := newPipelineRunListTool(deps)

			req := mcp.CallToolRequest{}
			req.Params.Arguments = map[string]any{"limit": tt.inputLimit}

			_, err := tool.Handler(context.Background(), req)
			if err != nil {
				t.Fatalf("Handler failed: %v", err)
			}
		})
	}
}

func TestPipelineRunList_ServiceError(t *testing.T) {
	mock := &mockPipelineRunService{
		listPipelineRunsFunc: func(ctx context.Context, opts tektonresults.ListOptions) ([]tektonresults.RunSummary, error) {
			return nil, &testError{msg: "service unavailable"}
		},
	}

	deps := Dependencies{Service: mock, DefaultNamespace: "default"}
	tool := newPipelineRunListTool(deps)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{}

	result, err := tool.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("Handler failed: %v", err)
	}

	if !result.IsError {
		t.Fatal("Expected error result")
	}

	if !strings.Contains(getTextFromResult(result), "service unavailable") {
		t.Errorf("Error message doesn't contain expected text: %s", getTextFromResult(result))
	}
}

func TestPipelineRunGet_ByName(t *testing.T) {
	mock := &mockPipelineRunService{
		getPipelineRunFunc: func(ctx context.Context, selector tektonresults.RunSelector) (*tektonresults.RunDetail, error) {
			if selector.Name != "my-pipeline" {
				t.Errorf("Expected name 'my-pipeline', got %s", selector.Name)
			}
			if selector.Namespace != "test-ns" {
				t.Errorf("Expected namespace 'test-ns', got %s", selector.Namespace)
			}
			if !selector.SelectLast {
				t.Error("Expected SelectLast to be true by default")
			}
			return &tektonresults.RunDetail{
				Raw: json.RawMessage(`{"metadata":{"name":"my-pipeline"}}`),
			}, nil
		},
	}

	deps := Dependencies{Service: mock, DefaultNamespace: "test-ns"}
	tool := newPipelineRunGetTool(deps)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"name": "my-pipeline",
	}

	result, err := tool.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("Handler failed: %v", err)
	}

	if result.IsError {
		t.Fatalf("Result is error: %s", getTextFromResult(result))
	}

	// Default output is YAML
	text := getTextFromResult(result)
	if !strings.Contains(text, "my-pipeline") {
		t.Errorf("Response doesn't contain expected name: %s", text)
	}
}

func TestPipelineRunGet_ByUID(t *testing.T) {
	mock := &mockPipelineRunService{
		getPipelineRunFunc: func(ctx context.Context, selector tektonresults.RunSelector) (*tektonresults.RunDetail, error) {
			if selector.UID != "test-uid-123" {
				t.Errorf("Expected UID 'test-uid-123', got %s", selector.UID)
			}
			if selector.Name != "" {
				t.Error("Expected Name to be empty when querying by UID")
			}
			return &tektonresults.RunDetail{
				Raw: json.RawMessage(`{"metadata":{"uid":"test-uid-123"}}`),
			}, nil
		},
	}

	deps := Dependencies{Service: mock, DefaultNamespace: "default"}
	tool := newPipelineRunGetTool(deps)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"uid": "test-uid-123",
	}

	result, err := tool.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("Handler failed: %v", err)
	}

	if result.IsError {
		t.Fatalf("Result is error: %s", getTextFromResult(result))
	}
}

func TestPipelineRunGet_OutputFormats(t *testing.T) {
	tests := []struct {
		name           string
		outputFormat   string
		expectedInText string
	}{
		{"yaml format", "yaml", "metadata:"},  // YAML uses colons
		{"json format", "json", `"metadata"`}, // JSON uses quotes
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockPipelineRunService{
				getPipelineRunFunc: func(ctx context.Context, selector tektonresults.RunSelector) (*tektonresults.RunDetail, error) {
					return &tektonresults.RunDetail{
						Raw: json.RawMessage(`{"metadata":{"name":"test"}}`),
					}, nil
				},
			}

			deps := Dependencies{Service: mock, DefaultNamespace: "default"}
			tool := newPipelineRunGetTool(deps)

			req := mcp.CallToolRequest{}
			req.Params.Arguments = map[string]any{
				"name":   "test",
				"output": tt.outputFormat,
			}

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

func TestPipelineRunGet_SelectLastParameter(t *testing.T) {
	mock := &mockPipelineRunService{
		getPipelineRunFunc: func(ctx context.Context, selector tektonresults.RunSelector) (*tektonresults.RunDetail, error) {
			if selector.SelectLast {
				t.Error("Expected SelectLast to be false")
			}
			return &tektonresults.RunDetail{
				Raw: json.RawMessage(`{"metadata":{"name":"test"}}`),
			}, nil
		},
	}

	deps := Dependencies{Service: mock, DefaultNamespace: "default"}
	tool := newPipelineRunGetTool(deps)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"name":       "test",
		"selectLast": false,
	}

	_, err := tool.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("Handler failed: %v", err)
	}
}

func TestPipelineRunGet_ServiceError(t *testing.T) {
	mock := &mockPipelineRunService{
		getPipelineRunFunc: func(ctx context.Context, selector tektonresults.RunSelector) (*tektonresults.RunDetail, error) {
			return nil, &testError{msg: "not found"}
		},
	}

	deps := Dependencies{Service: mock, DefaultNamespace: "default"}
	tool := newPipelineRunGetTool(deps)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"name": "missing"}

	result, err := tool.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("Handler failed: %v", err)
	}

	if !result.IsError {
		t.Fatal("Expected error result")
	}

	if !strings.Contains(getTextFromResult(result), "not found") {
		t.Errorf("Error message doesn't contain expected text: %s", getTextFromResult(result))
	}
}

func TestPipelineRunLogs_ByName(t *testing.T) {
	completionTime := metav1.NewTime(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	mock := &mockPipelineRunService{
		getPipelineRunFunc: func(ctx context.Context, selector tektonresults.RunSelector) (*tektonresults.RunDetail, error) {
			if selector.Name != "my-pipeline" {
				t.Errorf("Expected name 'my-pipeline', got %s", selector.Name)
			}
			return &tektonresults.RunDetail{
				Summary: tektonresults.RunSummary{
					UID:            "pr-uid",
					Namespace:      "test-ns",
					RecordName:     "test-ns/results/pr-uid/records/pr-uid",
					CompletionTime: &completionTime,
				},
			}, nil
		},
		listTaskRunsFunc: func(ctx context.Context, opts tektonresults.ListOptions) ([]tektonresults.RunSummary, error) {
			if opts.LabelSelector != "tekton.dev/pipelineRunUID=pr-uid" {
				t.Errorf("Expected label selector for PipelineRun UID, got %s", opts.LabelSelector)
			}
			return []tektonresults.RunSummary{
				{RecordName: "test-ns/results/pr-uid/records/tr-1"},
			}, nil
		},
		fetchLogsFunc: func(ctx context.Context, recordName string) (string, error) {
			return "task logs output", nil
		},
	}

	deps := Dependencies{Service: mock, DefaultNamespace: "test-ns"}
	tool := newPipelineRunLogsTool(deps)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"name": "my-pipeline",
	}

	result, err := tool.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("Handler failed: %v", err)
	}

	if result.IsError {
		t.Fatalf("Result is error: %s", getTextFromResult(result))
	}

	text := getTextFromResult(result)
	if !strings.Contains(text, "task logs output") {
		t.Errorf("Response doesn't contain expected logs: %s", text)
	}
}

func TestPipelineRunLogs_ByUID(t *testing.T) {
	mock := &mockPipelineRunService{
		getPipelineRunFunc: func(ctx context.Context, selector tektonresults.RunSelector) (*tektonresults.RunDetail, error) {
			if selector.UID != "pr-uid-456" {
				t.Errorf("Expected UID 'pr-uid-456', got %s", selector.UID)
			}
			return &tektonresults.RunDetail{
				Summary: tektonresults.RunSummary{
					UID:        "pr-uid-456",
					Namespace:  "test-ns",
					RecordName: "test-ns/results/pr-uid-456/records/pr-uid-456",
				},
			}, nil
		},
		listTaskRunsFunc: func(ctx context.Context, opts tektonresults.ListOptions) ([]tektonresults.RunSummary, error) {
			return []tektonresults.RunSummary{}, nil
		},
	}

	deps := Dependencies{Service: mock, DefaultNamespace: "test-ns"}
	tool := newPipelineRunLogsTool(deps)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"uid": "pr-uid-456",
	}

	_, err := tool.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("Handler failed: %v", err)
	}
}

func TestPipelineRunLogs_NoTaskRuns(t *testing.T) {
	completionTime := metav1.NewTime(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	mock := &mockPipelineRunService{
		getPipelineRunFunc: func(ctx context.Context, selector tektonresults.RunSelector) (*tektonresults.RunDetail, error) {
			return &tektonresults.RunDetail{
				Summary: tektonresults.RunSummary{
					UID:            "pr-uid",
					RecordName:     "test-ns/results/pr-uid/records/pr-uid",
					CompletionTime: &completionTime,
				},
			}, nil
		},
		listTaskRunsFunc: func(ctx context.Context, opts tektonresults.ListOptions) ([]tektonresults.RunSummary, error) {
			return []tektonresults.RunSummary{}, nil
		},
	}

	deps := Dependencies{Service: mock, DefaultNamespace: "default"}
	tool := newPipelineRunLogsTool(deps)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"name": "test"}

	result, err := tool.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("Handler failed: %v", err)
	}

	text := getTextFromResult(result)
	if !strings.Contains(text, "No TaskRuns found") {
		t.Errorf("Expected 'No TaskRuns found' message, got: %s", text)
	}
}

func TestPipelineRunLogs_ServiceError(t *testing.T) {
	mock := &mockPipelineRunService{
		getPipelineRunFunc: func(ctx context.Context, selector tektonresults.RunSelector) (*tektonresults.RunDetail, error) {
			return nil, &testError{msg: "pipeline not found"}
		},
	}

	deps := Dependencies{Service: mock, DefaultNamespace: "default"}
	tool := newPipelineRunLogsTool(deps)

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

// testError is a simple error type for testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
