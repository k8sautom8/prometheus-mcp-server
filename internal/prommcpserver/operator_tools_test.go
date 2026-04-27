package prommcpserver

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidateDuration(t *testing.T) {
	for _, d := range []string{"3h", "24h", "30m", "45m", "168h", "15m"} {
		if err := validateDuration(d); err != nil {
			t.Fatalf("%s: %v", d, err)
		}
	}
	if err := validateDuration("not-a-duration"); err == nil {
		t.Fatal("want error")
	}
}

func TestOperatorHostResourceReportNodeExporter(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/query" {
			http.NotFound(w, r)
			return
		}
		q := r.URL.Query().Get("query")
		var payload map[string]any
		switch {
		case strings.Contains(q, "count(") && strings.Contains(q, "node_cpu_seconds_total"):
			payload = map[string]any{
				"status": "success",
				"data": map[string]any{
					"resultType": "scalar",
					"result":     []any{1.0, "1"},
				},
			}
		case strings.Contains(q, "max_over_time") && strings.Contains(q, "node_cpu_seconds_total"):
			payload = map[string]any{
				"status": "success",
				"data": map[string]any{
					"resultType": "vector",
					"result": []any{
						map[string]any{
							"metric": map[string]any{"instance": "10.0.0.1:9100"},
							"value":  []any{1.0, "91.5"},
						},
						map[string]any{
							"metric": map[string]any{"instance": "10.0.0.2:9100"},
							"value":  []any{1.0, "42.0"},
						},
					},
				},
			}
		case strings.Contains(q, "node_memory_MemAvailable_bytes"):
			payload = map[string]any{
				"status": "success",
				"data": map[string]any{
					"resultType": "vector",
					"result": []any{
						map[string]any{
							"metric": map[string]any{"instance": "10.0.0.1:9100"},
							"value":  []any{1.0, "88.2"},
						},
					},
				},
			}
		default:
			http.Error(w, "unexpected query: "+q, http.StatusBadRequest)
			return
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
	th := 80.0
	_, out, err := deps.operatorHostResourceReport(t.Context(), nil, operatorHostResourceReportIn{
		Lookback:            "3h",
		MetricProfile:       "node_exporter",
		CPUPercentThreshold: &th,
		MemPercentThreshold: &th,
	})
	if err != nil {
		t.Fatal(err)
	}
	m, ok := out.(map[string]any)
	if !ok {
		t.Fatalf("type %T", out)
	}
	if m["metric_profile_used"] != "node_exporter" {
		t.Fatalf("profile %#v", m["metric_profile_used"])
	}
	breaches, ok := m["cpu_breaches"].([]map[string]any)
	if !ok || len(breaches) != 1 {
		t.Fatalf("cpu_breaches %#v", m["cpu_breaches"])
	}
	mb, ok := m["memory_breaches"].([]map[string]any)
	if !ok || len(mb) != 1 {
		t.Fatalf("memory_breaches %#v", m["memory_breaches"])
	}
}

func TestOperatorHostResourceReportWindowsDiskNetwork(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/query" {
			http.NotFound(w, r)
			return
		}
		payload := map[string]any{
			"status": "success",
			"data": map[string]any{
				"resultType": "vector",
				"result": []any{
					map[string]any{
						"metric": map[string]any{"instance": "win:9182"},
						"value":  []any{1.0, "12.3"},
					},
				},
			},
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
	disk, net := true, true
	mbps := 100.0
	_, out, err := deps.operatorHostResourceReport(t.Context(), nil, operatorHostResourceReportIn{
		Lookback:                         "3h",
		MetricProfile:                    "windows_exporter",
		IncludeDiskIO:                    &disk,
		IncludeNetwork:                   &net,
		WindowsNetworkTotalMbpsThreshold: &mbps,
	})
	if err != nil {
		t.Fatal(err)
	}
	m, ok := out.(map[string]any)
	if !ok {
		t.Fatalf("type %T", out)
	}
	pq, _ := m["promql_disk_busy_peak_percent"].(string)
	if !strings.Contains(pq, "windows_physical_disk_idle_seconds_total") {
		t.Fatalf("disk promql: %s", pq)
	}
	pqn, _ := m["promql_network_errors_peak_per_sec"].(string)
	if !strings.Contains(pqn, "windows_net_packets_received_errors_total") {
		t.Fatalf("net errors promql: %s", pqn)
	}
}
