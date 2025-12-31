package tektonresults

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// mockRestClient is a test double for restClient
type mockRestClient struct {
	getRecordFunc   func(ctx context.Context, recordName string) (*record, error)
	listResultsFunc func(ctx context.Context, req listResultsRequest) (*listResultsResponse, error)
	listRecordsFunc func(ctx context.Context, req listRecordsRequest) (*listRecordsResponse, error)
	getLogFunc      func(ctx context.Context, logPath string) ([]byte, error)
}

func (m *mockRestClient) getRecord(ctx context.Context, recordName string) (*record, error) {
	if m.getRecordFunc != nil {
		return m.getRecordFunc(ctx, recordName)
	}
	return nil, fmt.Errorf("getRecord not mocked")
}

func (m *mockRestClient) listResults(ctx context.Context, req listResultsRequest) (*listResultsResponse, error) {
	if m.listResultsFunc != nil {
		return m.listResultsFunc(ctx, req)
	}
	return nil, fmt.Errorf("listResults not mocked")
}

func (m *mockRestClient) listRecords(ctx context.Context, req listRecordsRequest) (*listRecordsResponse, error) {
	if m.listRecordsFunc != nil {
		return m.listRecordsFunc(ctx, req)
	}
	return nil, fmt.Errorf("listRecords not mocked")
}

func (m *mockRestClient) getLog(ctx context.Context, logPath string) ([]byte, error) {
	if m.getLogFunc != nil {
		return m.getLogFunc(ctx, logPath)
	}
	return nil, fmt.Errorf("getLog not mocked")
}

func TestService_GetRun_PipelineRun_DirectGetSuccess(t *testing.T) {
	prUID := "pr-test-uid-123"
	prName := "test-pipelinerun"
	namespace := "foo"

	mockClient := &mockRestClient{
		getRecordFunc: func(ctx context.Context, recordName string) (*record, error) {
			expectedName := fmt.Sprintf("%s/results/%s/records/%s", namespace, prUID, prUID)
			if recordName != expectedName {
				t.Errorf("Expected record name %s, got %s", expectedName, recordName)
			}

			rec := &record{
				Name: recordName,
				Uid:  prUID,
			}
			rec.Data.Value = json.RawMessage(fmt.Sprintf(`{
				"metadata": {
					"name": "%s",
					"namespace": "%s",
					"uid": "%s"
				},
				"spec": {},
				"status": {"conditions": [{"type": "Succeeded", "status": "True"}]}
			}`, prName, namespace, prUID))
			return rec, nil
		},
	}

	service := &Service{client: mockClient}

	detail, err := service.getRun(context.Background(), resourceKindPipelineRun, RunSelector{
		Namespace: namespace,
		UID:       prUID,
	})

	if err != nil {
		t.Fatalf("getRun() failed: %v", err)
	}

	if detail.Summary.UID != prUID {
		t.Errorf("Expected UID %s, got %s", prUID, detail.Summary.UID)
	}
	if detail.Summary.Name != prName {
		t.Errorf("Expected name %s, got %s", prName, detail.Summary.Name)
	}
	if detail.Summary.Namespace != namespace {
		t.Errorf("Expected namespace %s, got %s", namespace, detail.Summary.Namespace)
	}
}

func TestService_GetRun_PipelineRun_NotFound(t *testing.T) {
	prUID := "missing-uid"
	namespace := "foo"

	mockClient := &mockRestClient{
		getRecordFunc: func(ctx context.Context, recordName string) (*record, error) {
			return nil, fmt.Errorf(`results API GET /apis/results.tekton.dev/v1alpha2/parents/foo/results/%s/records/%s: {"code":5,"message":"record not found"}`, prUID, prUID)
		},
	}

	service := &Service{client: mockClient}

	_, err := service.getRun(context.Background(), resourceKindPipelineRun, RunSelector{
		Namespace: namespace,
		UID:       prUID,
	})

	if err == nil {
		t.Fatal("Expected error for missing PipelineRun, got nil")
	}

	if !strings.Contains(err.Error(), "get record by UID") {
		t.Errorf("Expected error about get record by UID, got: %v", err)
	}
}

