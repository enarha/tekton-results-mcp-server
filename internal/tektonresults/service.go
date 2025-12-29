package tektonresults

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/yaml"
)

const (
	listFields                = "records.name,records.uid,records.data.value.metadata,records.data.value.status,next_page_token"
	nameUIDAndDataField       = "records.name,records.uid,records.data.value"
	defaultListLimit    int   = 50
	maxPageSize         int32 = 200
	describePageSize    int32 = 50
)

type resourceKind string

const (
	resourceKindPipelineRun resourceKind = "pipelinerun"
	resourceKindTaskRun     resourceKind = "taskrun"
)

var resourceTypeFilters = map[resourceKind][]string{
	resourceKindPipelineRun: {"tekton.dev/v1.PipelineRun", "tekton.dev/v1beta1.PipelineRun"},
	resourceKindTaskRun:     {"tekton.dev/v1.TaskRun", "tekton.dev/v1beta1.TaskRun"},
}

// Service exposes convenience helpers to interact with Tekton Results.
// resultsClient is an interface for interacting with the Tekton Results API
type resultsClient interface {
	getRecord(ctx context.Context, recordName string) (*record, error)
	listResults(ctx context.Context, req listResultsRequest) (*listResultsResponse, error)
	listRecords(ctx context.Context, req listRecordsRequest) (*listRecordsResponse, error)
	getLog(ctx context.Context, logPath string) ([]byte, error)
}

type Service struct {
	client resultsClient
}

// NewService constructs a Service using the Kubernetes REST config for auth.
func NewService(cfg *rest.Config, overrides Overrides) (*Service, error) {
	rc, err := newRESTClient(cfg, overrides)
	if err != nil {
		return nil, err
	}
	return &Service{client: rc}, nil
}

// ListPipelineRuns returns summaries of PipelineRuns.
func (s *Service) ListPipelineRuns(ctx context.Context, opts ListOptions) ([]RunSummary, error) {
	return s.listRuns(ctx, resourceKindPipelineRun, opts)
}

// ListTaskRuns returns summaries of TaskRuns.
func (s *Service) ListTaskRuns(ctx context.Context, opts ListOptions) ([]RunSummary, error) {
	return s.listRuns(ctx, resourceKindTaskRun, opts)
}

// GetPipelineRun returns the detailed Run representation.
func (s *Service) GetPipelineRun(ctx context.Context, selector RunSelector) (*RunDetail, error) {
	return s.getRun(ctx, resourceKindPipelineRun, selector)
}

// GetTaskRun returns the detailed Run representation.
func (s *Service) GetTaskRun(ctx context.Context, selector RunSelector) (*RunDetail, error) {
	return s.getRun(ctx, resourceKindTaskRun, selector)
}

