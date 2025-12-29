//go:build integration
// +build integration

package tektonresults

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// TestIntegration_PipelineRunUIDLookup tests the complete flow of looking up a PipelineRun by UID
func TestIntegration_PipelineRunUIDLookup(t *testing.T) {
	prUID := "pr-integration-test-uid"
	prName := "test-pipelinerun"
	namespace := "integration-test"

	// Create mock Tekton Results API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request has parents/ prefix
		if !strings.Contains(r.URL.Path, "/parents/") {
			t.Errorf("Expected parents/ prefix in path, got: %s", r.URL.Path)
			http.Error(w, "Missing parents/ prefix", http.StatusNotFound)
			return
		}

		// Handle GetRecord request
		expectedPath := fmt.Sprintf("/apis/results.tekton.dev/v1alpha2/parents/%s/results/%s/records/%s",
			namespace, prUID, prUID)
		if r.URL.Path == expectedPath && r.Method == http.MethodGet {
			response := record{
				Name: fmt.Sprintf("%s/results/%s/records/%s", namespace, prUID, prUID),
				Uid:  prUID,
			}
			response.Data.Value = json.RawMessage(fmt.Sprintf(`{
				"apiVersion": "tekton.dev/v1",
				"kind": "PipelineRun",
				"metadata": {
					"name": "%s",
					"namespace": "%s",
					"uid": "%s",
					"creationTimestamp": "2025-12-28T10:00:00Z"
				},
				"spec": {
					"pipelineSpec": {
						"tasks": [{"name": "task1"}]
					}
				},
				"status": {
					"conditions": [{
						"type": "Succeeded",
						"status": "True",
						"reason": "Succeeded"
					}],
					"startTime": "2025-12-28T10:00:00Z",
					"completionTime": "2025-12-28T10:05:00Z"
				}
			}`, prName, namespace, prUID))
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}

		http.NotFound(w, r)
	}))
	defer server.Close()

	// Create service with mock server
	baseURL := server.URL + "/apis/results.tekton.dev/v1alpha2"
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		t.Fatalf("Failed to parse URL: %v", err)
	}

	client := &restClient{
		baseURL:    parsedURL,
		httpClient: server.Client(),
	}
	service := &Service{client: client}

	// Execute: Lookup PipelineRun by UID
	detail, err := service.getRun(context.Background(), resourceKindPipelineRun, RunSelector{
		Namespace: namespace,
		UID:       prUID,
	})

	// Verify: Check results
	if err != nil {
		t.Fatalf("Failed to get PipelineRun by UID: %v", err)
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
	if detail.Summary.Status != "True" {
		t.Errorf("Expected status True, got %s", detail.Summary.Status)
	}
	if detail.RecordName != fmt.Sprintf("%s/results/%s/records/%s", namespace, prUID, prUID) {
		t.Errorf("Unexpected record name: %s", detail.RecordName)
	}

	// Verify raw YAML/JSON is present
	if len(detail.Raw) == 0 {
		t.Error("Expected raw data to be present")
	}
}

