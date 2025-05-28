package resources

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func Add(ctx context.Context, s *server.MCPServer) {
	s.AddResourceTemplate(exampleResources(ctx))
}

func exampleResources(ctx context.Context) (mcp.ResourceTemplate, server.ResourceTemplateHandlerFunc) {
	return mcp.NewResourceTemplate(
		"tekton://example/{namespace}/{name}",
		"Example",
	), exampleHandler(ctx)
}

func exampleHandler(ctx context.Context) server.ResourceTemplateHandlerFunc {
	return func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		ns, ok := request.Params.Arguments["namespace"].([]string)
		if !ok || len(ns) == 0 {
			return nil, errors.New("namespace is required")
		}
		namespace := ns[0]

		n, ok := request.Params.Arguments["name"].([]string)
		if !ok || len(n) == 0 {
			return nil, errors.New("name is required")
		}
		name := n[0]

		exampleData := map[string]string{
			"namespace": namespace,
			"name":      name,
			"type":      "example",
		}

		jsonData, err := json.Marshal(exampleData)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal resource to JSON: %w", err)
		}

		contents := mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "application/json;type=example",
			Text:     string(jsonData),
		}

		return []mcp.ResourceContents{contents}, nil
	}
}
