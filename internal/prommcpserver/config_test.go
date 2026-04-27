package prommcpserver

import (
	"testing"
)

func TestConfigToolName(t *testing.T) {
	c := &Config{ToolPrefix: "stg"}
	if c.ToolName("execute_query") != "stg_execute_query" {
		t.Fatal()
	}
	c2 := &Config{}
	if c2.ToolName("execute_query") != "execute_query" {
		t.Fatal()
	}
}

func TestLoadConfigVictoriaMetricsURL(t *testing.T) {
	t.Setenv("PROMETHEUS_URL", "")
	t.Setenv("VICTORIAMETRICS_URL", "http://vmselect:8481/select/0/prometheus")
	t.Setenv("VM_SELECT_URL", "http://ignored:8481")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PrometheusURL != "http://vmselect:8481/select/0/prometheus" {
		t.Fatalf("url: %q", cfg.PrometheusURL)
	}
	if cfg.MetricsURLSource != "VICTORIAMETRICS_URL" {
		t.Fatalf("source: %q", cfg.MetricsURLSource)
	}
}

func TestLoadConfigPrometheusURLWins(t *testing.T) {
	t.Setenv("PROMETHEUS_URL", "http://prom:9090")
	t.Setenv("VICTORIAMETRICS_URL", "http://vmselect:8481/select/0/prometheus")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PrometheusURL != "http://prom:9090" || cfg.MetricsURLSource != "PROMETHEUS_URL" {
		t.Fatalf("got %q %q", cfg.PrometheusURL, cfg.MetricsURLSource)
	}
}

func TestLoadConfigVMSelectURLFallback(t *testing.T) {
	t.Setenv("PROMETHEUS_URL", "")
	t.Setenv("VICTORIAMETRICS_URL", "")
	t.Setenv("VM_SELECT_URL", "http://vm:8481/select/1/prometheus")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PrometheusURL != "http://vm:8481/select/1/prometheus" || cfg.MetricsURLSource != "VM_SELECT_URL" {
		t.Fatalf("got %q %q", cfg.PrometheusURL, cfg.MetricsURLSource)
	}
}

func TestAdminBaseFromMetricsURL(t *testing.T) {
	if got := AdminBaseFromMetricsURL("http://vm:8481/select/0/prometheus"); got != "http://vm:8481" {
		t.Fatalf("got %q", got)
	}
	if got := AdminBaseFromMetricsURL("http://vm:8428"); got != "http://vm:8428" {
		t.Fatalf("got %q", got)
	}
}

func TestConfigVMAlertAPIPrefix(t *testing.T) {
	c := &Config{PrometheusURL: "http://x/select/0/prometheus"}
	if p := c.VMAlertAPIPrefix(); p != "http://x/select/0/prometheus/vmalert" {
		t.Fatalf("got %q", p)
	}
	c2 := &Config{PrometheusURL: "http://prom:9090", VMAlertURL: "http://alert:8880"}
	if p := c2.VMAlertAPIPrefix(); p != "http://alert:8880" {
		t.Fatalf("got %q", p)
	}
}

func TestConfigValidate(t *testing.T) {
	if err := (&Config{PrometheusURL: "http://x", MCPTransport: "stdio"}).Validate(); err != nil {
		t.Fatal(err)
	}
	if err := (&Config{MCPTransport: "stdio"}).Validate(); err == nil {
		t.Fatal("want error")
	}
	if err := (&Config{PrometheusURL: "http://x", MCPTransport: "bad"}).Validate(); err == nil {
		t.Fatal("want error")
	}
	if err := (&Config{PrometheusURL: "http://x", MCPTransport: "http", MCPBindPort: 0}).Validate(); err == nil {
		t.Fatal("want error")
	}
}