// TestIntegration_StandaloneTaskRunUIDLookup tests lookup of a standalone TaskRun
func TestIntegration_StandaloneTaskRunUIDLookup(t *testing.T) {
	trUID := "tr-standalone-uid"
	trName := "standalone-taskrun"
	namespace := "integration-test"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := fmt.Sprintf("/apis/results.tekton.dev/v1alpha2/parents/%s/results/%s/records/%s",
			namespace, trUID, trUID)

		if r.URL.Path == expectedPath && r.Method == http.MethodGet {
			response := record{
				Name: fmt.Sprintf("%s/results/%s/records/%s", namespace, trUID, trUID),
				Uid:  trUID,
			}
			response.Data.Value = json.RawMessage(fmt.Sprintf(`{
				"apiVersion": "tekton.dev/v1",
				"kind": "TaskRun",
				"metadata": {
					"name": "%s",
					"namespace": "%s",
					"uid": "%s"
				},
				"spec": {
					"taskSpec": {
						"steps": [{"name": "step1", "image": "alpine"}]
					}
				},
				"status": {
					"conditions": [{
						"type": "Succeeded",
						"status": "True"
					}],
					"startTime": "2025-12-28T10:00:00Z",
					"completionTime": "2025-12-28T10:01:00Z"
				}
			}`, trName, namespace, trUID))
			json.NewEncoder(w).Encode(response)
			return
		}

		http.NotFound(w, r)
	}))
	defer server.Close()

	baseURL := server.URL + "/apis/results.tekton.dev/v1alpha2"
	parsedURL, _ := url.Parse(baseURL)
	client := &restClient{baseURL: parsedURL, httpClient: server.Client()}
	service := &Service{client: client}

	detail, err := service.getRun(context.Background(), resourceKindTaskRun, RunSelector{
		Namespace: namespace,
		UID:       trUID,
	})

	if err != nil {
		t.Fatalf("Failed to get standalone TaskRun by UID: %v", err)
	}

	if detail.Summary.UID != trUID {
		t.Errorf("Expected UID %s, got %s", trUID, detail.Summary.UID)
	}

	// Verify standalone TaskRun uses its own Result
	expectedRecordName := fmt.Sprintf("%s/results/%s/records/%s", namespace, trUID, trUID)
	if detail.RecordName != expectedRecordName {
		t.Errorf("Expected record name %s for standalone TaskRun, got %s", expectedRecordName, detail.RecordName)
	}
}

// TestIntegration_TaskRunInPipelineUIDLookup tests fallback for TaskRun that's part of a Pipeline
func TestIntegration_TaskRunInPipelineUIDLookup(t *testing.T) {
	trUID := "tr-in-pipeline-uid"
	prUID := "parent-pipeline-uid"
	trName := "test-pipeline-task1"
	namespace := "integration-test"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Direct GetRecord attempt (will fail)
		directPath := fmt.Sprintf("/apis/results.tekton.dev/v1alpha2/parents/%s/results/%s/records/%s",
			namespace, trUID, trUID)
		if r.URL.Path == directPath && r.Method == http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"code":5,"message":"record not found"}`))
			return
		}

		// Fallback: ListRecords request
		listPath := fmt.Sprintf("/apis/results.tekton.dev/v1alpha2/parents/%s/results/-/records", namespace)
		if r.URL.Path == listPath && r.Method == http.MethodGet {
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
						"tekton.dev/pipelineRun": "test-pipeline",
						"tekton.dev/pipelineRunUID": "%s",
						"tekton.dev/pipelineTask": "task1"
					}
				},
				"spec": {},
				"status": {
					"conditions": [{
						"type": "Succeeded",
						"status": "True"
					}],
					"startTime": "2025-12-28T10:00:00Z",
					"completionTime": "2025-12-28T10:02:00Z"
				}
			}`, trName, namespace, trUID, prUID))
			response := listRecordsResponse{
				Records: []record{rec},
			}
			json.NewEncoder(w).Encode(response)
			return
		}

		http.NotFound(w, r)
	}))
	defer server.Close()

	baseURL := server.URL + "/apis/results.tekton.dev/v1alpha2"
	parsedURL, _ := url.Parse(baseURL)
	client := &restClient{baseURL: parsedURL, httpClient: server.Client()}
	service := &Service{client: client}

	detail, err := service.getRun(context.Background(), resourceKindTaskRun, RunSelector{
		Namespace: namespace,
		UID:       trUID,
	})

	if err != nil {
		t.Fatalf("Failed to get TaskRun (in Pipeline) by UID: %v", err)
	}

	if detail.Summary.UID != trUID {
		t.Errorf("Expected UID %s, got %s", trUID, detail.Summary.UID)
	}

	// Verify TaskRun is stored under PipelineRun's Result
	expectedPrefix := fmt.Sprintf("%s/results/%s/records/", namespace, prUID)
	if !strings.HasPrefix(detail.RecordName, expectedPrefix) {
		t.Errorf("Expected record name to start with %s (PipelineRun's Result), got %s",
			expectedPrefix, detail.RecordName)
	}

	if detail.RecordName != fmt.Sprintf("%s/results/%s/records/%s", namespace, prUID, trUID) {
		t.Errorf("Unexpected record name format: %s", detail.RecordName)
	}
}

