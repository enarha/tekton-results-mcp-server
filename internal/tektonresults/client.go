package tektonresults

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
)

const (
	apiPathSegment = "apis"
	resultsGroup   = "results.tekton.dev"
	resultsVersion = "v1alpha2"
	defaultTimeout = 30 * time.Second
	customAPIPath  = "/apis/results.tekton.dev/v1alpha2"
)

type restClient struct {
	baseURL    *url.URL
	httpClient *http.Client
	authToken  string
}

type Overrides struct {
	Host               string
	BearerToken        string
	InsecureSkipVerify bool
}

// newRESTClient creates a lightweight HTTP client that reuses the Kubernetes
// rest.Config for authentication while targeting the Tekton Results aggregated API.
func newRESTClient(cfg *rest.Config, overrides Overrides) (*restClient, error) {
	if overrides.Host != "" {
		return newCustomClient(cfg, overrides)
	}

	if cfg == nil {
		return nil, fmt.Errorf("kubernetes config is required")
	}

	rc := rest.CopyConfig(cfg)
	if rc.Timeout == 0 {
		rc.Timeout = defaultTimeout
	}
	rc.APIPath = path.Join(rc.APIPath, apiPathSegment)
	gv := schema.GroupVersion{
		Group:   resultsGroup,
		Version: resultsVersion,
	}
	rc.GroupVersion = &gv

	httpClient, err := rest.HTTPClientFor(rc)
	if err != nil {
		return nil, fmt.Errorf("create http client: %w", err)
	}

	baseURL, versionedPath, err := rest.DefaultServerUrlFor(rc)
	if err != nil {
		return nil, fmt.Errorf("resolve results endpoint: %w", err)
	}
	baseURL.Path = versionedPath

	return &restClient{
		baseURL:    baseURL,
		httpClient: httpClient,
	}, nil
}

type listRecordsRequest struct {
	Parent    string
	Filter    string
	OrderBy   string
	PageSize  int32
	PageToken string
	Fields    string
}

type listRecordsResponse struct {
	Records       []record `json:"records"`
	NextPageToken string   `json:"nextPageToken"`
}

type listResultsRequest struct {
	Parent    string
	Filter    string
	OrderBy   string
	PageSize  int32
	PageToken string
}

type result struct {
	Name string `json:"name"`
	UID  string `json:"uid"`
}

type listResultsResponse struct {
	Results      []result `json:"results"`
	NextPageToken string  `json:"nextPageToken"`
}

type record struct {
	Name string `json:"name"`
	Uid  string `json:"uid"`
	Data struct {
		Value        json.RawMessage `json:"value"`
		valueDecoded json.RawMessage // cached decoded value
	} `json:"data"`
}

// GetValue returns the decoded value, handling base64 encoding if present
func (r *record) GetValue() (json.RawMessage, error) {
	if r.Data.valueDecoded != nil {
		return r.Data.valueDecoded, nil
	}

	var base64Str string

	// Try to unmarshal as JSON first
	var test interface{}
	if err := json.Unmarshal(r.Data.Value, &test); err == nil {
		// If it's already valid JSON (object or array), use it directly
		if _, isString := test.(string); !isString {
			r.Data.valueDecoded = r.Data.Value
			return r.Data.Value, nil
		}
		// It's a JSON string, extract it for base64 decoding
		if err := json.Unmarshal(r.Data.Value, &base64Str); err != nil {
			return nil, fmt.Errorf("decode base64 string from JSON: %w", err)
		}
	} else {
		// Assume it's raw base64 (shouldn't happen but handle it)
		base64Str = string(r.Data.Value)
	}

	// Decode base64 (common path for both JSON string and raw base64)
	decoded, err := base64.StdEncoding.DecodeString(base64Str)
	if err != nil {
		return nil, fmt.Errorf("decode base64: %w", err)
	}
	r.Data.valueDecoded = json.RawMessage(decoded)
	return r.Data.valueDecoded, nil
}

