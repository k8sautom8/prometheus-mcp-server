package prommcpserver

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCPExecuteQueryStructuredContent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "success",
			"data": map[string]any{
				"resultType": "vector",
				"result": []any{
					map[string]any{"metric": map[string]any{"__name__": "up"}, "value": []any{1617898448.214, "1"}},
				},
			},
		})
	}))
	defer ts.Close()

	cfg := &Config{
		PrometheusURL:       ts.URL,
		MCPTransport:        "stdio",
		RequestTimeoutSec:   10,
		DisablePrometheusLinks: false,
	}
	prom, err := NewPrometheusClient(cfg, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}

	srv := mcp.NewServer(&mcp.Implementation{Name: cfg.ServerName(), Version: Version}, nil)
	deps := &Deps{Cfg: cfg, Prom: prom, Log: slog.New(slog.DiscardHandler), Metrics: &MetricsCache{}}
	deps.RegisterTools(srv)

	ctx := context.Background()
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v1"}, nil)
	t1, t2 := mcp.NewInMemoryTransports()
	ss, err := srv.Connect(ctx, t1, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ss.Close() })
	cs, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "execute_query",
		Arguments: map[string]any{"query": "up"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("tool error: %+v", res)
	}
	m, ok := res.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("structured content type %T", res.StructuredContent)
	}
	if m["resultType"] != "vector" {
		t.Fatalf("resultType %#v", m["resultType"])
	}
	links, ok := m["links"].([]any)
	if !ok || len(links) == 0 {
		t.Fatalf("missing links: %#v", m["links"])
	}
	link := links[0].(map[string]any)
	if link["rel"] != "prometheus-ui" {
		t.Fatalf("link rel %#v", link["rel"])
	}
}
