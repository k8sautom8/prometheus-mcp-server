package prommcpserver

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// PrometheusClient performs HTTP requests against the Prometheus API.
type PrometheusClient struct {
	cfg    *Config
	client *http.Client
	log    *slog.Logger
}

// NewPrometheusClient builds an HTTP client from configuration (TLS, auth, timeouts).
func NewPrometheusClient(cfg *Config, log *slog.Logger) (*PrometheusClient, error) {
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}
	if !cfg.URLSSLVerify {
		tlsConfig.InsecureSkipVerify = true
		log.Warn("SSL certificate verification is disabled; do not use in production", "component", "prometheus")
	}

	if cfg.RequestsCABundle != "" {
		pool, err := x509.SystemCertPool()
		if err != nil {
			pool = x509.NewCertPool()
		}
		pemData, err := os.ReadFile(cfg.RequestsCABundle)
		if err != nil {
			return nil, fmt.Errorf("read REQUESTS_CA_BUNDLE: %w", err)
		}
		if !pool.AppendCertsFromPEM(pemData) {
			return nil, fmt.Errorf("no certificates parsed from REQUESTS_CA_BUNDLE")
		}
		tlsConfig.RootCAs = pool
	}

	tr := &http.Transport{TLSClientConfig: tlsConfig}
	if cfg.ClientCertPath != "" {
		var cert tls.Certificate
		var err error
		if cfg.ClientKeyPath != "" {
			cert, err = tls.LoadX509KeyPair(cfg.ClientCertPath, cfg.ClientKeyPath)
		} else {
			cert, err = tls.LoadX509KeyPair(cfg.ClientCertPath, cfg.ClientCertPath)
		}
		if err != nil {
			return nil, fmt.Errorf("load client certificate: %w", err)
		}
		tr.TLSClientConfig.Certificates = []tls.Certificate{cert}
	}

	return &PrometheusClient{
		cfg: cfg,
		client: &http.Client{
			Transport: tr,
			Timeout:   time.Duration(cfg.RequestTimeoutSec) * time.Second,
		},
		log: log,
	}, nil
}

// BaseURL returns the configured Prometheus URL without a trailing slash.
func (p *PrometheusClient) BaseURL() string {
	return strings.TrimRight(p.cfg.PrometheusURL, "/")
}

func (p *PrometheusClient) buildHeaders() http.Header {
	h := make(http.Header)
	if p.cfg.Token != "" {
		h.Set("Authorization", "Bearer "+p.cfg.Token)
	}
	if p.cfg.OrgID != "" {
		h.Set("X-Scope-OrgID", p.cfg.OrgID)
	}
	for k, v := range p.cfg.CustomHeaders {
		h.Set(k, v)
	}
	return h
}

// MakePrometheusRequest calls Prometheus /api/v1/{endpoint} and returns the "data" field on success.
func (p *PrometheusClient) MakePrometheusRequest(endpoint string, params url.Values) (any, error) {
	if p.cfg.PrometheusURL == "" {
		p.log.Error("Prometheus configuration missing", "error", "PROMETHEUS_URL not set")
		return nil, fmt.Errorf("prometheus configuration is missing; set PROMETHEUS_URL")
	}
	u, err := url.Parse(p.BaseURL() + "/api/v1/" + endpoint)
	if err != nil {
		return nil, err
	}
	if len(params) > 0 {
		u.RawQuery = params.Encode()
	}
	return p.getPrometheusEnvelope(u, endpoint)
}

// MakeVMAlertRequest calls vmalert /api/v1/{endpoint} (see Config.VMAlertAPIPrefix).
func (p *PrometheusClient) MakeVMAlertRequest(endpoint string, params url.Values) (any, error) {
	if p.cfg.PrometheusURL == "" {
		return nil, fmt.Errorf("prometheus configuration is missing; set PROMETHEUS_URL")
	}
	prefix := p.cfg.VMAlertAPIPrefix()
	u, err := url.Parse(prefix + "/api/v1/" + endpoint)
	if err != nil {
		return nil, err
	}
	if len(params) > 0 {
		u.RawQuery = params.Encode()
	}
	return p.getPrometheusEnvelope(u, "vmalert/"+endpoint)
}