func (c *restClient) listResults(ctx context.Context, req listResultsRequest) (*listResultsResponse, error) {
	if req.Parent == "" {
		return nil, fmt.Errorf("parent is required")
	}

	params := url.Values{}
	if req.Filter != "" {
		params.Set("filter", req.Filter)
	}
	if req.OrderBy != "" {
		params.Set("order_by", req.OrderBy)
	}
	if req.PageSize > 0 {
		params.Set("page_size", fmt.Sprintf("%d", req.PageSize))
	}
	if req.PageToken != "" {
		params.Set("page_token", req.PageToken)
	}

	relative := fmt.Sprintf("parents/%s/results", strings.TrimPrefix(req.Parent, "/"))
	body, err := c.do(ctx, http.MethodGet, relative, params)
	if err != nil {
		return nil, err
	}

	var resp listResultsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode list results response: %w", err)
	}
	return &resp, nil
}

func (c *restClient) getRecord(ctx context.Context, recordName string) (*record, error) {
	if recordName == "" {
		return nil, fmt.Errorf("record name is required")
	}

	// Record name format: "namespace/results/result-uid/records/record-uid"
	// REST API requires "parents/" prefix
	relative := fmt.Sprintf("parents/%s", strings.TrimPrefix(recordName, "/"))
	body, err := c.do(ctx, http.MethodGet, relative, nil)
	if err != nil {
		return nil, err
	}

	var rec record
	if err := json.Unmarshal(body, &rec); err != nil {
		return nil, fmt.Errorf("decode record response: %w", err)
	}
	return &rec, nil
}

func (c *restClient) listRecords(ctx context.Context, req listRecordsRequest) (*listRecordsResponse, error) {
	if req.Parent == "" {
		return nil, fmt.Errorf("parent is required")
	}

	params := url.Values{}
	if req.Filter != "" {
		params.Set("filter", req.Filter)
	}
	if req.OrderBy != "" {
		params.Set("order_by", req.OrderBy)
	}
	if req.PageSize > 0 {
		params.Set("page_size", fmt.Sprintf("%d", req.PageSize))
	}
	if req.PageToken != "" {
		params.Set("page_token", req.PageToken)
	}
	if req.Fields != "" {
		params.Set("fields", req.Fields)
	}

	relative := fmt.Sprintf("parents/%s/records", strings.TrimPrefix(req.Parent, "/"))
	body, err := c.do(ctx, http.MethodGet, relative, params)
	if err != nil {
		return nil, err
	}

	var resp listRecordsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode list records response: %w", err)
	}
	return &resp, nil
}

func (c *restClient) getLog(ctx context.Context, logPath string) ([]byte, error) {
	if logPath == "" {
		return nil, fmt.Errorf("log path is required")
	}
	relative := fmt.Sprintf("parents/%s", strings.TrimPrefix(logPath, "/"))
	return c.do(ctx, http.MethodGet, relative, nil)
}

func (c *restClient) do(ctx context.Context, method, relPath string, params url.Values) ([]byte, error) {
	u := *c.baseURL
	u.Path = path.Join(c.baseURL.Path, relPath)
	if params != nil {
		u.RawQuery = params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create %s request: %w", method, err)
	}
	req.Header.Set("Accept", "application/json")
	if c.authToken != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.authToken))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("perform %s request: %w", method, err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			slog.Warn("failed to close response body", "error", closeErr)
		}
	}()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("results API %s %s: %s", method, u.Path, strings.TrimSpace(string(data)))
	}

	return data, nil
}

func newCustomClient(cfg *rest.Config, overrides Overrides) (*restClient, error) {
	baseURL, err := url.Parse(overrides.Host)
	if err != nil {
		return nil, fmt.Errorf("parse TEKTON_RESULTS_BASE_URL: %w", err)
	}
	if baseURL.Scheme == "" {
		baseURL.Scheme = "https"
	}
	if baseURL.Host == "" {
		return nil, fmt.Errorf("TEKTON_RESULTS_BASE_URL must include host")
	}
	if !strings.Contains(baseURL.Path, resultsGroup) {
		baseURL.Path = path.Join(baseURL.Path, customAPIPath)
	}

	token := overrides.BearerToken
	if token == "" && cfg != nil {
		token = cfg.BearerToken
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	if overrides.InsecureSkipVerify {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   defaultTimeout,
	}

	return &restClient{
		baseURL:    baseURL,
		httpClient: client,
		authToken:  token,
	}, nil
}
