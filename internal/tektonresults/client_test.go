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

func TestRecordGetValue(t *testing.T) {
	tests := []struct {
		name    string
		record  record
		want    string
		wantErr bool
	}{
		{
			name: "plain JSON object",
			record: func() record {
				r := record{}
				r.Data.Value = json.RawMessage(`{"metadata":{"name":"test"}}`)
				return r
			}(),
			want:    `{"metadata":{"name":"test"}}`,
			wantErr: false,
		},
		{
			name: "base64 encoded JSON",
			record: func() record {
				r := record{}
				r.Data.Value = json.RawMessage(`"` + base64.StdEncoding.EncodeToString([]byte(`{"metadata":{"name":"encoded"}}`)) + `"`)
				return r
			}(),
			want:    `{"metadata":{"name":"encoded"}}`,
			wantErr: false,
		},
		{
			name: "empty value",
			record: func() record {
				r := record{}
				r.Data.Value = json.RawMessage(``)
				return r
			}(),
			want:    "",
			wantErr: false,
		},
		{
			name: "plain JSON array",
			record: func() record {
				r := record{}
				r.Data.Value = json.RawMessage(`[1,2,3]`)
				return r
			}(),
			want:    `[1,2,3]`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.record.GetValue()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if string(got) != tt.want {
				t.Errorf("GetValue() = %v, want %v", string(got), tt.want)
			}
		})
	}
}

func TestRestClient_GetRecord(t *testing.T) {
	tests := []struct {
		name           string
		recordName     string
		serverResponse func(w http.ResponseWriter, r *http.Request)
		wantErr        bool
		errContains    string
		wantRecord     *record
	}{
		{
			name:       "successful get with parents prefix",
			recordName: "foo/results/test-uid/records/test-uid",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				// Verify the path includes "parents/" prefix
				expectedPath := "/apis/results.tekton.dev/v1alpha2/parents/foo/results/test-uid/records/test-uid"
				if r.URL.Path != expectedPath {
					t.Errorf("Expected path %s, got %s", expectedPath, r.URL.Path)
				}

				resp := record{
					Name: "foo/results/test-uid/records/test-uid",
					Uid:  "test-uid",
				}
				resp.Data.Value = json.RawMessage(`{"metadata":{"name":"test-pr","uid":"test-uid"}}`)
				//nolint:errcheck // Writing to test HTTP response writer
				json.NewEncoder(w).Encode(resp)
			},
			wantErr: false,
			wantRecord: &record{
				Name: "foo/results/test-uid/records/test-uid",
				Uid:  "test-uid",
			},
		},
		{
			name:       "record not found",
			recordName: "foo/results/missing-uid/records/missing-uid",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				//nolint:errcheck // Writing to test HTTP response writer
				w.Write([]byte(`{"code":5,"message":"record not found"}`))
			},
			wantErr:     true,
			errContains: `"code":5`,
		},
		{
			name:       "empty record name",
			recordName: "",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				t.Error("Server should not be called with empty record name")
			},
			wantErr:     true,
			errContains: "record name is required",
		},
		{
			name:       "malformed JSON response",
			recordName: "foo/results/test-uid/records/test-uid",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				//nolint:errcheck // Writing to test HTTP response writer
				w.Write([]byte(`{invalid json`))
			},
			wantErr:     true,
			errContains: "decode record response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.serverResponse))
			defer server.Close()

			baseURL := server.URL + "/apis/results.tekton.dev/v1alpha2"
			parsedURL, err := url.Parse(baseURL)
			if err != nil {
				t.Fatalf("Failed to parse URL: %v", err)
			}

			client := &restClient{
				baseURL:    parsedURL,
				httpClient: server.Client(),
			}

			got, err := client.getRecord(context.Background(), tt.recordName)

			if (err != nil) != tt.wantErr {
				t.Errorf("getRecord() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errContains != "" {
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("Expected error to contain %q, got %q", tt.errContains, err.Error())
				}
				return
			}

			if !tt.wantErr && got != nil {
				if got.Name != tt.wantRecord.Name {
					t.Errorf("Expected record name %s, got %s", tt.wantRecord.Name, got.Name)
				}
				if got.Uid != tt.wantRecord.Uid {
					t.Errorf("Expected record UID %s, got %s", tt.wantRecord.Uid, got.Uid)
				}
			}
		})
	}
}

func TestRestClient_GetRecord_PathFormatting(t *testing.T) {
	// Test that various record name formats all result in correct path with parents/ prefix
	tests := []struct {
		name         string
		recordName   string
		expectedPath string
	}{
		{
			name:         "without leading slash",
			recordName:   "foo/results/uid/records/uid",
			expectedPath: "/apis/results.tekton.dev/v1alpha2/parents/foo/results/uid/records/uid",
		},
		{
			name:         "with leading slash",
			recordName:   "/foo/results/uid/records/uid",
			expectedPath: "/apis/results.tekton.dev/v1alpha2/parents/foo/results/uid/records/uid",
		},
		{
			name:         "different namespace",
			recordName:   "bar/results/another-uid/records/another-uid",
			expectedPath: "/apis/results.tekton.dev/v1alpha2/parents/bar/results/another-uid/records/another-uid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedPath string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedPath = r.URL.Path
				//nolint:errcheck // Writing to test HTTP response writer
				json.NewEncoder(w).Encode(record{Name: tt.recordName, Uid: "test"})
			}))
			defer server.Close()

			baseURL := server.URL + "/apis/results.tekton.dev/v1alpha2"
			parsedURL, _ := url.Parse(baseURL)
			client := &restClient{
				baseURL:    parsedURL,
				httpClient: server.Client(),
			}

			_, err := client.getRecord(context.Background(), tt.recordName)
			if err != nil {
				t.Fatalf("getRecord() unexpected error: %v", err)
			}

			if receivedPath != tt.expectedPath {
				t.Errorf("Expected path %s, got %s", tt.expectedPath, receivedPath)
			}
		})
	}
}

func TestRestClient_ListResults_MinimumPageSize(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pageSize := r.URL.Query().Get("page_size")
		if pageSize != "" && pageSize != "0" {
			// Verify page size is at least 5 (API requirement)
			var size int
			if _, err := fmt.Sscanf(pageSize, "%d", &size); err == nil {
				if size < 5 {
					w.WriteHeader(http.StatusBadRequest)
					//nolint:errcheck // Writing to test HTTP response writer
					w.Write([]byte(`{"code":3,"message":"invalid page size: value must be greater than 5"}`))
					return
				}
			}
		}

		//nolint:errcheck // Writing to test HTTP response writer
		json.NewEncoder(w).Encode(listResultsResponse{
			Results: []result{
				{Name: "foo/results/test-uid", UID: "test-uid"},
			},
		})
	}))
	defer server.Close()

	baseURL := server.URL + "/apis/results.tekton.dev/v1alpha2"
	parsedURL, _ := url.Parse(baseURL)
	client := &restClient{
		baseURL:    parsedURL,
		httpClient: server.Client(),
	}

	// Test with page size 5 (minimum)
	resp, err := client.listResults(context.Background(), listResultsRequest{
		Parent:   "foo",
		Filter:   `uid=="test-uid"`,
		PageSize: 5,
	})

	if err != nil {
		t.Errorf("listResults with page_size=5 failed: %v", err)
	}
	if len(resp.Results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(resp.Results))
	}
}
