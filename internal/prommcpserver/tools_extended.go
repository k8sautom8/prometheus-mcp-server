package prommcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/VictoriaMetrics/metricsql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerExtendedTools adds Prometheus- and VictoriaMetrics-compatible tools aligned with mcp-victoriametrics (OSS paths).
func (d *Deps) registerExtendedTools(s *mcp.Server) {
	reg := func(name, title, desc string, handler any) {
		t := &mcp.Tool{
			Name:        d.Cfg.ToolName(name),
			Description: desc,
			Title:       title,
			Annotations: toolAnnotations(title),
		}
		switch h := handler.(type) {
		case func(context.Context, *mcp.CallToolRequest, seriesIn) (*mcp.CallToolResult, any, error):
			mcp.AddTool(s, t, h)
		case func(context.Context, *mcp.CallToolRequest, labelsIn) (*mcp.CallToolResult, any, error):
			mcp.AddTool(s, t, h)
		case func(context.Context, *mcp.CallToolRequest, labelValuesIn) (*mcp.CallToolResult, any, error):
			mcp.AddTool(s, t, h)
		case func(context.Context, *mcp.CallToolRequest, metricsIn) (*mcp.CallToolResult, any, error):
			mcp.AddTool(s, t, h)
		case func(context.Context, *mcp.CallToolRequest, rulesIn) (*mcp.CallToolResult, any, error):
			mcp.AddTool(s, t, h)
		case func(context.Context, *mcp.CallToolRequest, alertsIn) (*mcp.CallToolResult, any, error):
			mcp.AddTool(s, t, h)
		case func(context.Context, *mcp.CallToolRequest, exportIn) (*mcp.CallToolResult, any, error):
			mcp.AddTool(s, t, h)
		case func(context.Context, *mcp.CallToolRequest, tsdbStatusIn) (*mcp.CallToolResult, any, error):
			mcp.AddTool(s, t, h)
		case func(context.Context, *mcp.CallToolRequest, topQueriesIn) (*mcp.CallToolResult, any, error):
			mcp.AddTool(s, t, h)
		case func(context.Context, *mcp.CallToolRequest, metricStatisticsIn) (*mcp.CallToolResult, any, error):
			mcp.AddTool(s, t, h)
		case func(context.Context, *mcp.CallToolRequest, emptyIn) (*mcp.CallToolResult, any, error):
			mcp.AddTool(s, t, h)
		case func(context.Context, *mcp.CallToolRequest, prettifyIn) (*mcp.CallToolResult, any, error):
			mcp.AddTool(s, t, h)
		case func(context.Context, *mcp.CallToolRequest, explainIn) (*mcp.CallToolResult, any, error):
			mcp.AddTool(s, t, h)
		case func(context.Context, *mcp.CallToolRequest, metricRelabelDebugIn) (*mcp.CallToolResult, any, error):
			mcp.AddTool(s, t, h)
		case func(context.Context, *mcp.CallToolRequest, retentionFiltersDebugIn) (*mcp.CallToolResult, any, error):
			mcp.AddTool(s, t, h)
		case func(context.Context, *mcp.CallToolRequest, downsamplingFiltersDebugIn) (*mcp.CallToolResult, any, error):
			mcp.AddTool(s, t, h)
		default:
			panic(fmt.Sprintf("registerExtendedTools: unsupported handler type %T", handler))
		}
	}

	reg("series", "List time series", "List series via /api/v1/series (match[], start, end, limit).", d.series)
	reg("labels", "List label names", "Label names via /api/v1/labels.", d.labels)
	reg("label_values", "List label values", "Values for one label via /api/v1/label/{name}/values.", d.labelValues)
	reg("metrics", "List metric names", "Metric names via /api/v1/label/__name__/values with optional match/start/end/limit (VictoriaMetrics-style).", d.metrics)
	reg("rules", "List rule groups", "Alerting/recording rules via vmalert or /api/v1/rules.", d.rules)
	reg("alerts", "List alerts", "Active alerts via vmalert or /api/v1/alerts.", d.alerts)
	reg("export", "Export raw samples", "GET /api/v1/export or /api/v1/export/csv (raw body, size capped by PROMETHEUS_EXPORT_MAX_BYTES).", d.exportSeries)
	reg("tsdb_status", "TSDB cardinality stats", "GET /api/v1/status/tsdb (VictoriaMetrics/Prometheus-compatible).", d.tsdbStatus)
	reg("top_queries", "Top queries stats", "GET /api/v1/status/top_queries.", d.topQueries)
	reg("metric_statistics", "Metric name query stats", "GET /api/v1/status/metric_names_stats (VictoriaMetrics).", d.metricStatistics)
	reg("active_queries", "Active queries", "GET /api/v1/status/active_queries.", d.activeQueries)
	reg("get_build_info", "Build info", "GET /api/v1/status/buildinfo.", d.getBuildInfo)
	reg("get_runtime_info", "Runtime info", "GET /api/v1/status/runtimeinfo.", d.getRuntimeInfo)
	reg("get_config_yaml", "Prometheus config", "GET /api/v1/status/config (YAML in data; may be unsupported).", d.getConfigYAML)
	reg("get_flags", "Process / status flags", "GET /api/v1/status/flags, or vmselect /flags as raw text.", d.getFlags)
	reg("prettify_query", "Prettify query", "Format MetricsQL/PromQL via metricsql or HTTP prettify-query (VictoriaMetrics).", d.prettifyQuery)
	reg("explain_query", "Explain query structure", "Parse query with metricsql and return canonical form (see VictoriaMetrics MetricsQL docs for functions).", d.explainQuery)
	reg("documentation", "Query language documentation links", "Pointers to MetricsQL and Prometheus HTTP API documentation.", d.documentation)
	reg("metric_relabel_debug", "Debug metric relabel rules", "GET .../metric-relabel-debug (VictoriaMetrics).", d.metricRelabelDebug)
	reg("retention_filters_debug", "Debug retention filters", "GET .../retention-filters-debug (Enterprise/OSS where exposed).", d.retentionFiltersDebug)
	reg("downsampling_filters_debug", "Debug downsampling filters", "GET .../downsampling-filters-debug (Enterprise/OSS where exposed).", d.downsamplingFiltersDebug)
}

