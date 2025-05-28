package main

import (
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/enarha/tekton-results-mcp-server/internal/tektonresults"
	"github.com/enarha/tekton-results-mcp-server/internal/tools"
	"github.com/mark3labs/mcp-go/server"
	"k8s.io/client-go/tools/clientcmd"
	"knative.dev/pkg/signals"
)

func main() {
	var transport string
	var httpAddr string
	flag.StringVar(&transport, "transport", "http", "Transport type (stdio or http)")
	flag.StringVar(&httpAddr, "address", ":8080", "Address to bind the HTTP server to")
	flag.Parse()

	// For stdio mode, disable slog output to avoid polluting the JSON-RPC protocol
	if transport == "stdio" {
		slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	}

	if httpAddr == "" && transport == "http" {
		slog.Error("-address is required when transport is set to 'hhtp'")
		os.Exit(1)
	}

	// Create MCP server
	s := server.NewMCPServer(
		"Tekton Results MCP Server",
		"0.0.1", // FIXME get this from internal package
		server.WithToolCapabilities(true),
		server.WithLogging(),
	)

	ctx := signals.NewContext()

	// Load kubernetes configuration
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	cfg, err := kubeConfig.ClientConfig()
	if err != nil {
		slog.Error(fmt.Sprintf("failed to get Kubernetes config: %v", err))
		os.Exit(1)
	}

	namespace, _, err := kubeConfig.Namespace()
	if err != nil || namespace == "" {
		namespace = "default"
	}

	overrides := tektonresults.Overrides{
		Host:        os.Getenv("TEKTON_RESULTS_BASE_URL"),
		BearerToken: os.Getenv("TEKTON_RESULTS_BEARER_TOKEN"),
	}
	if v := os.Getenv("TEKTON_RESULTS_INSECURE_SKIP_VERIFY"); v != "" {
		if b, parseErr := strconv.ParseBool(v); parseErr == nil {
			overrides.InsecureSkipVerify = b
		} else {
			slog.Warn("invalid TEKTON_RESULTS_INSECURE_SKIP_VERIFY value, ignoring", "value", v)
		}
	}

	resultsSvc, err := tektonresults.NewService(cfg, overrides)
	if err != nil {
		slog.Error(fmt.Sprintf("failed to initialize Tekton Results client: %v", err))
		os.Exit(1)
	}

	slog.Info("Adding tools and resources to the server.")
	if err := tools.Add(ctx, s, tools.Dependencies{
		Service:          resultsSvc,
		DefaultNamespace: namespace,
	}); err != nil {
		slog.Error(fmt.Sprintf("failed to add tools: %v", err))
		os.Exit(1)
	}
	// resources.Add(ctx, s)

	slog.Info("Starting the server.")

	errC := make(chan error, 1)

	switch transport {
	case "http":
		streamableHandler := server.NewStreamableHTTPServer(s)
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			streamableHandler.ServeHTTP(w, r.WithContext(ctx))
		})
		server := &http.Server{
			Addr:              httpAddr,
			Handler:           handler,
			ReadHeaderTimeout: 3 * time.Second,
		}
		go func() {
			errC <- server.ListenAndServe()
		}()
		slog.Info("Tekton Results MCP Server is listening at " + httpAddr)
	case "stdio":
		stdioServer := server.NewStdioServer(s)
		go func() {
			in, out := io.Reader(os.Stdin), io.Writer(os.Stdout)
			errC <- stdioServer.Listen(ctx, in, out)
		}()
	default:
		slog.Error(fmt.Sprintf("Invalid transport %q; must be http or stdio", transport))
		os.Exit(1)
	}

	// Wait for shutdown signal
	select {
	case <-ctx.Done():
		slog.Info("Shutting down server...")
	case err := <-errC:
		if err != nil {
			slog.Error(fmt.Sprintf("Error running server: %v", err))
			os.Exit(1)
		}
	}
}
