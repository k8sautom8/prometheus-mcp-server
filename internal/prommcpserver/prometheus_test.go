package prommcpserver

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
)

func TestMakePrometheusRequestSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/query" {
			t.Fatalf("path %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "success",
			"data": map[string]any{
				"resultType": "vector",
				"result":     []any{},
			},
		})
	}))
	defer ts.Close()

	cfg := &Config{
		PrometheusURL:       ts.URL,
		URLSSLVerify:        true,
		RequestTimeoutSec:     10,
		Username:            "u",
		Password:            "p",
		OrgID:               "org1",
		CustomHeaders:       map[string]string{"X-Custom": "v"},
	}
	c, err := NewPrometheusClient(cfg, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	q := url.Values{"query": {"up"}}
	data, err := c.MakePrometheusRequest("query", q)
	if err != nil {
		t.Fatal(err)
	}
	m, ok := data.(map[string]any)
	if !ok || m["resultType"] != "vector" {
		t.Fatalf("got %#v", data)
	}
}

func TestMakePrometheusRequestBearerToken(t *testing.T) {
	var auth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": map[string]any{}})
	}))
	defer ts.Close()
	cfg := &Config{PrometheusURL: ts.URL, Token: "tok", RequestTimeoutSec: 5}
	c, err := NewPrometheusClient(cfg, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.MakePrometheusRequest("query", url.Values{"query": {"1"}})
	if err != nil {
		t.Fatal(err)
	}
	if auth != "Bearer tok" {
		t.Fatalf("auth %q", auth)
	}
}

func TestMakePrometheusRequestAPIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "error", "error": "bad query"})
	}))
	defer ts.Close()
	cfg := &Config{PrometheusURL: ts.URL, RequestTimeoutSec: 5}
	c, err := NewPrometheusClient(cfg, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.MakePrometheusRequest("query", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMakePrometheusRequestListData(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "success",
			"data": []any{
				map[string]any{"metric": map[string]any{}, "value": []any{1609459200.0, "1"}},
			},
		})
	}))
	defer ts.Close()
	cfg := &Config{PrometheusURL: ts.URL, RequestTimeoutSec: 5}
	c, err := NewPrometheusClient(cfg, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	data, err := c.MakePrometheusRequest("query", nil)
	if err != nil {
		t.Fatal(err)
	}
	arr, ok := data.([]any)
	if !ok || len(arr) != 1 {
		t.Fatalf("got %#v", data)
	}
}

func TestRequestsCABundle(t *testing.T) {
	// Skip if we cannot create a temp file; exercise error path for missing file
	cfg := &Config{
		PrometheusURL:   "https://example.com",
		RequestsCABundle: "/nonexistent-ca-bundle-xyz",
		RequestTimeoutSec: 5,
	}
	_, err := NewPrometheusClient(cfg, slog.New(slog.DiscardHandler))
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("got %v", err)
	}
}