type seriesIn struct {
	Match string  `json:"match,omitempty"`
	Start *string `json:"start,omitempty"`
	End   *string `json:"end,omitempty"`
	Limit *int    `json:"limit,omitempty"`
}

func (d *Deps) series(_ context.Context, _ *mcp.CallToolRequest, in seriesIn) (*mcp.CallToolResult, any, error) {
	q := url.Values{}
	match := strings.TrimSpace(in.Match)
	if match == "" {
		match = `{__name__!=''}`
	}
	q.Add("match[]", match)
	if in.Start != nil && *in.Start != "" {
		q.Set("start", *in.Start)
	}
	if in.End != nil && *in.End != "" {
		q.Set("end", *in.End)
	}
	if in.Limit != nil && *in.Limit > 0 {
		q.Set("limit", strconv.Itoa(*in.Limit))
	}
	data, err := d.Prom.MakePrometheusRequest("series", q)
	if err != nil {
		return nil, nil, err
	}
	return nil, data, nil
}

type labelsIn struct {
	Match *string `json:"match,omitempty"`
	Start *string `json:"start,omitempty"`
	End   *string `json:"end,omitempty"`
	Limit *int    `json:"limit,omitempty"`
}

func (d *Deps) labels(_ context.Context, _ *mcp.CallToolRequest, in labelsIn) (*mcp.CallToolResult, any, error) {
	q := url.Values{}
	if in.Match != nil && *in.Match != "" {
		q.Add("match[]", *in.Match)
	}
	if in.Start != nil && *in.Start != "" {
		q.Set("start", *in.Start)
	}
	if in.End != nil && *in.End != "" {
		q.Set("end", *in.End)
	}
	if in.Limit != nil && *in.Limit > 0 {
		q.Set("limit", strconv.Itoa(*in.Limit))
	}
	data, err := d.Prom.MakePrometheusRequest("labels", q)
	if err != nil {
		return nil, nil, err
	}
	return nil, data, nil
}