func TestService_GetRun_PipelineRun_DefaultNamespace(t *testing.T) {
	prUID := "test-uid"
	mockClient := &mockRestClient{
		getRecordFunc: func(ctx context.Context, recordName string) (*record, error) {
			// Should use "default" namespace when empty
			if !strings.HasPrefix(recordName, "default/results/") {
				t.Errorf("Expected default namespace, got record name: %s", recordName)
			}
			rec := &record{
				Name: recordName,
				Uid:  prUID,
			}
			rec.Data.Value = json.RawMessage(`{"metadata":{"name":"test","namespace":"default","uid":"` + prUID + `"},"spec":{},"status":{}}`)
			return rec, nil
		},
	}

	service := &Service{client: mockClient}

	_, err := service.getRun(context.Background(), resourceKindPipelineRun, RunSelector{
		Namespace: "", // Empty namespace
		UID:       prUID,
	})

	if err != nil {
		t.Fatalf("getRun() with empty namespace failed: %v", err)
	}
}

func TestService_GetRun_StandaloneTaskRun_DirectGetSuccess(t *testing.T) {
	trUID := "tr-standalone-uid"
	trName := "test-taskrun"
	namespace := "foo"

	mockClient := &mockRestClient{
		getRecordFunc: func(ctx context.Context, recordName string) (*record, error) {
			expectedName := fmt.Sprintf("%s/results/%s/records/%s", namespace, trUID, trUID)
			if recordName != expectedName {
				t.Errorf("Expected record name %s, got %s", expectedName, recordName)
			}

			rec := &record{
				Name: recordName,
				Uid:  trUID,
			}
			rec.Data.Value = json.RawMessage(fmt.Sprintf(`{
				"apiVersion": "tekton.dev/v1",
				"kind": "TaskRun",
				"metadata": {
					"name": "%s",
					"namespace": "%s",
					"uid": "%s"
				},
				"spec": {},
				"status": {"conditions": [{"type": "Succeeded", "status": "True"}]}
			}`, trName, namespace, trUID))
			return rec, nil
		},
	}

	service := &Service{client: mockClient}

	detail, err := service.getRun(context.Background(), resourceKindTaskRun, RunSelector{
		Namespace: namespace,
		UID:       trUID,
	})

	if err != nil {
		t.Fatalf("getRun() for standalone TaskRun failed: %v", err)
	}

	if detail.Summary.UID != trUID {
		t.Errorf("Expected UID %s, got %s", trUID, detail.Summary.UID)
	}
}

func TestService_GetRun_TaskRunInPipeline_FallbackSuccess(t *testing.T) {
	trUID := "tr-in-pipeline-uid"
	prUID := "parent-pr-uid"
	trName := "test-pipelinerun-task1"
	namespace := "foo"

	mockClient := &mockRestClient{
		getRecordFunc: func(ctx context.Context, recordName string) (*record, error) {
			// Direct GetRecord fails (TaskRun is stored under PipelineRun's Result)
			return nil, fmt.Errorf(`results API GET /apis/results.tekton.dev/v1alpha2/parents/foo/results/%s/records/%s: {"code":5,"message":"record not found"}`, trUID, trUID)
		},
		listRecordsFunc: func(ctx context.Context, req listRecordsRequest) (*listRecordsResponse, error) {
			// Fallback: search across all records in namespace
			if !strings.HasPrefix(req.Parent, namespace) {
				t.Errorf("Expected parent to start with namespace %s, got %s", namespace, req.Parent)
			}

			// Return TaskRun stored under PipelineRun's Result
			rec := record{
				Name: fmt.Sprintf("%s/results/%s/records/%s", namespace, prUID, trUID),
				Uid:  trUID,
			}
			rec.Data.Value = json.RawMessage(fmt.Sprintf(`{
				"apiVersion": "tekton.dev/v1",
				"kind": "TaskRun",
				"metadata": {
					"name": "%s",
					"namespace": "%s",
					"uid": "%s",
					"labels": {
						"tekton.dev/pipelineRun": "test-pipelinerun",
						"tekton.dev/pipelineRunUID": "%s"
					}
				},
				"spec": {},
				"status": {"conditions": [{"type": "Succeeded", "status": "True"}]}
			}`, trName, namespace, trUID, prUID))
			return &listRecordsResponse{
				Records: []record{rec},
			}, nil
		},
	}

	service := &Service{client: mockClient}

	detail, err := service.getRun(context.Background(), resourceKindTaskRun, RunSelector{
		Namespace: namespace,
		UID:       trUID,
	})

	if err != nil {
		t.Fatalf("getRun() fallback for TaskRun in Pipeline failed: %v", err)
	}

	if detail.Summary.UID != trUID {
		t.Errorf("Expected UID %s, got %s", trUID, detail.Summary.UID)
	}

	// Verify TaskRun is stored under PipelineRun's Result
	expectedRecordPrefix := fmt.Sprintf("%s/results/%s/records/", namespace, prUID)
	if !strings.HasPrefix(detail.RecordName, expectedRecordPrefix) {
		t.Errorf("Expected record name to start with %s, got %s", expectedRecordPrefix, detail.RecordName)
	}
}

