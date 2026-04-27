package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path"

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

// healthPassthrough returns 200 for simple GETs without an MCP session so health checks and
// manual probes (e.g. curl host:8052/ or :8052/mcp) work. The streamable MCP handler requires
// Mcp-Session-Id and JSON-RPC for real traffic; use an MCP HTTP client for that.
func healthPassthrough(inner http.Handler) http.Handler {
	healthPaths := map[string]struct{}{
		"/":        {},
		"/mcp":     {},
		"/health":  {},
		"/healthz": {},
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.Header.Get("Mcp-Session-Id") != "" {
			inner.ServeHTTP(w, r)
			return
		}
		p := path.Clean(r.URL.Path)
		if p == "." {
			p = "/"
		}
		if _, ok := healthPaths[p]; ok {
			w.WriteHeader(http.StatusOK)
			return
		}
		inner.ServeHTTP(w, r)
	})
}
