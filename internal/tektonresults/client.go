package tektonresults

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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
)

type restClient struct {
	baseURL    *url.URL
	httpClient *http.Client
}

// newRESTClient creates a lightweight HTTP client that reuses the Kubernetes
// rest.Config for authentication while targeting the Tekton Results aggregated API.
func newRESTClient(cfg *rest.Config) (*restClient, error) {
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

type record struct {
	Name string `json:"name"`
	Uid  string `json:"uid"`
	Data struct {
		Value json.RawMessage `json:"value"`
	} `json:"data"`
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

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("perform %s request: %w", method, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("results API %s %s: %s", method, u.Path, strings.TrimSpace(string(data)))
	}

	return data, nil
}