type labelValuesIn struct {
	LabelName string  `json:"label_name"`
	Match     *string `json:"match,omitempty"`
	Start     *string `json:"start,omitempty"`
	End       *string `json:"end,omitempty"`
	Limit     *int    `json:"limit,omitempty"`
}

func (d *Deps) labelValues(_ context.Context, _ *mcp.CallToolRequest, in labelValuesIn) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.LabelName) == "" {
		return nil, nil, fmt.Errorf("label_name is required")
	}
	path := "label/" + url.PathEscape(in.LabelName) + "/values"
	q := url.Values{}
	if in.Match != nil && *in.Match != "" {
		q.Add("match[]", *in.Match)
	}
	if in.Start != nil && *in.Start != "" {
		q.Set("start", *in.Start)
	}
	if in.End != nil && *in.End != "" {
		q.Set("end", *in.End)
	}
	if in.Limit != nil && *in.Limit > 0 {
		q.Set("limit", strconv.Itoa(*in.Limit))
	}
	data, err := d.Prom.MakePrometheusRequest(path, q)
	if err != nil {
		return nil, nil, err
	}
	return nil, data, nil
}

type metricsIn struct {
	Match *string `json:"match,omitempty"`
	Start *string `json:"start,omitempty"`
	End   *string `json:"end,omitempty"`
	Limit *int    `json:"limit,omitempty"`
}

func (d *Deps) metrics(_ context.Context, _ *mcp.CallToolRequest, in metricsIn) (*mcp.CallToolResult, any, error) {
	q := url.Values{}
	if in.Match != nil && *in.Match != "" {
		q.Add("match[]", *in.Match)
	}
	if in.Start != nil && *in.Start != "" {
		q.Set("start", *in.Start)
	}
	if in.End != nil && *in.End != "" {
		q.Set("end", *in.End)
	}
	if in.Limit != nil && *in.Limit > 0 {
		q.Set("limit", strconv.Itoa(*in.Limit))
	}
	data, err := d.Prom.MakePrometheusRequest("label/__name__/values", q)
	if err != nil {
		return nil, nil, err
	}
	return nil, data, nil
}

type rulesIn struct {
	Type          *string  `json:"type,omitempty"`
	Filter        *string  `json:"filter,omitempty"`
	ExcludeAlerts *bool    `json:"exclude_alerts,omitempty"`
	RuleNames     []string `json:"rule_names,omitempty"`
	RuleGroups    []string `json:"rule_groups,omitempty"`
	RuleFiles     []string `json:"rule_files,omitempty"`
}

func (d *Deps) rules(_ context.Context, _ *mcp.CallToolRequest, in rulesIn) (*mcp.CallToolResult, any, error) {
	q := url.Values{}
	if in.Type != nil && *in.Type != "" {
		q.Set("type", *in.Type)
	}
	if in.Filter != nil && *in.Filter != "" {
		q.Set("filter", *in.Filter)
	}
	if in.ExcludeAlerts != nil && *in.ExcludeAlerts {
		q.Set("exclude_alerts", "true")
	}
	for _, n := range in.RuleNames {
		q.Add("rule_name[]", n)
	}
	for _, g := range in.RuleGroups {
		q.Add("rule_group[]", g)
	}
	for _, f := range in.RuleFiles {
		q.Add("file[]", f)
	}
	data, err := d.Prom.MakeVMAlertRequest("rules", q)
	if err != nil {
		data, err = d.Prom.MakePrometheusRequest("rules", q)
	}
	if err != nil {
		return nil, nil, err
	}
	return nil, data, nil
}

type alertsIn struct {
	State  *string `json:"state,omitempty"`
	Group  *string `json:"group,omitempty"`
	Limit  *int    `json:"limit,omitempty"`
	Offset *int    `json:"offset,omitempty"`
}

