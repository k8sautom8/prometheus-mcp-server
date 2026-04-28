package prommcpserver

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOperatorProcessExporterReport(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/query" {
			http.NotFound(w, r)
			return
		}
		q := r.URL.Query().Get("query")
		var payload map[string]any
		switch {
		case strings.Contains(q, "count(") && strings.Contains(q, "namedprocess_namegroup_cpu_seconds_total"):
			payload = map[string]any{
				"status": "success",
				"data": map[string]any{
					"resultType": "scalar",
					"result":     []any{1.0, "1"},
				},
			}
		case strings.Contains(q, "max_over_time") && strings.Contains(q, "namedprocess_namegroup_cpu_seconds_total"):
			payload = map[string]any{
				"status": "success",
				"data": map[string]any{
					"resultType": "vector",
					"result": []any{
						map[string]any{
							"metric": map[string]any{"instance": "10.0.0.1:9256", "groupname": "java"},
							"value":  []any{1.0, "2.5"},
						},
					},
				},
			}
		default:
			payload = map[string]any{
				"status": "success",
				"data": map[string]any{
					"resultType": "vector",
					"result":     []any{},
				},
			}
		}
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer ts.Close()

	cfg := &Config{
		PrometheusURL:     ts.URL,
		MCPTransport:      "stdio",
		RequestTimeoutSec: 5,
		ExportMaxBytes:    1024,
	}
	prom, err := NewPrometheusClient(cfg, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	deps := &Deps{Cfg: cfg, Prom: prom, Log: slog.New(slog.DiscardHandler), Metrics: &MetricsCache{}}
	cpu := true
	th := 1.0
	_, out, err := deps.operatorProcessExporterReport(t.Context(), nil, operatorProcessExporterReportIn{
		Lookback:                  "3h",
		IncludeCPUPerGroup:        &cpu,
		CPUSecondsPerSecThreshold: &th,
		LabelSelector:             `job="process-exporter"`,
	})
	if err != nil {
		t.Fatal(err)
	}
	m, ok := out.(map[string]any)
	if !ok {
		t.Fatalf("type %T", out)
	}
	if m["error"] != nil {
		t.Fatalf("unexpected error: %#v", m["error"])
	}
	b, ok := m["process_group_cpu_breaches"].([]map[string]any)
	if !ok || len(b) != 1 {
		t.Fatalf("breaches %#v", m["process_group_cpu_breaches"])
	}
}

func TestProcessExporterCPUPeakExpr(t *testing.T) {
	q := processExporterCPUPeakExpr(`,job="pe"`, "5m", "3h", "5m")
	if !strings.Contains(q, `namedprocess_namegroup_cpu_seconds_total{job="pe"}`) {
		t.Fatal(q)
	}
	if !strings.Contains(q, "max_over_time") {
		t.Fatal(q)
	}
}
