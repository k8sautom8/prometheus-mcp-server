package prommcpserver

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Deps carries shared state for MCP tool handlers.
type Deps struct {
	Cfg     *Config
	Prom    *PrometheusClient
	Log     *slog.Logger
	Metrics *MetricsCache
}

func toolAnnotations(title string) *mcp.ToolAnnotations {
	destructive := false
	openWorld := true
	return &mcp.ToolAnnotations{
		Title:           title,
		ReadOnlyHint:    true,
		DestructiveHint: &destructive,
		IdempotentHint:  true,
		OpenWorldHint:   &openWorld,
	}
}

func (d *Deps) notifyProgress(ctx context.Context, req *mcp.CallToolRequest, progress, total float64, message string) {
	if req == nil || req.Session == nil {
		return
	}
	token := req.Params.GetProgressToken()
	if token == nil {
		return
	}
	_ = req.Session.NotifyProgress(ctx, &mcp.ProgressNotificationParams{
		ProgressToken: token,
		Progress:      progress,
		Total:         total,
		Message:       message,
	})
}

func prometheusGraphLink(base string, v url.Values) []any {
	return []any{
		map[string]any{
			"href":  base + "/graph?" + v.Encode(),
			"rel":   "prometheus-ui",
			"title": "View in Prometheus UI",
		},
	}
}

// RegisterTools registers all Prometheus MCP tools on the server.
func (d *Deps) RegisterTools(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        d.Cfg.ToolName("health_check"),
		Description: "Health check endpoint for container monitoring and status verification",
		Title:       "Health Check",
		Annotations: toolAnnotations("Health Check"),
	}, d.healthCheck)

	mcp.AddTool(s, &mcp.Tool{
		Name:        d.Cfg.ToolName("execute_query"),
		Description: "Execute a PromQL instant query against Prometheus",
		Title:       "Execute PromQL Query",
		Annotations: toolAnnotations("Execute PromQL Query"),
	}, d.executeQuery)

	mcp.AddTool(s, &mcp.Tool{
		Name:        d.Cfg.ToolName("execute_range_query"),
		Description: "Execute a PromQL range query with start time, end time, and step interval",
		Title:       "Execute PromQL Range Query",
		Annotations: toolAnnotations("Execute PromQL Range Query"),
	}, d.executeRangeQuery)

	mcp.AddTool(s, &mcp.Tool{
		Name:        d.Cfg.ToolName("list_metrics"),
		Description: "List all available metrics in Prometheus with optional pagination support",
		Title:       "List Available Metrics",
		Annotations: toolAnnotations("List Available Metrics"),
	}, d.listMetrics)

	mcp.AddTool(s, &mcp.Tool{
		Name: d.Cfg.ToolName("get_metric_metadata"),
		Description: "Get metadata (type, help, unit) for metrics. " +
			"Returns all metric metadata when no metric name is provided. " +
			"Use filter_pattern to search metric names and descriptions.",
		Title:       "Get Metric Metadata",
		Annotations: toolAnnotations("Get Metric Metadata"),
	}, d.getMetricMetadata)

	mcp.AddTool(s, &mcp.Tool{
		Name:        d.Cfg.ToolName("get_targets"),
		Description: "Get information about all scrape targets",
		Title:       "Get Scrape Targets",
		Annotations: toolAnnotations("Get Scrape Targets"),
	}, d.getTargets)

	d.registerExtendedTools(s)
}

type healthCheckIn struct{}

func (d *Deps) healthCheck(ctx context.Context, req *mcp.CallToolRequest, _ healthCheckIn) (*mcp.CallToolResult, map[string]any, error) {
	ts := time.Now().UTC().Format(time.RFC3339Nano)
	out := map[string]any{
		"status":    "healthy",
		"service":   "prometheus-mcp-server",
		"version":   Version,
		"timestamp": ts,
		"transport": d.Cfg.MCPTransport,
		"configuration": map[string]any{
			"prometheus_url_configured": d.Cfg.PrometheusURL != "",
			"metrics_url_env":           d.Cfg.MetricsURLSource,
			"authentication_configured": (d.Cfg.Username != "" && d.Cfg.Password != "") || d.Cfg.Token != "" || d.Cfg.ClientCertPath != "",
			"org_id_configured":         d.Cfg.OrgID != "",
		},
	}
	if d.Cfg.PrometheusURL == "" {
		out["status"] = "unhealthy"
		out["error"] = "PROMETHEUS_URL not configured"
		return nil, out, nil
	}
	q := url.Values{}
	q.Set("query", "up")
	q.Set("time", strconv.FormatInt(time.Now().Unix(), 10))
	_, err := d.Prom.MakePrometheusRequest("query", q)
	if err != nil {
		out["prometheus_connectivity"] = "unhealthy"
		out["prometheus_error"] = err.Error()
		out["status"] = "degraded"
		return nil, out, nil
	}
	out["prometheus_connectivity"] = "healthy"
	out["prometheus_url"] = d.Cfg.PrometheusURL
	d.Log.Info("health check completed", "status", out["status"])
	return nil, out, nil
}

