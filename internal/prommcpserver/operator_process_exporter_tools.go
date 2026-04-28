package prommcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerOperatorProcessExporterTools adds Linux process-exporter (ncabatoff/process-exporter) helpers.
func (d *Deps) registerOperatorProcessExporterTools(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name: d.Cfg.ToolName("operator_process_exporter_report"),
		Description: "Linux SRE: ncabatoff/process-exporter namedprocess_namegroup_* metrics — peak CPU per process group (cores ≈ rate sum), " +
			"RSS memory, process count, worst FD ratio, zombie threads, exporter scrape error rates. " +
			"Use label_selector (e.g. job=\"process-exporter\") to match your scrape. Set include_all=true to enable all blocks.",
		Title:       "Operator process-exporter report",
		Annotations: toolAnnotations("Operator process-exporter report"),
	}, d.operatorProcessExporterReport)
}

type operatorProcessExporterReportIn struct {
	Lookback      string `json:"lookback"` // e.g. 3h, 24h
	SubqueryStep  string `json:"subquery_step,omitempty"`
	RateWindow    string `json:"rate_window,omitempty"`
	LabelSelector string `json:"label_selector,omitempty"` // without outer braces, e.g. job="process-exporter"

	IncludeAll *bool `json:"include_all,omitempty"`

	IncludeCPUPerGroup *bool `json:"include_cpu_per_group,omitempty"`
	// Peak sum(rate(cpu user+system)) per group; interpret as ~CPU cores consumed by that group.
	CPUSecondsPerSecThreshold *float64 `json:"cpu_seconds_per_sec_threshold,omitempty"` // default 1

	IncludeMemoryRSS        *bool    `json:"include_memory_rss,omitempty"`
	MemoryRSSBytesThreshold *float64 `json:"memory_rss_bytes_threshold,omitempty"`

	IncludeNumProcs       *bool    `json:"include_num_procs,omitempty"`
	NumProcsHighThreshold *float64 `json:"num_procs_high_threshold,omitempty"`

	IncludeWorstFDRatio   *bool    `json:"include_worst_fd_ratio,omitempty"`
	WorstFDRatioThreshold *float64 `json:"worst_fd_ratio_threshold,omitempty"` // default 0.9

	IncludeZombieProcs       *bool    `json:"include_zombie_procs,omitempty"`
	ZombieCountHighThreshold *float64 `json:"zombie_count_high_threshold,omitempty"` // default 1

	IncludeScrapeHealth         *bool    `json:"include_scrape_health,omitempty"`
	ScrapeErrorsPerSecThreshold *float64 `json:"scrape_errors_per_sec_threshold,omitempty"` // default 0.01
}