// TestIntegration_VerifyParentsPrefix verifies all API calls use the correct parents/ prefix
func TestIntegration_VerifyParentsPrefix(t *testing.T) {
	requestCount := 0
	var receivedPaths []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		receivedPaths = append(receivedPaths, r.URL.Path)

		// All requests should have parents/ in the path
		if !strings.Contains(r.URL.Path, "/parents/") {
			t.Errorf("Request %d missing parents/ prefix: %s", requestCount, r.URL.Path)
		}

		// Return minimal valid response
		if strings.Contains(r.URL.Path, "/records/") {
			if strings.HasSuffix(r.URL.Path, "/records") {
				// ListRecords
				json.NewEncoder(w).Encode(listRecordsResponse{Records: []record{}})
			} else {
				// GetRecord
				json.NewEncoder(w).Encode(record{Name: "test", Uid: "test"})
			}
		} else {
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	baseURL := server.URL + "/apis/results.tekton.dev/v1alpha2"
	parsedURL, _ := url.Parse(baseURL)
	client := &restClient{baseURL: parsedURL, httpClient: server.Client()}
	service := &Service{client: client}

	// Make various requests
	tests := []struct {
		name     string
		runType  resourceKind
		selector RunSelector
	}{
		{
			name:    "PipelineRun by UID",
			runType: resourceKindPipelineRun,
			selector: RunSelector{
				Namespace: "test",
				UID:       "pr-uid",
			},
		},
		{
			name:    "TaskRun by UID",
			runType: resourceKindTaskRun,
			selector: RunSelector{
				Namespace: "test",
				UID:       "tr-uid",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			beforeCount := requestCount
			service.getRun(context.Background(), tt.runType, tt.selector)
			afterCount := requestCount

			if afterCount == beforeCount {
				t.Error("Expected at least one request to be made")
			}
		})
	}

	t.Logf("Total requests made: %d", requestCount)
	t.Logf("Request paths: %v", receivedPaths)
}

// TestIntegration_Base64DecodedResponse tests handling of base64-encoded responses
func TestIntegration_Base64DecodedResponse(t *testing.T) {
	prUID := "pr-base64-test"
	namespace := "test"

	// Tekton Results API sometimes returns base64-encoded data
	rawJSON := `{"apiVersion":"tekton.dev/v1","kind":"PipelineRun","metadata":{"name":"test","namespace":"test","uid":"pr-base64-test"},"spec":{},"status":{}}`
	encoded := `"` + base64EncodeString(rawJSON) + `"`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := record{
			Name: fmt.Sprintf("%s/results/%s/records/%s", namespace, prUID, prUID),
			Uid:  prUID,
		}
		response.Data.Value = json.RawMessage(encoded)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	baseURL := server.URL + "/apis/results.tekton.dev/v1alpha2"
	parsedURL, _ := url.Parse(baseURL)
	client := &restClient{baseURL: parsedURL, httpClient: server.Client()}
	service := &Service{client: client}

	detail, err := service.getRun(context.Background(), resourceKindPipelineRun, RunSelector{
		Namespace: namespace,
		UID:       prUID,
	})

	if err != nil {
		t.Fatalf("Failed to handle base64-encoded response: %v", err)
	}

	if detail.Summary.Name != "test" {
		t.Errorf("Failed to decode base64 data, expected name 'test', got %s", detail.Summary.Name)
	}

	// Verify raw data is decoded
	if !strings.Contains(string(detail.Raw), "apiVersion") {
		t.Error("Raw data should be decoded from base64")
	}
}

// Helper function for base64 encoding
func base64EncodeString(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}