func (d *Deps) alerts(_ context.Context, _ *mcp.CallToolRequest, in alertsIn) (*mcp.CallToolResult, any, error) {
	data, err := d.Prom.MakeVMAlertRequest("alerts", nil)
	if err != nil {
		data, err = d.Prom.MakePrometheusRequest("alerts", nil)
	}
	if err != nil {
		return nil, nil, err
	}
	m, ok := data.(map[string]any)
	if !ok {
		return nil, data, nil
	}
	alertsAny, ok := m["alerts"]
	if !ok {
		return nil, data, nil
	}
	alerts, ok := alertsAny.([]any)
	if !ok {
		return nil, data, nil
	}
	state := "all"
	if in.State != nil && strings.TrimSpace(*in.State) != "" {
		state = strings.ToLower(strings.TrimSpace(*in.State))
	}
	groupFilter := ""
	if in.Group != nil {
		groupFilter = *in.Group
	}
	filtered := make([]any, 0, len(alerts))
	for _, a := range alerts {
		alert, ok := a.(map[string]any)
		if !ok {
			continue
		}
		if state != "all" {
			if st, _ := alert["state"].(string); strings.ToLower(st) != state {
				continue
			}
		}
		if groupFilter != "" {
			labels, _ := alert["labels"].(map[string]any)
			gn, _ := labels["alertgroup"].(string)
			if gn != groupFilter {
				continue
			}
		}
		filtered = append(filtered, alert)
	}
	sort.Slice(filtered, func(i, j int) bool {
		ai, _ := filtered[i].(map[string]any)
		aj, _ := filtered[j].(map[string]any)
		idI, _ := ai["id"].(string)
		idJ, _ := aj["id"].(string)
		return idI < idJ
	})
	off := 0
	if in.Offset != nil && *in.Offset > 0 {
		off = *in.Offset
	}
	if off > len(filtered) {
		off = len(filtered)
	}
	end := len(filtered)
	if in.Limit != nil && *in.Limit > 0 {
		end = off + *in.Limit
		if end > len(filtered) {
			end = len(filtered)
		}
	}
	filtered = filtered[off:end]
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	out["alerts"] = filtered
	return nil, out, nil
}

type exportIn struct {
	Match  string `json:"match"`
	Start  string `json:"start,omitempty"`
	End    string `json:"end,omitempty"`
	Format string `json:"format"` // json | csv
}

func (d *Deps) exportSeries(_ context.Context, _ *mcp.CallToolRequest, in exportIn) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.Match) == "" {
		return nil, nil, fmt.Errorf("match is required")
	}
	format := strings.ToLower(strings.TrimSpace(in.Format))
	if format == "" {
		format = "json"
	}
	apiPath := "export"
	if format == "csv" {
		apiPath = "export/csv"
	} else if format != "json" {
		return nil, nil, fmt.Errorf("format must be json or csv")
	}
	q := url.Values{}
	q.Add("match[]", in.Match)
	if in.Start != "" {
		q.Set("start", in.Start)
	}
	if in.End != "" {
		q.Set("end", in.End)
	}
	q.Set("format", format)
	raw, err := d.Prom.GetAPIv1Raw(apiPath, q)
	if err != nil {
		return nil, nil, err
	}
	return nil, map[string]any{
		"format": format,
		"body":   string(raw),
	}, nil
}

type tsdbStatusIn struct {
	TopN       *int    `json:"topN,omitempty"`
	FocusLabel *string `json:"focusLabel,omitempty"`
	Date       *string `json:"date,omitempty"`
	Match      *string `json:"match,omitempty"`
	ExtraLabel *string `json:"extra_label,omitempty"`
}

func (d *Deps) tsdbStatus(_ context.Context, _ *mcp.CallToolRequest, in tsdbStatusIn) (*mcp.CallToolResult, any, error) {
	q := url.Values{}
	topN := 10
	if in.TopN != nil && *in.TopN > 0 {
		topN = *in.TopN
	}
	q.Set("topN", strconv.Itoa(topN))
	if in.FocusLabel != nil && *in.FocusLabel != "" {
		q.Set("focusLabel", *in.FocusLabel)
	}
	if in.Date != nil && *in.Date != "" {
		q.Set("date", *in.Date)
	}
	if in.Match != nil && *in.Match != "" {
		q.Add("match[]", *in.Match)
	}
	if in.ExtraLabel != nil && *in.ExtraLabel != "" {
		q.Set("extra_label", *in.ExtraLabel)
	}
	data, err := d.Prom.MakePrometheusRequest("status/tsdb", q)
	if err != nil {
		return nil, nil, err
	}
	return nil, data, nil
}