func (d *Deps) operatorProcessExporterReport(_ context.Context, _ *mcp.CallToolRequest, in operatorProcessExporterReportIn) (*mcp.CallToolResult, any, error) {
	lookback := strings.TrimSpace(in.Lookback)
	if lookback == "" {
		lookback = "3h"
	}
	if err := validateDuration(lookback); err != nil {
		return nil, nil, err
	}
	step := strings.TrimSpace(in.SubqueryStep)
	if step == "" {
		step = "5m"
	}
	rateWin := strings.TrimSpace(in.RateWindow)
	if rateWin == "" {
		rateWin = "5m"
	}
	sel := strings.TrimSpace(in.LabelSelector)
	selComma := ""
	if sel != "" {
		selComma = "," + sel
	}

	all := ptrTrue(in.IncludeAll)
	incCPU := ptrTrue(in.IncludeCPUPerGroup) || all
	incMem := ptrTrue(in.IncludeMemoryRSS) || all
	incNP := ptrTrue(in.IncludeNumProcs) || all
	incFD := ptrTrue(in.IncludeWorstFDRatio) || all
	incZ := ptrTrue(in.IncludeZombieProcs) || all
	incScr := ptrTrue(in.IncludeScrapeHealth) || all

	cpuTh := 1.0
	if in.CPUSecondsPerSecThreshold != nil {
		cpuTh = *in.CPUSecondsPerSecThreshold
	}
	fdTh := 0.9
	if in.WorstFDRatioThreshold != nil {
		fdTh = *in.WorstFDRatioThreshold
	}
	zTh := 1.0
	if in.ZombieCountHighThreshold != nil {
		zTh = *in.ZombieCountHighThreshold
	}
	scrTh := 0.01
	if in.ScrapeErrorsPerSecThreshold != nil {
		scrTh = *in.ScrapeErrorsPerSecThreshold
	}

	out := map[string]any{
		"lookback":                         lookback,
		"subquery_step":                    step,
		"rate_window":                      rateWin,
		"label_selector":                   sel,
		"include_all":                      all,
		"effective_include_cpu_per_group":  incCPU,
		"effective_include_memory_rss":     incMem,
		"effective_include_num_procs":      incNP,
		"effective_include_worst_fd_ratio": incFD,
		"effective_include_zombie_procs":   incZ,
		"effective_include_scrape_health":  incScr,
		"cpu_seconds_per_sec_threshold":    cpuTh,
		"worst_fd_ratio_threshold":         fdTh,
		"zombie_count_high_threshold":      zTh,
		"scrape_errors_per_sec_threshold":  scrTh,
	}
	if in.MemoryRSSBytesThreshold != nil {
		out["memory_rss_bytes_threshold"] = *in.MemoryRSSBytesThreshold
	}
	if in.NumProcsHighThreshold != nil {
		out["num_procs_high_threshold"] = *in.NumProcsHighThreshold
	}

	probeSel := fmt.Sprintf(`namedprocess_namegroup_cpu_seconds_total{%s}`, strings.TrimPrefix(selComma, ","))
	if selComma == "" {
		probeSel = `namedprocess_namegroup_cpu_seconds_total`
	}
	ok, errProbe := d.metricPresent(probeSel)
	if errProbe != nil {
		out["probe_error"] = errProbe.Error()
	}
	if !ok {
		out["error"] = "no process-exporter group CPU metrics found for this label_selector"
		out["hint"] = "Point label_selector at your process-exporter job (e.g. job=\"process-exporter\"). Hosts without the exporter will not appear."
		return nil, out, nil
	}
	out["probe_namedprocess_namegroup_cpu_seconds_total"] = true

	if incCPU {
		q := processExporterCPUPeakExpr(selComma, rateWin, lookback, step)
		out["promql_process_group_cpu_peak_cores"] = q
		samples, errQ := d.promInstantVector(q)
		if errQ != nil {
			out["process_group_cpu_error"] = errQ.Error()
		} else {
			out["process_group_cpu_peak_by_instance_group"] = samplesToMapsNamed(samples, "peak_cpu_cores")
			out["process_group_cpu_breaches"] = filterBreachesNamed(samples, cpuTh, "peak_cpu_cores")
		}
	}
	if incMem {
		q := processExporterMemoryRSSPeakExpr(selComma, lookback, step)
		out["promql_process_group_memory_rss_peak_bytes"] = q
		samples, errQ := d.promInstantVector(q)
		if errQ != nil {
			out["process_group_memory_rss_error"] = errQ.Error()
		} else {
			out["process_group_memory_rss_peak_by_instance_group"] = samplesToMapsNamed(samples, "peak_rss_bytes")
			if in.MemoryRSSBytesThreshold != nil {
				out["process_group_memory_rss_breaches"] = filterBreachesNamed(samples, *in.MemoryRSSBytesThreshold, "peak_rss_bytes")
			}
		}
	}
	if incNP {
		q := processExporterNumProcsPeakExpr(selComma, lookback, step)
		out["promql_process_group_num_procs_peak"] = q
		samples, errQ := d.promInstantVector(q)
		if errQ != nil {
			out["process_group_num_procs_error"] = errQ.Error()
		} else {
			out["process_group_num_procs_peak_by_instance_group"] = samplesToMapsNamed(samples, "peak_num_procs")
			if in.NumProcsHighThreshold != nil {
				out["process_group_num_procs_breaches"] = filterBreachesNamed(samples, *in.NumProcsHighThreshold, "peak_num_procs")
			}
		}
	}
	if incFD {
		q := processExporterWorstFDRatioPeakExpr(selComma, lookback, step)
		out["promql_process_group_worst_fd_ratio_peak"] = q
		samples, errQ := d.promInstantVector(q)
		if errQ != nil {
			out["process_group_worst_fd_ratio_error"] = errQ.Error()
		} else {
			out["process_group_worst_fd_ratio_peak_by_instance_group"] = samplesToMapsNamed(samples, "peak_worst_fd_ratio")
			out["process_group_worst_fd_ratio_breaches"] = filterBreachesNamed(samples, fdTh, "peak_worst_fd_ratio")
		}
	}
	if incZ {
		q := processExporterZombiePeakExpr(selComma, lookback, step)
		out["promql_process_group_zombie_threads_peak"] = q
		samples, errQ := d.promInstantVector(q)
		if errQ != nil {
			out["process_group_zombie_error"] = errQ.Error()
		} else {
			out["process_group_zombie_peak_by_instance_group"] = samplesToMapsNamed(samples, "peak_zombie_threads")
			out["process_group_zombie_breaches"] = filterBreachesNamed(samples, zTh, "peak_zombie_threads")
		}
	}
	if incScr {
		q := processExporterScrapeErrorsPeakExpr(selComma, rateWin, lookback, step)
		out["promql_process_exporter_scrape_errors_peak_per_sec"] = q
		samples, errQ := d.promInstantVector(q)
		if errQ != nil {
			out["process_exporter_scrape_errors_error"] = errQ.Error()
		} else {
			out["process_exporter_scrape_errors_peak_by_instance"] = samplesToMapsNamed(samples, "peak_errors_per_sec")
			out["process_exporter_scrape_errors_breaches"] = filterBreachesNamed(samples, scrTh, "peak_errors_per_sec")
		}
		partialQ := processExporterScrapePartialErrorsPeakExpr(selComma, rateWin, lookback, step)
		out["promql_process_exporter_scrape_partial_errors_peak_per_sec"] = partialQ
		if partialSamples, errP := d.promInstantVector(partialQ); errP != nil {
			out["process_exporter_scrape_partial_errors_error"] = errP.Error()
		} else {
			out["process_exporter_scrape_partial_errors_peak_by_instance"] = samplesToMapsNamed(partialSamples, "peak_partial_errors_per_sec")
		}
	}

	out["caveats"] = []string{
		"process-exporter only sees configured process groups; metrics are absent on hosts without the exporter or without matching matchers.",
		"peak_cpu_cores is sum(rate(namedprocess_namegroup_cpu_seconds_total)) across mode=user+system — roughly cores if all work is on-CPU.",
		"namedprocess_namegroup_states counts threads in each state; Zombie uses state=\"Zombie\".",
		"Compare with operator_host_resource_report (node_exporter) for host-level CPU/mem vs per-application groups here.",
	}

	return nil, out, nil
}