func TestService_GetRun_TaskRunInPipeline_FallbackNotFound(t *testing.T) {
	trUID := "missing-tr-uid"
	namespace := "foo"

	mockClient := &mockRestClient{
		getRecordFunc: func(ctx context.Context, recordName string) (*record, error) {
			// Direct GetRecord fails
			return nil, fmt.Errorf(`{"code":5,"message":"record not found"}`)
		},
		listRecordsFunc: func(ctx context.Context, req listRecordsRequest) (*listRecordsResponse, error) {
			// Fallback: no records found
			return &listRecordsResponse{
				Records: []record{},
			}, nil
		},
	}

	service := &Service{client: mockClient}

	_, err := service.getRun(context.Background(), resourceKindTaskRun, RunSelector{
		Namespace: namespace,
		UID:       trUID,
	})

	if err == nil {
		t.Fatal("Expected error for missing TaskRun, got nil")
	}

	if !strings.Contains(err.Error(), "no run found") {
		t.Errorf("Expected 'no run found' error, got: %v", err)
	}
}

func TestService_GetRun_ByName_WithSelectLast_MultipleMatches(t *testing.T) {
	runName := "my-taskrun"
	namespace := "foo"

	mockClient := &mockRestClient{
		listRecordsFunc: func(ctx context.Context, req listRecordsRequest) (*listRecordsResponse, error) {
			// Return multiple records with same NAME but different UIDs (historical runs)
			rec1 := record{
				Name: fmt.Sprintf("%s/results/uid-newer/records/uid-newer", namespace),
				Uid:  "uid-newer",
			}
			rec1.Data.Value = json.RawMessage(fmt.Sprintf(`{
				"apiVersion": "tekton.dev/v1",
				"kind": "TaskRun",
				"metadata": {"name":"%s","namespace":"%s","uid":"uid-newer"},
				"spec": {}, "status": {"startTime":"2025-12-29T12:00:00Z"}
			}`, runName, namespace))

			rec2 := record{
				Name: fmt.Sprintf("%s/results/uid-older/records/uid-older", namespace),
				Uid:  "uid-older",
			}
			rec2.Data.Value = json.RawMessage(fmt.Sprintf(`{
				"apiVersion": "tekton.dev/v1",
				"kind": "TaskRun",
				"metadata": {"name":"%s","namespace":"%s","uid":"uid-older"},
				"spec": {}, "status": {"startTime":"2025-12-29T11:00:00Z"}
			}`, runName, namespace))

			return &listRecordsResponse{
				Records: []record{rec1, rec2},
			}, nil
		},
	}

	service := &Service{client: mockClient}

	// With SelectLast=true, should return first match (most recent due to create_time desc ordering)
	detail, err := service.getRun(context.Background(), resourceKindTaskRun, RunSelector{
		Namespace:  namespace,
		Name:       runName,
		SelectLast: true,
	})

	if err != nil {
		t.Fatalf("getRun() with SelectLast=true failed: %v", err)
	}

	if detail.Summary.UID != "uid-newer" {
		t.Errorf("Expected to get UID 'uid-newer' (first match), got %s", detail.Summary.UID)
	}
	if detail.Summary.Name != runName {
		t.Errorf("Expected name %s, got %s", runName, detail.Summary.Name)
	}
}