type topQueriesIn struct {
	TopN        int    `json:"topN"`
	MaxLifetime string `json:"maxLifetime,omitempty"`
}

func (d *Deps) topQueries(_ context.Context, _ *mcp.CallToolRequest, in topQueriesIn) (*mcp.CallToolResult, any, error) {
	topN := in.TopN
	if topN < 1 {
		topN = 20
	}
	q := url.Values{}
	q.Set("topN", strconv.Itoa(topN))
	if strings.TrimSpace(in.MaxLifetime) != "" {
		q.Set("maxLifetime", strings.TrimSpace(in.MaxLifetime))
	}
	data, err := d.Prom.MakePrometheusRequest("status/top_queries", q)
	if err != nil {
		return nil, nil, err
	}
	return nil, data, nil
}

type metricStatisticsIn struct {
	MatchPattern *string `json:"match_pattern,omitempty"`
	Limit        *int    `json:"limit,omitempty"`
	Le           *int    `json:"le,omitempty"`
}

func (d *Deps) metricStatistics(_ context.Context, _ *mcp.CallToolRequest, in metricStatisticsIn) (*mcp.CallToolResult, any, error) {
	q := url.Values{}
	if in.MatchPattern != nil && *in.MatchPattern != "" {
		q.Set("match_pattern", *in.MatchPattern)
	}
	if in.Limit != nil && *in.Limit > 0 {
		q.Set("limit", strconv.Itoa(*in.Limit))
	}
	if in.Le != nil {
		q.Set("le", strconv.Itoa(*in.Le))
	}
	data, err := d.Prom.MakePrometheusRequest("status/metric_names_stats", q)
	if err != nil {
		return nil, nil, err
	}
	return nil, data, nil
}

type emptyIn struct{}

func (d *Deps) activeQueries(_ context.Context, _ *mcp.CallToolRequest, _ emptyIn) (*mcp.CallToolResult, any, error) {
	data, err := d.Prom.MakePrometheusRequest("status/active_queries", nil)
	if err != nil {
		return nil, nil, err
	}
	return nil, data, nil
}

func (d *Deps) getBuildInfo(_ context.Context, _ *mcp.CallToolRequest, _ emptyIn) (*mcp.CallToolResult, any, error) {
	data, err := d.Prom.MakePrometheusRequest("status/buildinfo", nil)
	if err != nil {
		return nil, nil, err
	}
	return nil, data, nil
}

func (d *Deps) getRuntimeInfo(_ context.Context, _ *mcp.CallToolRequest, _ emptyIn) (*mcp.CallToolResult, any, error) {
	data, err := d.Prom.MakePrometheusRequest("status/runtimeinfo", nil)
	if err != nil {
		return nil, nil, err
	}
	return nil, data, nil
}

func (d *Deps) getConfigYAML(_ context.Context, _ *mcp.CallToolRequest, _ emptyIn) (*mcp.CallToolResult, any, error) {
	data, err := d.Prom.MakePrometheusRequest("status/config", nil)
	if err != nil {
		return nil, nil, err
	}
	return nil, data, nil
}

func (d *Deps) getFlags(_ context.Context, _ *mcp.CallToolRequest, _ emptyIn) (*mcp.CallToolResult, any, error) {
	data, err := d.Prom.MakePrometheusRequest("status/flags", nil)
	if err == nil {
		return nil, data, nil
	}
	raw, err2 := d.Prom.GetUnderAdminRootRaw("flags", nil)
	if err2 != nil {
		return nil, nil, fmt.Errorf("status/flags: %v; admin /flags: %w", err, err2)
	}
	return nil, map[string]any{
		"source": "admin_root_text",
		"body":   string(raw),
	}, nil
}

type prettifyIn struct {
	Query string `json:"query"`
}