func processExporterMetricSuffix(selComma string, extraLabels ...string) string {
	var parts []string
	if s := strings.TrimPrefix(selComma, ","); s != "" {
		parts = append(parts, s)
	}
	parts = append(parts, extraLabels...)
	if len(parts) == 0 {
		return ""
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func processExporterCPUPeakExpr(selComma, rateWin, lookback, step string) string {
	suff := processExporterMetricSuffix(selComma)
	var inner string
	if suff == "" {
		inner = fmt.Sprintf(
			`sum by (instance, groupname) (rate(namedprocess_namegroup_cpu_seconds_total[%s]))`,
			rateWin,
		)
	} else {
		inner = fmt.Sprintf(
			`sum by (instance, groupname) (rate(namedprocess_namegroup_cpu_seconds_total%s[%s]))`,
			suff, rateWin,
		)
	}
	return fmt.Sprintf(`max_over_time((%s)[%s:%s])`, inner, lookback, step)
}

func processExporterMemoryRSSPeakExpr(selComma, lookback, step string) string {
	suff := processExporterMetricSuffix(selComma, `memtype="resident"`)
	inner := fmt.Sprintf(`namedprocess_namegroup_memory_bytes%s`, suff)
	return fmt.Sprintf(`max_over_time((%s)[%s:%s])`, inner, lookback, step)
}

func processExporterNumProcsPeakExpr(selComma, lookback, step string) string {
	suff := processExporterMetricSuffix(selComma)
	var inner string
	if suff == "" {
		inner = `namedprocess_namegroup_num_procs`
	} else {
		inner = fmt.Sprintf(`namedprocess_namegroup_num_procs%s`, suff)
	}
	return fmt.Sprintf(`max_over_time((%s)[%s:%s])`, inner, lookback, step)
}

func processExporterWorstFDRatioPeakExpr(selComma, lookback, step string) string {
	suff := processExporterMetricSuffix(selComma)
	var inner string
	if suff == "" {
		inner = `namedprocess_namegroup_worst_fd_ratio`
	} else {
		inner = fmt.Sprintf(`namedprocess_namegroup_worst_fd_ratio%s`, suff)
	}
	return fmt.Sprintf(`max_over_time((%s)[%s:%s])`, inner, lookback, step)
}

func processExporterZombiePeakExpr(selComma, lookback, step string) string {
	suff := processExporterMetricSuffix(selComma, `state="Zombie"`)
	inner := fmt.Sprintf(`namedprocess_namegroup_states%s`, suff)
	return fmt.Sprintf(`max_over_time((%s)[%s:%s])`, inner, lookback, step)
}

func processExporterScrapeErrorsPeakExpr(selComma, rateWin, lookback, step string) string {
	suff := processExporterMetricSuffix(selComma)
	var inner string
	if suff == "" {
		inner = fmt.Sprintf(`rate(namedprocess_scrape_errors[%s])`, rateWin)
	} else {
		inner = fmt.Sprintf(`rate(namedprocess_scrape_errors%s[%s])`, suff, rateWin)
	}
	return fmt.Sprintf(`max_over_time((%s)[%s:%s])`, inner, lookback, step)
}

func processExporterScrapePartialErrorsPeakExpr(selComma, rateWin, lookback, step string) string {
	suff := processExporterMetricSuffix(selComma)
	var inner string
	if suff == "" {
		inner = fmt.Sprintf(`rate(namedprocess_scrape_partial_errors[%s])`, rateWin)
	} else {
		inner = fmt.Sprintf(`rate(namedprocess_scrape_partial_errors%s[%s])`, suff, rateWin)
	}
	return fmt.Sprintf(`max_over_time((%s)[%s:%s])`, inner, lookback, step)
}
