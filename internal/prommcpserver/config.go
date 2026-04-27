package prommcpserver

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds runtime configuration loaded from the environment.
type Config struct {
	PrometheusURL string
	// MetricsURLSource records which env var populated PrometheusURL (PROMETHEUS_URL, VICTORIAMETRICS_URL, or VM_SELECT_URL).
	MetricsURLSource       string
	URLSSLVerify           bool
	DisablePrometheusLinks bool
	Username               string
	Password               string
	Token                  string
	OrgID                  string
	ClientCertPath         string
	ClientKeyPath          string
	CustomHeaders          map[string]string
	RequestTimeoutSec      int
	ToolPrefix             string
	RequestsCABundle       string
	MCPTransport           string
	MCPBindHost            string
	MCPBindPort            int
	// VMAlertURL is the base URL for vmalert's /api/v1/rules and /api/v1/alerts when not served under the Prometheus prefix.
	// If empty and PrometheusURL contains "/select/", the client defaults to {PrometheusURL}/vmalert (VictoriaMetrics cluster layout).
	VMAlertURL string
	// ExportMaxBytes caps /api/v1/export and similar raw responses (default 2MiB).
	ExportMaxBytes int64
}

func parseBoolEnv(key string, defaultVal bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return defaultVal
	}
	switch strings.ToLower(v) {
	case "true", "1", "yes":
		return true
	case "false", "0", "no":
		return false
	default:
		return defaultVal
	}
}

// LoadConfig reads configuration from the environment.
func LoadConfig() (*Config, error) {
	customHeadersRaw := os.Getenv("PROMETHEUS_CUSTOM_HEADERS")
	var customHeaders map[string]string
	if strings.TrimSpace(customHeadersRaw) != "" {
		if err := json.Unmarshal([]byte(customHeadersRaw), &customHeaders); err != nil {
			return nil, fmt.Errorf("PROMETHEUS_CUSTOM_HEADERS: %w", err)
		}
	}

	timeout := 30
	if v := os.Getenv("PROMETHEUS_REQUEST_TIMEOUT"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("PROMETHEUS_REQUEST_TIMEOUT: %w", err)
		}
		timeout = n
	}

	port := 8052
	if v := os.Getenv("PROMETHEUS_MCP_BIND_PORT"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("PROMETHEUS_MCP_BIND_PORT: %w", err)
		}
		port = n
	}

	promURL := strings.TrimSpace(os.Getenv("PROMETHEUS_URL"))
	urlSource := "PROMETHEUS_URL"
	if promURL == "" {
		promURL = strings.TrimSpace(os.Getenv("VICTORIAMETRICS_URL"))
		urlSource = "VICTORIAMETRICS_URL"
	}
	if promURL == "" {
		promURL = strings.TrimSpace(os.Getenv("VM_SELECT_URL"))
		urlSource = "VM_SELECT_URL"
	}
	if promURL == "" {
		urlSource = ""
	}

	cfg := &Config{
		PrometheusURL:          promURL,
		MetricsURLSource:       urlSource,
		URLSSLVerify:           parseBoolEnv("PROMETHEUS_URL_SSL_VERIFY", true),
		DisablePrometheusLinks: parseBoolEnv("PROMETHEUS_DISABLE_LINKS", false),
		Username:               os.Getenv("PROMETHEUS_USERNAME"),
		Password:               os.Getenv("PROMETHEUS_PASSWORD"),
		Token:                  os.Getenv("PROMETHEUS_TOKEN"),
		OrgID:                  os.Getenv("ORG_ID"),
		ClientCertPath:         strings.TrimSpace(os.Getenv("PROMETHEUS_CLIENT_CERT")),
		ClientKeyPath:          strings.TrimSpace(os.Getenv("PROMETHEUS_CLIENT_KEY")),
		CustomHeaders:          customHeaders,
		RequestTimeoutSec:      timeout,
		ToolPrefix:             strings.TrimSpace(os.Getenv("TOOL_PREFIX")),
		RequestsCABundle:       strings.TrimSpace(os.Getenv("REQUESTS_CA_BUNDLE")),
		MCPTransport:           strings.ToLower(strings.TrimSpace(os.Getenv("PROMETHEUS_MCP_SERVER_TRANSPORT"))),
		MCPBindHost:            strings.TrimSpace(os.Getenv("PROMETHEUS_MCP_BIND_HOST")),
		MCPBindPort:            port,
	}

	if cfg.MCPTransport == "" {
		cfg.MCPTransport = "stdio"
	}
	if cfg.MCPBindHost == "" {
		cfg.MCPBindHost = "127.0.0.1"
	}

	exportMax := int64(2097152)
	if v := strings.TrimSpace(os.Getenv("PROMETHEUS_EXPORT_MAX_BYTES")); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("PROMETHEUS_EXPORT_MAX_BYTES: %w", err)
		}
		if n <= 0 {
			return nil, fmt.Errorf("PROMETHEUS_EXPORT_MAX_BYTES must be positive")
		}
		exportMax = n
	}

	cfg.VMAlertURL = strings.TrimSpace(os.Getenv("VMALERT_URL"))
	cfg.ExportMaxBytes = exportMax

	return cfg, nil
}

// Validate checks required fields and transport settings before serving.
func (c *Config) Validate() error {
	if c.PrometheusURL == "" {
		return fmt.Errorf("set PROMETHEUS_URL or VICTORIAMETRICS_URL (VictoriaMetrics vmselect Prometheus API base URL)")
	}
	switch c.MCPTransport {
	case "stdio", "http", "sse":
	default:
		return fmt.Errorf("PROMETHEUS_MCP_SERVER_TRANSPORT must be stdio, http, or sse")
	}
	if c.MCPTransport == "http" || c.MCPTransport == "sse" {
		if c.MCPBindPort <= 0 {
			return fmt.Errorf("PROMETHEUS_MCP_BIND_PORT must be a positive integer")
		}
	}
	return nil
}

func (c *Config) ServerName() string {
	if c.ToolPrefix != "" {
		return "Prometheus MCP (" + c.ToolPrefix + ")"
	}
	return "Prometheus MCP"
}

func (c *Config) ToolName(base string) string {
	if c.ToolPrefix != "" {
		return c.ToolPrefix + "_" + base
	}
	return base
}

// AdminBaseFromMetricsURL returns the vmselect (or single-node) host root used for /flags and similar endpoints.
// For URLs like http://vm:8481/select/0/prometheus it returns http://vm:8481.
func AdminBaseFromMetricsURL(metricsBase string) string {
	metricsBase = strings.TrimRight(metricsBase, "/")
	if i := strings.Index(metricsBase, "/select/"); i >= 0 {
		return metricsBase[:i]
	}
	return metricsBase
}

// VMAlertAPIPrefix returns the base path for vmalert HTTP API (/api/v1/rules, /api/v1/alerts).
func (c *Config) VMAlertAPIPrefix() string {
	if v := strings.TrimSpace(c.VMAlertURL); v != "" {
		return strings.TrimRight(v, "/")
	}
	base := strings.TrimRight(c.PrometheusURL, "/")
	if strings.Contains(base, "/select/") {
		return base + "/vmalert"
	}
	return base
}
