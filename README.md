# tekton-results-mcp-server

MCP Server for Tekton Results

## Available tools

The server exposes read-only tools for PipelineRuns and TaskRuns inside a Tekton
Results deployment:

- `pipelinerun_list` / `taskrun_list` – return JSON summaries filtered by
  namespace, label selectors, name prefixes, and result count limits.
- `pipelinerun_describe` / `taskrun_describe` – fetch a single run and emit the
  full resource in YAML (default) or JSON.
- `pipelinerun_logs` / `taskrun_logs` – stream stored logs for completed runs.

The filters mirror Tekton Results behavior:

- `namespace`: defaults to the current kubeconfig namespace; use `-` to search
  all namespaces.
- `labelSelector`: comma separated `key=value` pairs.
- `prefix`: run name prefix.
- `limit` (list only): cap returned items between 1 and 200.

All tools rely on the in-cluster or kubeconfig context used to start the MCP
server, so no additional Tekton Results credentials are required. When running
outside the cluster, ensure the kubeconfig context has access to the Tekton
Results aggregated API.