func TestService_GetRun_ByName_WithoutSelectLast_MultipleMatches(t *testing.T) {
	runName := "my-taskrun"
	namespace := "foo"

	mockClient := &mockRestClient{
		listRecordsFunc: func(ctx context.Context, req listRecordsRequest) (*listRecordsResponse, error) {
			// Return multiple records with same NAME but different UIDs
			rec1 := record{
				Name: fmt.Sprintf("%s/results/uid-1/records/uid-1", namespace),
				Uid:  "uid-1",
			}
			rec1.Data.Value = json.RawMessage(fmt.Sprintf(`{"apiVersion":"tekton.dev/v1","kind":"TaskRun","metadata":{"name":"%s","namespace":"%s","uid":"uid-1"},"spec":{},"status":{}}`, runName, namespace))

			rec2 := record{
				Name: fmt.Sprintf("%s/results/uid-2/records/uid-2", namespace),
				Uid:  "uid-2",
			}
			rec2.Data.Value = json.RawMessage(fmt.Sprintf(`{"apiVersion":"tekton.dev/v1","kind":"TaskRun","metadata":{"name":"%s","namespace":"%s","uid":"uid-2"},"spec":{},"status":{}}`, runName, namespace))

			return &listRecordsResponse{
				Records: []record{rec1, rec2},
			}, nil
		},
	}

	service := &Service{client: mockClient}

	// With SelectLast=false, should return error for multiple matches
	_, err := service.getRun(context.Background(), resourceKindTaskRun, RunSelector{
		Namespace:  namespace,
		Name:       runName,
		SelectLast: false,
	})

	if err == nil {
		t.Fatal("Expected error for multiple matches with SelectLast=false, got nil")
	}

	if !strings.Contains(err.Error(), "multiple run instances match") {
		t.Errorf("Expected 'multiple run instances match' error, got: %v", err)
	}
}

func TestService_QueryRecords_UIDFilter(t *testing.T) {
	targetUID := "target-uid"
	otherUID := "other-uid"
	namespace := "foo"

	mockClient := &mockRestClient{
		listRecordsFunc: func(ctx context.Context, req listRecordsRequest) (*listRecordsResponse, error) {
			// Simulate API returning multiple records, only one matches UID
			rec1 := record{
				Name: fmt.Sprintf("%s/results/r1/records/%s", namespace, targetUID),
				Uid:  targetUID,
			}
			rec1.Data.Value = json.RawMessage(fmt.Sprintf(`{"apiVersion":"tekton.dev/v1","kind":"TaskRun","metadata":{"name":"target-tr","namespace":"%s","uid":"%s"},"spec":{},"status":{}}`, namespace, targetUID))

			rec2 := record{
				Name: fmt.Sprintf("%s/results/r2/records/%s", namespace, otherUID),
				Uid:  otherUID,
			}
			rec2.Data.Value = json.RawMessage(fmt.Sprintf(`{"apiVersion":"tekton.dev/v1","kind":"TaskRun","metadata":{"name":"other-tr","namespace":"%s","uid":"%s"},"spec":{},"status":{}}`, namespace, otherUID))

			return &listRecordsResponse{
				Records: []record{rec1, rec2},
			}, nil
		},
	}

	service := &Service{client: mockClient}

	req := listRecordsRequest{
		Parent:  fmt.Sprintf("%s/results/-", namespace),
		Filter:  `data_type=="tekton.dev/v1.TaskRun"`,
		OrderBy: "create_time desc",
	}

	detail, err := service.queryRecords(context.Background(), req, RunSelector{
		Namespace: namespace,
		UID:       targetUID,
	})

	if err != nil {
		t.Fatalf("queryRecords() failed: %v", err)
	}

	// Should only return the record matching targetUID
	if detail.Summary.UID != targetUID {
		t.Errorf("Expected UID %s, got %s", targetUID, detail.Summary.UID)
	}
	if detail.Summary.Name != "target-tr" {
		t.Errorf("Expected name 'target-tr', got %s", detail.Summary.Name)
	}
}

func TestService_QueryRecords_EmptyResults(t *testing.T) {
	mockClient := &mockRestClient{
		listRecordsFunc: func(ctx context.Context, req listRecordsRequest) (*listRecordsResponse, error) {
			return &listRecordsResponse{
				Records: []record{},
			}, nil
		},
	}

	service := &Service{client: mockClient}

	req := listRecordsRequest{
		Parent: "foo/results/-",
		Filter: `data_type=="tekton.dev/v1.TaskRun"`,
	}

	_, err := service.queryRecords(context.Background(), req, RunSelector{
		Namespace: "foo",
		UID:       "missing-uid",
	})

	if err == nil {
		t.Fatal("Expected error for empty results, got nil")
	}

	if !strings.Contains(err.Error(), "no run found") {
		t.Errorf("Expected 'no run found' error, got: %v", err)
	}
}