func (p *PrometheusClient) getPrometheusEnvelope(u *url.URL, logLabel string) (any, error) {
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header = p.buildHeaders()
	if p.cfg.Username != "" && p.cfg.Password != "" && p.cfg.Token == "" {
		req.SetBasicAuth(p.cfg.Username, p.cfg.Password)
	}

	p.log.Debug("Prometheus API request", "endpoint", logLabel, "url", u.String())

	resp, err := p.client.Do(req)
	if err != nil {
		p.log.Error("HTTP request to Prometheus failed", "endpoint", logLabel, "error", err)
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		p.log.Error("Prometheus HTTP error", "endpoint", logLabel, "status", resp.StatusCode, "body", string(body))
		return nil, fmt.Errorf("prometheus HTTP %d", resp.StatusCode)
	}

	var envelope struct {
		Status string          `json:"status"`
		Data   json.RawMessage `json:"data"`
		Error  string          `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		p.log.Error("invalid JSON from Prometheus", "endpoint", logLabel, "error", err)
		return nil, fmt.Errorf("invalid JSON response from Prometheus: %w", err)
	}
	if envelope.Status != "success" {
		msg := envelope.Error
		if msg == "" {
			msg = "unknown error"
		}
		p.log.Error("Prometheus API error", "endpoint", logLabel, "error", msg)
		return nil, fmt.Errorf("prometheus API error: %s", msg)
	}

	var data any
	if len(envelope.Data) == 0 {
		return nil, nil
	}
	if err := json.Unmarshal(envelope.Data, &data); err != nil {
		return nil, fmt.Errorf("invalid JSON data from Prometheus: %w", err)
	}

	switch v := data.(type) {
	case map[string]any:
		rt, _ := v["resultType"].(string)
		p.log.Debug("Prometheus API success", "endpoint", logLabel, "resultType", rt)
	default:
		p.log.Debug("Prometheus API success", "endpoint", logLabel, "resultType", "list")
	}

	return data, nil
}

// GetAPIv1Raw performs GET /api/v1/{apiPath} and returns the raw body (for export NDJSON/CSV).
func (p *PrometheusClient) GetAPIv1Raw(apiPath string, params url.Values) ([]byte, error) {
	if p.cfg.PrometheusURL == "" {
		return nil, fmt.Errorf("prometheus configuration is missing; set PROMETHEUS_URL")
	}
	u, err := url.Parse(p.BaseURL() + "/api/v1/" + apiPath)
	if err != nil {
		return nil, err
	}
	if len(params) > 0 {
		u.RawQuery = params.Encode()
	}
	return p.getRawLimited(u, "api/v1/"+apiPath, p.cfg.ExportMaxBytes)
}

// GetUnderMetricsRootRaw GETs {PrometheusURL}/{relPath} (e.g. prettify-query, *-debug helpers on VictoriaMetrics).
func (p *PrometheusClient) GetUnderMetricsRootRaw(relPath string, params url.Values) ([]byte, error) {
	if p.cfg.PrometheusURL == "" {
		return nil, fmt.Errorf("prometheus configuration is missing; set PROMETHEUS_URL")
	}
	u, err := url.Parse(p.BaseURL() + "/" + strings.TrimLeft(relPath, "/"))
	if err != nil {
		return nil, err
	}
	if len(params) > 0 {
		u.RawQuery = params.Encode()
	}
	return p.getRawLimited(u, relPath, p.cfg.ExportMaxBytes)
}

// GetUnderAdminRootRaw GETs {adminBase}/{relPath} (e.g. flags on vmselect root).
func (p *PrometheusClient) GetUnderAdminRootRaw(relPath string, params url.Values) ([]byte, error) {
	if p.cfg.PrometheusURL == "" {
		return nil, fmt.Errorf("prometheus configuration is missing; set PROMETHEUS_URL")
	}
	admin := AdminBaseFromMetricsURL(p.cfg.PrometheusURL)
	u, err := url.Parse(admin + "/" + strings.TrimLeft(relPath, "/"))
	if err != nil {
		return nil, err
	}
	if len(params) > 0 {
		u.RawQuery = params.Encode()
	}
	return p.getRawLimited(u, "admin/"+relPath, p.cfg.ExportMaxBytes)
}

func (p *PrometheusClient) getRawLimited(u *url.URL, logLabel string, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		maxBytes = 2097152
	}
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header = p.buildHeaders()
	if p.cfg.Username != "" && p.cfg.Password != "" && p.cfg.Token == "" {
		req.SetBasicAuth(p.cfg.Username, p.cfg.Password)
	}
	p.log.Debug("raw HTTP GET", "label", logLabel, "url", u.String())
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	limited := io.LimitReader(resp.Body, maxBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > maxBytes {
		return nil, fmt.Errorf("response exceeded PROMETHEUS_EXPORT_MAX_BYTES (%d bytes)", maxBytes)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}
