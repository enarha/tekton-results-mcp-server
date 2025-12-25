# tekton-results-mcp-server

MCP Server for Tekton Results

## Available tools

The server exposes read-only tools for PipelineRuns and TaskRuns inside a Tekton
Results deployment:

- `pipelinerun_list` / `taskrun_list` – return JSON summaries filtered by
  namespace, label selectors, name prefixes, and result count limits.
- `pipelinerun_describe` / `taskrun_describe` – fetch a single run and emit the
  full resource in YAML (default) or JSON.
- `pipelinerun_logs` / `taskrun_logs` – retrieve stored logs for completed runs.
  - For PipelineRuns: fetches logs from all TaskRuns in execution order (sorted by completion time).
  - For TaskRuns: fetches logs directly from the TaskRun.

The filters mirror Tekton Results behavior:

- `namespace`: defaults to the current kubeconfig namespace; use `-` to search
  all namespaces.
- `labelSelector`: comma separated `key=value` pairs.
- `prefix`: run name prefix.
- `limit` (list only): cap returned items between 1 and 200.
- `selectLast` (describe and logs only): when `true` (default), automatically
  selects the most recent match if multiple runs match the filters. When `false`,
  returns an error if multiple matches are found, requiring more specific filters.
  This is useful because run names are not unique in Tekton Results history.

All tools rely on the in-cluster or kubeconfig context used to start the MCP
server, so no additional Tekton Results credentials are required. When running
outside the cluster, ensure the kubeconfig context has access to the Tekton
Results aggregated API.

### Handling multiple matches with `selectLast`

When using `pipelinerun_describe`, `pipelinerun_logs`, `taskrun_describe`, or
`taskrun_logs`, you may encounter situations where multiple runs match your
filters. This commonly happens because:

- Run names are unique on the cluster at any given time, but not in Tekton
  Results history
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

### Direct Tekton Results access

If your cluster does not expose the aggregated API (for example when you
port-forward `tekton-results-api-service`), set the following environment
variables before starting the MCP server:

- `TEKTON_RESULTS_BASE_URL`: Base host for the API server (e.g.
  `https://localhost:8443`). The MCP server automatically appends
  `/apis/results.tekton.dev/v1alpha2`.
- `TEKTON_RESULTS_BEARER_TOKEN`: Optional bearer token to authenticate against
  the Tekton Results API. If omitted, the token from your kubeconfig is used.
- `TEKTON_RESULTS_INSECURE_SKIP_VERIFY`: Set to `true` when using self-signed
  certificates (for example, with port-forwarded services).

When these variables are not set, the MCP server communicates with Tekton
Results through the Kubernetes aggregated API endpoint (`/apis/results.tekton.dev`).