// FetchLogs downloads the log payload referenced by the record name.
func (s *Service) FetchLogs(ctx context.Context, recordName string) (string, error) {
	logPath := strings.Replace(recordName, "/records/", "/logs/", 1)
	if logPath == recordName {
		logPath = strings.Replace(recordName, "records", "logs", 1)
	}
	data, err := s.client.getLog(ctx, logPath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

type ListOptions struct {
	Namespace     string
	LabelSelector string
	Prefix        string
	Limit         int
}

// RunSelector specifies filters for finding a single PipelineRun or TaskRun.
type RunSelector struct {
	Namespace     string // Kubernetes namespace; use "-" for all namespaces
	LabelSelector string // Comma-separated key=value label filters
	Prefix        string // Name prefix filter
	Name          string // Exact name match (not unique in Results history)
	UID           string // Exact UID match (unique identifier in Tekton Results database)
	SelectLast    bool   // If true, automatically select the most recent match when multiple runs match the filters.
	// Defaults to true. When false, returns an error if multiple matches are found.
	// Useful because run names are not unique in Tekton Results history.
}

type RunSummary struct {
	Name           string            `json:"name"`
	Namespace      string            `json:"namespace"`
	UID            string            `json:"uid,omitempty"`
	Labels         map[string]string `json:"labels,omitempty"`
	StartTime      *metav1.Time      `json:"startTime,omitempty"`
	CompletionTime *metav1.Time      `json:"completionTime,omitempty"`
	Status         string            `json:"status,omitempty"`
	Reason         string            `json:"reason,omitempty"`
	RecordName     string            `json:"recordName"`
}

type RunDetail struct {
	Summary    RunSummary
	Raw        json.RawMessage
	RecordName string
}

func (d RunDetail) Completed() bool {
	return d.Summary.CompletionTime != nil
}

func (d RunDetail) Format(output string) (string, error) {
	switch strings.ToLower(output) {
	case "json":
		var buf bytes.Buffer
		if err := json.Indent(&buf, d.Raw, "", "  "); err != nil {
			return "", fmt.Errorf("format JSON: %w", err)
		}
		return buf.String(), nil
	case "yaml", "":
		yamlBody, err := yaml.JSONToYAML(d.Raw)
		if err != nil {
			return "", fmt.Errorf("format YAML: %w", err)
		}
		return string(yamlBody), nil
	default:
		return "", fmt.Errorf("unsupported output %q", output)
	}
}

type tektonRun struct {
	Metadata struct {
		Name      string            `json:"name"`
		Namespace string            `json:"namespace"`
		UID       string            `json:"uid"`
		Labels    map[string]string `json:"labels"`
	} `json:"metadata"`
	Status struct {
		StartTime      *metav1.Time `json:"startTime"`
		CompletionTime *metav1.Time `json:"completionTime"`
		Conditions     []struct {
			Type    string `json:"type"`
			Status  string `json:"status"`
			Reason  string `json:"reason"`
			Message string `json:"message"`
		} `json:"conditions"`
	} `json:"status"`
}

func (s *Service) listRuns(ctx context.Context, kind resourceKind, opts ListOptions) ([]RunSummary, error) {
	labelFilters, err := parseLabelSelector(opts.LabelSelector)
	if err != nil {
		return nil, err
	}

	filter := buildFilterExpression(kind, labelFilters, "", "")
	parent := parentForNamespace(opts.Namespace)

	limit := opts.Limit
	if limit <= 0 {
		limit = defaultListLimit
	}
	pageSize := int32(limit)
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	req := listRecordsRequest{
		Parent:   parent,
		Filter:   filter,
		OrderBy:  "create_time desc",
		PageSize: pageSize,
		Fields:   listFields,
	}

	var summaries []RunSummary
	for {
		resp, err := s.client.listRecords(ctx, req)
		if err != nil {
			return nil, err
		}
		for _, rec := range resp.Records {
			run, err := decodeRun(rec)
			if err != nil {
				return nil, err
			}
			if !matchesLabels(run.Metadata.Labels, labelFilters) {
				continue
			}
			if opts.Prefix != "" && !strings.HasPrefix(run.Metadata.Name, opts.Prefix) {
				continue
			}
			summaries = append(summaries, summarizeRun(run, rec))
			if len(summaries) >= limit {
				return summaries, nil
			}
		}
		if resp.NextPageToken == "" {
			break
		}
		req.PageToken = resp.NextPageToken
		remaining := limit - len(summaries)
		if remaining <= 0 {
			break
		}
		if remaining < int(req.PageSize) {
			req.PageSize = int32(remaining)
		}
	}

	return summaries, nil
}

func (s *Service) getRun(ctx context.Context, kind resourceKind, selector RunSelector) (*RunDetail, error) {
	labelFilters, err := parseLabelSelector(selector.LabelSelector)
	if err != nil {
		return nil, err
	}

	// Optimized UID lookup: try direct GetRecord first
	if selector.UID != "" {
		ns := selector.Namespace
		if ns == "" {
			ns = "default"
		}
		recordName := fmt.Sprintf("%s/results/%s/records/%s", ns, selector.UID, selector.UID)
		rec, err := s.client.getRecord(ctx, recordName)
		if err == nil {
			// Found directly, decode and return
			run, err := decodeRun(*rec)
			if err != nil {
				return nil, fmt.Errorf("decode run from direct get: %w", err)
			}
			rawValue, err := rec.GetValue()
			if err != nil {
				return nil, fmt.Errorf("get value for detail from direct get: %w", err)
			}
			return &RunDetail{
				Summary:    summarizeRun(run, *rec),
				Raw:        rawValue,
				RecordName: rec.Name,
			}, nil
		}

		// If direct GetRecord failed for a TaskRun, it might be part of a PipelineRun.
		// Fallback to searching across all Records in the namespace.
		if kind == resourceKindTaskRun && strings.Contains(err.Error(), `"code":5`) {
			slog.Info("Direct GetRecord failed for TaskRun, falling back to namespace-wide search", "uid", selector.UID, "error", err)
			// Fall through to the standard query path below, which will filter by UID in memory
		} else {
			// For PipelineRuns, or other errors, direct GetRecord failure means it doesn't exist or other issue
			return nil, fmt.Errorf("get record by UID: %w", err)
		}
	}

	// Non-UID query path: use standard filtering
	resultParent := parentForNamespace(selector.Namespace)
	filter := buildFilterExpression(kind, labelFilters, selector.Name, "")
	req := listRecordsRequest{
		Parent:   resultParent,
		Filter:   filter,
		OrderBy:  "create_time desc",
		PageSize: describePageSize,
		Fields:   nameUIDAndDataField,
	}
	return s.queryRecords(ctx, req, selector)
}

// queryRecords handles the common logic for querying and filtering records
func (s *Service) queryRecords(ctx context.Context, req listRecordsRequest, selector RunSelector) (*RunDetail, error) {
	labelFilters, err := parseLabelSelector(selector.LabelSelector)
	if err != nil {
		return nil, err
	}

	var matches []RunDetail
	for {
		resp, err := s.client.listRecords(ctx, req)
		if err != nil {
			return nil, err
		}
		for _, rec := range resp.Records {
			run, err := decodeRun(rec)
			if err != nil {
				return nil, err
			}
			// Apply in-memory filters
			if selector.UID != "" {
				recordUID := chooseString(run.Metadata.UID, rec.Uid)
				if recordUID != selector.UID {
					continue
				}
			}
			if !matchesLabels(run.Metadata.Labels, labelFilters) {
				continue
			}
			if selector.Prefix != "" && !strings.HasPrefix(run.Metadata.Name, selector.Prefix) {
				continue
			}
			if selector.Name != "" && run.Metadata.Name != selector.Name {
				continue
			}
			rawValue, err := rec.GetValue()
			if err != nil {
				return nil, fmt.Errorf("get value for detail: %w", err)
			}
			matches = append(matches, RunDetail{
				Summary:    summarizeRun(run, rec),
				Raw:        rawValue,
				RecordName: rec.Name,
			})
			if len(matches) > 1 {
				break
			}
		}
		if len(matches) > 1 || resp.NextPageToken == "" {
			break
		}
		req.PageToken = resp.NextPageToken
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no run found that matches the provided filters")
	}
	if len(matches) > 1 {
		// If SelectLast is enabled, return the first match (most recent due to create_time desc ordering)
		if selector.SelectLast {
			return &matches[0], nil
		}
		var names []string
		for _, match := range matches {
			names = append(names, fmt.Sprintf("%s/%s", match.Summary.Namespace, match.Summary.Name))
		}
		return nil, fmt.Errorf("multiple run instances match the filters (%s). Please refine the filters with an exact name or prefix.", strings.Join(names, ", "))
	}

	return &matches[0], nil
}

func decodeRun(rec record) (tektonRun, error) {
	value, err := rec.GetValue()
	if err != nil {
		return tektonRun{}, fmt.Errorf("get value for record %s: %w", rec.Name, err)
	}
	if len(value) == 0 {
		return tektonRun{}, fmt.Errorf("record %s has no embedded Tekton data", rec.Name)
	}
	var run tektonRun
	if err := json.Unmarshal(value, &run); err != nil {
		return tektonRun{}, fmt.Errorf("decode Tekton resource in record %s: %w", rec.Name, err)
	}
	return run, nil
}

func summarizeRun(run tektonRun, rec record) RunSummary {
	status, reason := conditionStatus(run.Status.Conditions)
	return RunSummary{
		Name:           run.Metadata.Name,
		Namespace:      run.Metadata.Namespace,
		UID:            chooseString(run.Metadata.UID, rec.Uid),
		Labels:         run.Metadata.Labels,
		StartTime:      run.Status.StartTime,
		CompletionTime: run.Status.CompletionTime,
		Status:         status,
		Reason:         reason,
		RecordName:     rec.Name,
	}
}

func conditionStatus(conditions []struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}) (string, string) {
	for _, cond := range conditions {
		if cond.Type == "Succeeded" {
			return cond.Status, cond.Reason
		}
	}
	return "", ""
}

func chooseString(primary, fallback string) string {
	if primary != "" {
		return primary
	}
	return fallback
}
