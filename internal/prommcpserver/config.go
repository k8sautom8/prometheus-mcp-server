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
	PrometheusURL            string
	URLSSLVerify             bool
	DisablePrometheusLinks   bool
	Username                 string
	Password                 string
	Token                    string
	OrgID                    string
	ClientCertPath           string
	ClientKeyPath            string
	CustomHeaders            map[string]string
	RequestTimeoutSec        int
	ToolPrefix               string
	RequestsCABundle         string
	MCPTransport             string
	MCPBindHost              string
	MCPBindPort              int
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

	cfg := &Config{
		PrometheusURL:          strings.TrimSpace(os.Getenv("PROMETHEUS_URL")),
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

	return cfg, nil
}

// Validate checks required fields and transport settings before serving.
func (c *Config) Validate() error {
	if c.PrometheusURL == "" {
		return fmt.Errorf("PROMETHEUS_URL is not set")
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