type executeQueryIn struct {
	Query string  `json:"query"`
	Time  *string `json:"time,omitempty"`
}

func (d *Deps) executeQuery(_ context.Context, _ *mcp.CallToolRequest, in executeQueryIn) (*mcp.CallToolResult, map[string]any, error) {
	q := url.Values{}
	q.Set("query", in.Query)
	if in.Time != nil && *in.Time != "" {
		q.Set("time", *in.Time)
	}
	d.Log.Info("executing instant query", "query", in.Query)
	data, err := d.Prom.MakePrometheusRequest("query", q)
	if err != nil {
		return nil, nil, err
	}
	m, ok := data.(map[string]any)
	if !ok {
		return nil, nil, fmt.Errorf("unexpected query response shape")
	}
	result := map[string]any{
		"resultType": m["resultType"],
		"result":     m["result"],
	}
	if !d.Cfg.DisablePrometheusLinks {
		ui := url.Values{}
		ui.Set("g0.expr", in.Query)
		ui.Set("g0.tab", "0")
		if in.Time != nil && *in.Time != "" {
			ui.Set("g0.moment_input", *in.Time)
		}
		result["links"] = prometheusGraphLink(d.Prom.BaseURL(), ui)
	}
	d.Log.Info("instant query completed", "query", in.Query)
	return nil, result, nil
}

type executeRangeQueryIn struct {
	Query string `json:"query"`
	Start string `json:"start"`
	End   string `json:"end"`
	Step  string `json:"step"`
}

func (d *Deps) executeRangeQuery(ctx context.Context, req *mcp.CallToolRequest, in executeRangeQueryIn) (*mcp.CallToolResult, map[string]any, error) {
	q := url.Values{}
	q.Set("query", in.Query)
	q.Set("start", in.Start)
	q.Set("end", in.End)
	q.Set("step", in.Step)
	d.Log.Info("executing range query", "query", in.Query)
	d.notifyProgress(ctx, req, 0, 100, "Initiating range query...")
	data, err := d.Prom.MakePrometheusRequest("query_range", q)
	if err != nil {
		return nil, nil, err
	}
	d.notifyProgress(ctx, req, 50, 100, "Processing query results...")
	m, ok := data.(map[string]any)
	if !ok {
		return nil, nil, fmt.Errorf("unexpected range query response shape")
	}
	result := map[string]any{
		"resultType": m["resultType"],
		"result":     m["result"],
	}
	if !d.Cfg.DisablePrometheusLinks {
		ui := url.Values{}
		ui.Set("g0.expr", in.Query)
		ui.Set("g0.tab", "0")
		ui.Set("g0.range_input", in.Start+" to "+in.End)
		ui.Set("g0.step_input", in.Step)
		result["links"] = prometheusGraphLink(d.Prom.BaseURL(), ui)
	}
	d.notifyProgress(ctx, req, 100, 100, "Range query completed")
	return nil, result, nil
}

type listMetricsIn struct {
	Limit          *int    `json:"limit,omitempty"`
	Offset         int     `json:"offset,omitempty"`
	FilterPattern  *string `json:"filter_pattern,omitempty"`
	RefreshCache   bool    `json:"refresh_cache,omitempty"`
}

