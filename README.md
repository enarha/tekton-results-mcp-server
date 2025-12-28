# Tekton Results Model Context Protocol server

This project provides a [Model Context Protocol (MCP)](https://modelcontextprotocol.io) server for accessing historical Tekton PipelineRun and TaskRun data stored in [Tekton Results](https://github.com/tektoncd/results).

## Tools

### List Operations

#### `pipelinerun_list` – List PipelineRuns from Tekton Results with Filtering Options
- `namespace`: Namespace to list PipelineRuns from (string, optional, default: current kubeconfig namespace; use `-` for all namespaces)
- `labelSelector`: Label selector to filter PipelineRuns (string, optional, comma-separated `key=value` pairs)
- `prefix`: Name prefix to filter PipelineRuns (string, optional)
- `limit`: Maximum number of results to return (integer, optional, range: 1-200, default: 50)

#### `taskrun_list` – List TaskRuns from Tekton Results with Filtering Options
- `namespace`: Namespace to list TaskRuns from (string, optional, default: current kubeconfig namespace; use `-` for all namespaces)
- `labelSelector`: Label selector to filter TaskRuns (string, optional, comma-separated `key=value` pairs)
- `prefix`: Name prefix to filter TaskRuns (string, optional)
- `limit`: Maximum number of results to return (integer, optional, range: 1-200, default: 50)

### Get Operations

#### `pipelinerun_get` – Get a specific PipelineRun by name or filters
- `name`: Name of the PipelineRun to get (string, optional)
- `namespace`: Namespace of the PipelineRun (string, optional, default: current kubeconfig namespace; use `-` for all namespaces)
- `labelSelector`: Label selector to filter PipelineRuns (string, optional, comma-separated `key=value` pairs)
- `prefix`: Name prefix to filter PipelineRuns (string, optional)
- `uid`: Exact PipelineRun UID (string, optional). Unique identifier in Tekton Results database. This is the most efficient way to find a specific run.
- `output`: Return format - json or yaml (string, optional, default: "yaml")
- `selectLast`: If true, automatically select the most recent match when multiple runs match the filters (boolean, optional, default: true). When false, returns an error if multiple matches are found. Useful because run names are not unique in Tekton Results history.

#### `taskrun_get` – Get a specific TaskRun by name or filters
- `name`: Name of the TaskRun to get (string, optional)
- `namespace`: Namespace of the TaskRun (string, optional, default: current kubeconfig namespace; use `-` for all namespaces)
- `labelSelector`: Label selector to filter TaskRuns (string, optional, comma-separated `key=value` pairs)
- `prefix`: Name prefix to filter TaskRuns (string, optional)
- `uid`: Exact TaskRun UID (string, optional). Unique identifier in Tekton Results database. This is the most efficient way to find a specific run.
- `output`: Return format - json or yaml (string, optional, default: "yaml")
- `selectLast`: If true, automatically select the most recent match when multiple runs match the filters (boolean, optional, default: true). When false, returns an error if multiple matches are found. Useful because run names are not unique in Tekton Results history.

### Log Operations

#### `pipelinerun_logs` – Get logs for a PipelineRun
- `name`: Name of the PipelineRun to get logs from (string, optional)
- `namespace`: Namespace where the PipelineRun is located (string, optional, default: current kubeconfig namespace; use `-` for all namespaces)
- `labelSelector`: Label selector to filter PipelineRuns (string, optional, comma-separated `key=value` pairs)
- `prefix`: Name prefix to filter PipelineRuns (string, optional)
- `uid`: Exact PipelineRun UID (string, optional). Unique identifier in Tekton Results database. This is the most efficient way to find a specific run.
- `selectLast`: If true, automatically select the most recent match when multiple runs match the filters (boolean, optional, default: true). When false, returns an error if multiple matches are found. Useful because run names are not unique in Tekton Results history.

**Note:** This tool fetches logs from all TaskRuns associated with the PipelineRun, sorted by completion time in execution order. Logs are only available after the PipelineRun has completed.

#### `taskrun_logs` – Get logs for a TaskRun
- `name`: Name of the TaskRun to get logs from (string, optional)
- `namespace`: Namespace where the TaskRun is located (string, optional, default: current kubeconfig namespace; use `-` for all namespaces)
- `labelSelector`: Label selector to filter TaskRuns (string, optional, comma-separated `key=value` pairs)
- `prefix`: Name prefix to filter TaskRuns (string, optional)
- `uid`: Exact TaskRun UID (string, optional). Unique identifier in Tekton Results database. This is the most efficient way to find a specific run.
- `selectLast`: If true, automatically select the most recent match when multiple runs match the filters (boolean, optional, default: true). When false, returns an error if multiple matches are found. Useful because run names are not unique in Tekton Results history.

**Note:** Logs are only available after the TaskRun has completed and could even take a bit longer depending on logger confuguration (buffering, etc.).

## Handling Multiple Matches with `selectLast`

When using `pipelinerun_get`, `pipelinerun_logs`, `taskrun_get`, or `taskrun_logs`, you may encounter situations where multiple runs match your filters. This commonly happens because:

- Run names are unique on the cluster at any given time, but not in Tekton Results history
- The same PipelineRun/TaskRun name can be reused across multiple executions
- Each execution is stored with a unique UID but the same name

The `selectLast` parameter (defaults to `true`) controls this behavior:

**With `selectLast=true` (default):**
```json
{
  "name": "my-pipeline",
  "namespace": "default"
}
```
Automatically selects the most recent run.

**With `selectLast=false` (strict mode):**
```json
{
  "name": "my-pipeline",
  "namespace": "default",
  "selectLast": false
}
```
Returns error if multiple matches found:
```
Error: "multiple PipelineRun instances match the filters (default/my-pipeline, default/my-pipeline).
        Please refine the filters with an exact name or prefix."
```

To avoid ambiguity, you can:
- Use more specific name prefixes (e.g., `my-pipeline-abc123` instead of `my-pipeline`)
- Add label selectors to narrow results
- Use `selectLast=true` to automatically pick the most recent run

## Configuration

### Authentication

All tools rely on the in-cluster or kubeconfig context used to start the MCP server, so no additional Tekton Results credentials are required. When running outside the cluster, ensure the kubeconfig context has access to the Tekton Results aggregated API.

### Direct Tekton Results Access

If your cluster does not expose the aggregated API (for example when you port-forward `tekton-results-api-service`), set the following environment variables before starting the MCP server:

- `TEKTON_RESULTS_BASE_URL`: Base host for the API server (e.g., `https://localhost:8443`). The MCP server automatically appends `/apis/results.tekton.dev/v1alpha2`.
- `TEKTON_RESULTS_BEARER_TOKEN`: Optional bearer token to authenticate against the Tekton Results API. If omitted, the token from your kubeconfig is used.
- `TEKTON_RESULTS_INSECURE_SKIP_VERIFY`: Set to `true` when using self-signed certificates (for example, with port-forwarded services).

When these variables are not set, the MCP server communicates with Tekton Results through the Kubernetes aggregated API endpoint (`/apis/results.tekton.dev`).
