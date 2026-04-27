package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/pab1it0/prometheus-mcp-server/internal/prommcpserver"
)

func main() {
	_ = godotenv.Load()
	log := prommcpserver.NewLogger()

	cfg, err := prommcpserver.LoadConfig()
	if err != nil {
		log.Error("configuration error", "error", err)
		os.Exit(1)
	}
	if err := cfg.Validate(); err != nil {
		log.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	prom, err := prommcpserver.NewPrometheusClient(cfg, log)
	if err != nil {
		log.Error("prometheus client", "error", err)
		os.Exit(1)
	}

	srv := mcp.NewServer(&mcp.Implementation{
		Name:    cfg.ServerName(),
		Version: prommcpserver.Version,
	}, &mcp.ServerOptions{Logger: log})

	deps := &prommcpserver.Deps{
		Cfg:     cfg,
		Prom:    prom,
		Log:     log,
		Metrics: &prommcpserver.MetricsCache{},
	}
	deps.RegisterTools(srv)

	ctx := context.Background()
	switch cfg.MCPTransport {
	case "stdio":
		log.Info("starting Prometheus MCP Server", "transport", "stdio")
		if err := srv.Run(ctx, &mcp.StdioTransport{}); err != nil {
			log.Error("server stopped", "error", err)
			os.Exit(1)
		}
	case "http", "sse":
		addr := fmt.Sprintf("%s:%d", cfg.MCPBindHost, cfg.MCPBindPort)
		log.Info("starting Prometheus MCP Server", "transport", cfg.MCPTransport, "addr", addr)
		handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, nil)
		if err := http.ListenAndServe(addr, healthPassthrough(handler)); err != nil {
			log.Error("http server", "error", err)
			os.Exit(1)
		}
	}
}

// healthPassthrough returns 200 for GET / without an MCP session so Docker healthchecks succeed.
func healthPassthrough(inner http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/" && r.Header.Get("Mcp-Session-Id") == "" {
			w.WriteHeader(http.StatusOK)
			return
		}
		inner.ServeHTTP(w, r)
	})
}