func (d *Deps) listMetrics(ctx context.Context, req *mcp.CallToolRequest, in listMetricsIn) (*mcp.CallToolResult, map[string]any, error) {
	d.Log.Info("listing metrics", "limit", in.Limit, "offset", in.Offset, "refresh_cache", in.RefreshCache)
	d.notifyProgress(ctx, req, 0, 100, "Fetching metrics list...")
	if in.RefreshCache {
		d.Metrics.clear()
	}
	data, err := d.Metrics.get(func() ([]string, error) {
		raw, err := d.Prom.MakePrometheusRequest("label/__name__/values", nil)
		if err != nil {
			return nil, err
		}
		arr, ok := raw.([]any)
		if !ok {
			return nil, fmt.Errorf("unexpected metrics list response")
		}
		names := make([]string, 0, len(arr))
		for _, x := range arr {
			if s, ok := x.(string); ok {
				names = append(names, s)
			}
		}
		return names, nil
	})
	if err != nil {
		return nil, nil, err
	}
	d.notifyProgress(ctx, req, 50, 100, fmt.Sprintf("Processing %d metrics...", len(data)))
	if in.FilterPattern != nil && *in.FilterPattern != "" {
		fp := *in.FilterPattern
		filtered := make([]string, 0)
		for _, m := range data {
			if containsFold(m, fp) {
				filtered = append(filtered, m)
			}
		}
		data = filtered
	}
	total := len(data)
	start := in.Offset
	if start > total {
		start = total
	}
	end := total
	if in.Limit != nil {
		end = start + *in.Limit
		if end > total {
			end = total
		}
	}
	page := append([]string(nil), data[start:end]...)
	out := map[string]any{
		"metrics":        page,
		"total_count":    total,
		"returned_count": len(page),
		"offset":         in.Offset,
		"has_more":       end < total,
	}
	d.notifyProgress(ctx, req, 100, 100, fmt.Sprintf("Retrieved %d of %d metrics", len(page), total))
	return nil, out, nil
}

type getMetricMetadataIn struct {
	Metric         *string `json:"metric,omitempty"`
	FilterPattern  *string `json:"filter_pattern,omitempty"`
	Limit          *int    `json:"limit,omitempty"`
	Offset         int     `json:"offset,omitempty"`
}

func (d *Deps) getMetricMetadata(_ context.Context, _ *mcp.CallToolRequest, in getMetricMetadataIn) (*mcp.CallToolResult, any, error) {
	params := url.Values{}
	if in.Metric != nil && *in.Metric != "" {
		params.Set("metric", *in.Metric)
	}
	d.Log.Info("get metric metadata", "metric", in.Metric)
	rawData, err := d.Prom.MakePrometheusRequest("metadata", params)
	if err != nil {
		return nil, nil, err
	}
	metricName := ""
	if in.Metric != nil {
		metricName = *in.Metric
	}
	metadataByMetric := normalizeMetadataMap(rawData)
	if metricName != "" {
		if _, ok := metadataByMetric[metricName]; !ok {
			fb := coerceMetadataEntries(rawData)
			if len(fb) > 0 {
				metadataByMetric[metricName] = fb
			}
		}
		entries := metadataByMetric[metricName]
		if entries == nil {
			entries = []map[string]any{}
		}
		return nil, entries, nil
	}
	if in.FilterPattern != nil && *in.FilterPattern != "" {
		fp := *in.FilterPattern
		filtered := make(map[string][]map[string]any)
		for name, entries := range metadataByMetric {
			if metadataMatchesPattern(name, entries, fp) {
				filtered[name] = entries
			}
		}
		metadataByMetric = filtered
	}
	names := sortedMetricNames(metadataByMetric)
	total := len(names)
	start := in.Offset
	if start > total {
		start = total
	}
	end := total
	if in.Limit != nil {
		end = start + *in.Limit
		if end > total {
			end = total
		}
	}
	page := make(map[string][]map[string]any)
	for _, n := range names[start:end] {
		page[n] = metadataByMetric[n]
	}
	out := map[string]any{
		"metadata":        page,
		"total_count":     total,
		"returned_count":  len(page),
		"offset":          in.Offset,
		"has_more":        end < total,
	}
	return nil, out, nil
}

type getTargetsIn struct{}

func (d *Deps) getTargets(_ context.Context, _ *mcp.CallToolRequest, _ getTargetsIn) (*mcp.CallToolResult, map[string]any, error) {
	d.Log.Info("retrieving scrape targets")
	data, err := d.Prom.MakePrometheusRequest("targets", nil)
	if err != nil {
		return nil, nil, err
	}
	m, ok := data.(map[string]any)
	if !ok {
		return nil, nil, fmt.Errorf("unexpected targets response shape")
	}
	out := map[string]any{
		"activeTargets":  m["activeTargets"],
		"droppedTargets": m["droppedTargets"],
	}
	return nil, out, nil
}