func (d *Deps) prettifyQuery(_ context.Context, _ *mcp.CallToolRequest, in prettifyIn) (*mcp.CallToolResult, any, error) {
	q := strings.TrimSpace(in.Query)
	if q == "" {
		return nil, nil, fmt.Errorf("query is required")
	}
	if pretty, err := metricsql.Prettify(q); err == nil && strings.TrimSpace(pretty) != "" {
		return nil, map[string]any{"prettified": pretty, "source": "metricsql"}, nil
	}
	qv := url.Values{}
	qv.Set("query", q)
	raw, err := d.Prom.GetUnderMetricsRootRaw("prettify-query", qv)
	if err != nil {
		return nil, nil, err
	}
	return nil, map[string]any{"prettified": string(raw), "source": "http"}, nil
}

type explainIn struct {
	Query string `json:"query"`
}

func (d *Deps) explainQuery(_ context.Context, _ *mcp.CallToolRequest, in explainIn) (*mcp.CallToolResult, any, error) {
	q := strings.TrimSpace(in.Query)
	if q == "" {
		return nil, nil, fmt.Errorf("query is required")
	}
	expr, err := metricsql.Parse(q)
	if err != nil {
		return nil, nil, err
	}
	canonical := string(expr.AppendString(nil))
	pretty, _ := metricsql.Prettify(q)
	return nil, map[string]any{
		"canonical":  canonical,
		"prettified": pretty,
		"note":       "For function semantics see https://docs.victoriametrics.com/metricsql/",
	}, nil
}

func (d *Deps) documentation(_ context.Context, _ *mcp.CallToolRequest, _ emptyIn) (*mcp.CallToolResult, any, error) {
	return nil, map[string]any{
		"metricsql":                "https://docs.victoriametrics.com/metricsql/",
		"prometheus_query_api":     "https://prometheus.io/docs/prometheus/latest/querying/api/",
		"victoriametrics_prom_api": "https://docs.victoriametrics.com/victoriametrics/#prometheus-querying-api-usage",
	}, nil
}

type metricRelabelDebugIn struct {
	RelabelConfigs string `json:"relabel_configs"`
	Metric         string `json:"metric"`
}

func (d *Deps) metricRelabelDebug(_ context.Context, _ *mcp.CallToolRequest, in metricRelabelDebugIn) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.RelabelConfigs) == "" || strings.TrimSpace(in.Metric) == "" {
		return nil, nil, fmt.Errorf("relabel_configs and metric are required")
	}
	q := url.Values{}
	q.Set("relabel_configs", in.RelabelConfigs)
	q.Set("metric", in.Metric)
	q.Set("format", "json")
	raw, err := d.Prom.GetUnderMetricsRootRaw("metric-relabel-debug", q)
	if err != nil {
		return nil, nil, err
	}
	var parsed any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, map[string]any{"raw": string(raw)}, nil
	}
	return nil, parsed, nil
}

type retentionFiltersDebugIn struct {
	Flags   string `json:"flags"`
	Metrics string `json:"metrics"`
}

func (d *Deps) retentionFiltersDebug(_ context.Context, _ *mcp.CallToolRequest, in retentionFiltersDebugIn) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.Flags) == "" || strings.TrimSpace(in.Metrics) == "" {
		return nil, nil, fmt.Errorf("flags and metrics are required")
	}
	q := url.Values{}
	q.Set("flags", in.Flags)
	q.Set("metrics", in.Metrics)
	raw, err := d.Prom.GetUnderMetricsRootRaw("retention-filters-debug", q)
	if err != nil {
		return nil, nil, err
	}
	return nil, map[string]any{"body": string(raw)}, nil
}

type downsamplingFiltersDebugIn struct {
	Flags   string `json:"flags"`
	Metrics string `json:"metrics"`
}

func (d *Deps) downsamplingFiltersDebug(_ context.Context, _ *mcp.CallToolRequest, in downsamplingFiltersDebugIn) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.Flags) == "" || strings.TrimSpace(in.Metrics) == "" {
		return nil, nil, fmt.Errorf("flags and metrics are required")
	}
	q := url.Values{}
	q.Set("flags", in.Flags)
	q.Set("metrics", in.Metrics)
	raw, err := d.Prom.GetUnderMetricsRootRaw("downsampling-filters-debug", q)
	if err != nil {
		return nil, nil, err
	}
	return nil, map[string]any{"body": string(raw)}, nil
}
