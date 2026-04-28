package prommcpserver

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerOperatorTools adds SRE-oriented tools that encode common PromQL/MMetricsQL recipes
// so operators get structured host lists without hand-writing queries.
func (d *Deps) registerOperatorTools(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name: d.Cfg.ToolName("operator_host_resource_report"),
		Description: "SRE/ops runbook: CPU, memory, filesystem (Linux + Windows), disk busy %, network errors + throughput, optional Linux extras (load/core, PSI, swap %, CPU iowait %, inode/filefd/conntrack, TCP retrans/listen drops/socket gauges, softnet, disk MB/s), " +
			"Windows extras (logical disk space + optional logical-disk throughput, physical disk busy %, network with default isatap/VPN NIC exclude, optional link utilization % vs windows_net_current_bandwidth_bytes). Set include_all_standard_metrics=true for all optional blocks. VictoriaMetrics-compatible PromQL.",
		Title:       "Operator host saturation report",
		Annotations: toolAnnotations("Operator host saturation report"),
	}, d.operatorHostResourceReport)
	d.registerOperatorProcessExporterTools(s)
}

type operatorHostResourceReportIn struct {
	Lookback            string   `json:"lookback"`                        // e.g. 3h, 24h, 30m
	CPUPercentThreshold *float64 `json:"cpu_percent_threshold,omitempty"` // default 80
	MemPercentThreshold *float64 `json:"memory_percent_threshold,omitempty"`
	SubqueryStep        string   `json:"subquery_step,omitempty"`  // default 5m (resolution inside max_over_time)
	RateWindow          string   `json:"rate_window,omitempty"`    // default 5m (inner rate())
	LabelSelector       string   `json:"label_selector,omitempty"` // extra matchers without braces, e.g. job="node-exporter",env="prod"
	MetricProfile       string   `json:"metric_profile,omitempty"` // auto | node_exporter | windows_exporter
	IncludeHealthyTopN  *int     `json:"include_healthy_top_n,omitempty"`

	// Linux node_exporter only: disk time busy % (0–100) per host = worst disk on that host.
	IncludeDiskIO        *bool    `json:"include_disk_io,omitempty"`
	DiskPercentThreshold *float64 `json:"disk_percent_threshold,omitempty"` // default 80

	// Linux node_exporter only: combined interface errors+drops / sec, and optional total throughput breach.
	IncludeNetwork               *bool    `json:"include_network,omitempty"`
	NetworkErrorsPerSecThreshold *float64 `json:"network_errors_per_sec_threshold,omitempty"` // default 1
	NetworkTotalMbpsThreshold    *float64 `json:"network_total_mbps_threshold,omitempty"`     // if set, breach when peak RX+TX exceeds this (approx, all non-lo ifaces)

	// Filesystem space used % (0–100): worst mountpoint (Linux) or volume (Windows) per series.
	IncludeFilesystem              *bool    `json:"include_filesystem,omitempty"`
	FilesystemUsedPercentThreshold *float64 `json:"filesystem_used_percent_threshold,omitempty"` // default 85

	// When true, enables every optional signal below for the detected OS (you can still narrow with label_selector).
	IncludeAllStandardMetrics *bool `json:"include_all_standard_metrics,omitempty"`

	// Linux node_exporter — saturation & kernel signals
	IncludeLoadPerCore              *bool    `json:"include_load_per_core,omitempty"`    // load1 / CPU count
	LoadPerCoreThreshold            *float64 `json:"load_per_core_threshold,omitempty"`  // default 1.0
	IncludePSI                      *bool    `json:"include_psi,omitempty"`              // max of CPU/memory/IO PSI waiting counter rates
	PsiStallRateThreshold           *float64 `json:"psi_stall_rate_threshold,omitempty"` // default 0.3 (fraction of time stalled, per rate())
	IncludeFilesystemInodes         *bool    `json:"include_filesystem_inodes,omitempty"`
	FilesystemInodePercentThreshold *float64 `json:"filesystem_inode_percent_threshold,omitempty"` // default 90
	IncludeFileDescriptors          *bool    `json:"include_file_descriptors,omitempty"`
	FileDescriptorPercentThreshold  *float64 `json:"file_descriptor_percent_threshold,omitempty"` // default 90
	IncludeConntrack                *bool    `json:"include_conntrack,omitempty"`
	ConntrackPercentThreshold       *float64 `json:"conntrack_percent_threshold,omitempty"` // default 90
	IncludeTCPRetransmits           *bool    `json:"include_tcp_retransmits,omitempty"`
	TcpRetransPerSecThreshold       *float64 `json:"tcp_retrans_per_sec_threshold,omitempty"` // default 10
	IncludeSoftnetDrops             *bool    `json:"include_softnet_drops,omitempty"`
	SoftnetDropsPerSecThreshold     *float64 `json:"softnet_drops_per_sec_threshold,omitempty"` // default 1

	// Windows network throughput breach (optional); errors use network_errors_per_sec_threshold.
	WindowsNetworkTotalMbpsThreshold *float64 `json:"windows_network_total_mbps_threshold,omitempty"`

	// Windows: regex for nic!~ (RE2). Nil → default isatap.*|VPN.*; empty string disables exclusion (sum all interfaces).
	WindowsNicExcludeRegex *string `json:"windows_nic_exclude_regex,omitempty"`

	// Windows: peak link utilization % = rate(bytes)*8 / windows_net_current_bandwidth_bytes * 100 (worst NIC per host).
	IncludeNetworkLinkPercent            *bool    `json:"include_network_link_percent,omitempty"`
	NetworkLinkPercentThreshold          *float64 `json:"network_link_percent_threshold,omitempty"`            // default 90
	WindowsLogicalDiskTotalMbpsThreshold *float64 `json:"windows_logical_disk_total_mbps_threshold,omitempty"` // breach on peak read+write per volume

	// Linux: swap space used % (SwapTotal − SwapFree).
	IncludeSwap              *bool    `json:"include_swap,omitempty"`
	SwapUsedPercentThreshold *float64 `json:"swap_used_percent_threshold,omitempty"` // default 80

	// Linux: CPU time in iowait mode (%) — disk wait visible to scheduler.
	IncludeCPUIOWait          *bool    `json:"include_cpu_iowait,omitempty"`
	CPUIOWaitPercentThreshold *float64 `json:"cpu_iowait_percent_threshold,omitempty"` // default 40

	// Linux: per-host peak disk read/write throughput (MB/s, bytes/1e6), max over devices.
	IncludeDiskThroughput  *bool    `json:"include_disk_throughput,omitempty"`
	DiskReadMBpsThreshold  *float64 `json:"disk_read_megabytes_per_sec_threshold,omitempty"`
	DiskWriteMBpsThreshold *float64 `json:"disk_write_megabytes_per_sec_threshold,omitempty"`

	// Linux: TCP listen backlog drops (rate) and optional socket gauge thresholds.
	IncludeTCPListenDrops         *bool    `json:"include_tcp_listen_drops,omitempty"`
	TcpListenDropsPerSecThreshold *float64 `json:"tcp_listen_drops_per_sec_threshold,omitempty"` // default 0.1
	IncludeTCPSocketStats         *bool    `json:"include_tcp_socket_stats,omitempty"`           // time-wait + established counts
	TcpTimeWaitHighThreshold      *float64 `json:"tcp_time_wait_high_threshold,omitempty"`       // optional breach on peak TCP_tw
	TcpEstablishedHighThreshold   *float64 `json:"tcp_established_high_threshold,omitempty"`     // optional breach on peak CurrEstab

	// Windows: peak logical-disk read+write combined throughput (Mbps), max over volumes (excludes _Total).
	IncludeWindowsLogicalDiskThroughput *bool `json:"include_windows_logical_disk_throughput,omitempty"`
}

func (d *Deps) operatorHostResourceReport(_ context.Context, _ *mcp.CallToolRequest, in operatorHostResourceReportIn) (*mcp.CallToolResult, any, error) {
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
	cpuTh := 80.0
	if in.CPUPercentThreshold != nil {
		cpuTh = *in.CPUPercentThreshold
	}
	memTh := 80.0
	if in.MemPercentThreshold != nil {
		memTh = *in.MemPercentThreshold
	}
	sel := strings.TrimSpace(in.LabelSelector)
	selComma := ""
	if sel != "" {
		selComma = "," + sel
	}
	profile := strings.ToLower(strings.TrimSpace(in.MetricProfile))
	if profile == "" {
		profile = "auto"
	}

	diskTh := 80.0
	if in.DiskPercentThreshold != nil {
		diskTh = *in.DiskPercentThreshold
	}
	netErrTh := 1.0
	if in.NetworkErrorsPerSecThreshold != nil {
		netErrTh = *in.NetworkErrorsPerSecThreshold
	}
	fsTh := 85.0
	if in.FilesystemUsedPercentThreshold != nil {
		fsTh = *in.FilesystemUsedPercentThreshold
	}
	loadPerCoreTh := 1.0
	if in.LoadPerCoreThreshold != nil {
		loadPerCoreTh = *in.LoadPerCoreThreshold
	}
	psiTh := 0.3
	if in.PsiStallRateThreshold != nil {
		psiTh = *in.PsiStallRateThreshold
	}
	inodeTh := 90.0
	if in.FilesystemInodePercentThreshold != nil {
		inodeTh = *in.FilesystemInodePercentThreshold
	}
	fdTh := 90.0
	if in.FileDescriptorPercentThreshold != nil {
		fdTh = *in.FileDescriptorPercentThreshold
	}
	connTh := 90.0
	if in.ConntrackPercentThreshold != nil {
		connTh = *in.ConntrackPercentThreshold
	}
	retransTh := 10.0
	if in.TcpRetransPerSecThreshold != nil {
		retransTh = *in.TcpRetransPerSecThreshold
	}
	softnetTh := 1.0
	if in.SoftnetDropsPerSecThreshold != nil {
		softnetTh = *in.SoftnetDropsPerSecThreshold
	}
	linkPctTh := 90.0
	if in.NetworkLinkPercentThreshold != nil {
		linkPctTh = *in.NetworkLinkPercentThreshold
	}
	swapTh := 80.0
	if in.SwapUsedPercentThreshold != nil {
		swapTh = *in.SwapUsedPercentThreshold
	}
	iowaitTh := 40.0
	if in.CPUIOWaitPercentThreshold != nil {
		iowaitTh = *in.CPUIOWaitPercentThreshold
	}
	listenDropTh := 0.1
	if in.TcpListenDropsPerSecThreshold != nil {
		listenDropTh = *in.TcpListenDropsPerSecThreshold
	}

	allStd := ptrTrue(in.IncludeAllStandardMetrics)
	incFS := ptrTrue(in.IncludeFilesystem) || allStd
	incDisk := ptrTrue(in.IncludeDiskIO) || allStd
	incNet := ptrTrue(in.IncludeNetwork) || allStd
	incLoad := ptrTrue(in.IncludeLoadPerCore) || allStd
	incPSI := ptrTrue(in.IncludePSI) || allStd
	incInode := ptrTrue(in.IncludeFilesystemInodes) || allStd
	incFD := ptrTrue(in.IncludeFileDescriptors) || allStd
	incConn := ptrTrue(in.IncludeConntrack) || allStd
	incRetrans := ptrTrue(in.IncludeTCPRetransmits) || allStd
	incSoftnet := ptrTrue(in.IncludeSoftnetDrops) || allStd
	incLinkPct := ptrTrue(in.IncludeNetworkLinkPercent) || allStd
	incWinLogDisk := ptrTrue(in.IncludeWindowsLogicalDiskThroughput) || allStd
	incSwap := ptrTrue(in.IncludeSwap) || allStd
	incIOWait := ptrTrue(in.IncludeCPUIOWait) || allStd
	incDiskTP := ptrTrue(in.IncludeDiskThroughput) || allStd
	incListen := ptrTrue(in.IncludeTCPListenDrops) || allStd
	incSock := ptrTrue(in.IncludeTCPSocketStats) || allStd

	winNicEx := windowsNicExclude(in)

	out := map[string]any{
		"lookback":                                          lookback,
		"subquery_step":                                     step,
		"rate_window":                                       rateWin,
		"cpu_threshold_percent":                             cpuTh,
		"memory_threshold_percent":                          memTh,
		"label_selector":                                    sel,
		"metric_profile_requested":                          profile,
		"include_all_standard_metrics":                      allStd,
		"include_disk_io":                                   ptrTrue(in.IncludeDiskIO),
		"include_network":                                   ptrTrue(in.IncludeNetwork),
		"include_filesystem":                                ptrTrue(in.IncludeFilesystem),
		"effective_include_filesystem":                      incFS,
		"effective_include_disk_io":                         incDisk,
		"effective_include_network":                         incNet,
		"effective_include_load_per_core":                   incLoad,
		"effective_include_psi":                             incPSI,
		"effective_include_filesystem_inodes":               incInode,
		"effective_include_file_descriptors":                incFD,
		"effective_include_conntrack":                       incConn,
		"effective_include_tcp_retransmits":                 incRetrans,
		"effective_include_softnet_drops":                   incSoftnet,
		"effective_include_network_link_percent":            incLinkPct,
		"effective_include_windows_logical_disk_throughput": incWinLogDisk,
		"effective_include_swap":                            incSwap,
		"effective_include_cpu_iowait":                      incIOWait,
		"effective_include_disk_throughput":                 incDiskTP,
		"effective_include_tcp_listen_drops":                incListen,
		"effective_include_tcp_socket_stats":                incSock,
		"windows_nic_exclude_regex_applied":                 winNicEx,
		"network_link_percent_threshold":                    linkPctTh,
		"swap_used_percent_threshold":                       swapTh,
		"cpu_iowait_percent_threshold":                      iowaitTh,
		"tcp_listen_drops_per_sec_threshold":                listenDropTh,
		"disk_threshold_percent":                            diskTh,
		"filesystem_used_percent_threshold":                 fsTh,
		"network_errors_per_sec_threshold":                  netErrTh,
		"load_per_core_threshold":                           loadPerCoreTh,
		"psi_stall_rate_threshold":                          psiTh,
		"filesystem_inode_percent_threshold":                inodeTh,
		"file_descriptor_percent_threshold":                 fdTh,
		"conntrack_percent_threshold":                       connTh,
		"tcp_retrans_per_sec_threshold":                     retransTh,
		"softnet_drops_per_sec_threshold":                   softnetTh,
	}
	if in.NetworkTotalMbpsThreshold != nil {
		out["network_total_mbps_threshold"] = *in.NetworkTotalMbpsThreshold
	}
	if in.WindowsNetworkTotalMbpsThreshold != nil {
		out["windows_network_total_mbps_threshold"] = *in.WindowsNetworkTotalMbpsThreshold
	}
	if in.WindowsLogicalDiskTotalMbpsThreshold != nil {
		out["windows_logical_disk_total_mbps_threshold"] = *in.WindowsLogicalDiskTotalMbpsThreshold
	}
	if in.DiskReadMBpsThreshold != nil {
		out["disk_read_megabytes_per_sec_threshold"] = *in.DiskReadMBpsThreshold
	}
	if in.DiskWriteMBpsThreshold != nil {
		out["disk_write_megabytes_per_sec_threshold"] = *in.DiskWriteMBpsThreshold
	}
	if in.TcpTimeWaitHighThreshold != nil {
		out["tcp_time_wait_high_threshold"] = *in.TcpTimeWaitHighThreshold
	}
	if in.TcpEstablishedHighThreshold != nil {
		out["tcp_established_high_threshold"] = *in.TcpEstablishedHighThreshold
	}

	probes := map[string]bool{}
	var usedProfile string

	// --- pick profile ---
	switch profile {
	case "node_exporter":
		usedProfile = "node_exporter"
	case "windows_exporter":
		usedProfile = "windows_exporter"
	case "auto":
		if ok, _ := d.metricPresent(fmt.Sprintf("node_cpu_seconds_total{mode=\"idle\"%s}", selComma)); ok {
			usedProfile = "node_exporter"
			probes["node_cpu_seconds_total"] = true
		} else if ok, _ := d.metricPresent(fmt.Sprintf("windows_cpu_time_total{mode=\"idle\"%s}", selComma)); ok {
			usedProfile = "windows_exporter"
			probes["windows_cpu_time_total"] = true
		} else {
			probes["node_cpu_seconds_total"] = false
			probes["windows_cpu_time_total"] = false
			out["probes"] = probes
			out["error"] = "auto-detect failed: neither node_cpu_seconds_total nor windows_cpu_time_total (idle) found with the given label_selector"
			out["hint"] = "Set metric_profile to node_exporter or windows_exporter, fix label_selector, or scrape node/windows exporters."
			return nil, out, nil
		}
	default:
		return nil, nil, fmt.Errorf("metric_profile must be auto, node_exporter, or windows_exporter")
	}
	out["metric_profile_used"] = usedProfile
	out["probes"] = probes

	var cpuExpr, memExpr string
	selInner := strings.TrimPrefix(selComma, ",")
	switch usedProfile {
	case "node_exporter":
		cpuExpr = fmt.Sprintf(
			`max_over_time((100 * (1 - avg by (instance) (rate(node_cpu_seconds_total{mode="idle"%s}[%s]))))[%s:%s])`,
			selComma, rateWin, lookback, step,
		)
		if selInner == "" {
			memExpr = fmt.Sprintf(
				`max_over_time((100 * (1 - node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes))[%s:%s])`,
				lookback, step,
			)
		} else {
			memExpr = fmt.Sprintf(
				`max_over_time((100 * (1 - node_memory_MemAvailable_bytes{%s} / node_memory_MemTotal_bytes{%s}))[%s:%s])`,
				selInner, selInner, lookback, step,
			)
		}
	case "windows_exporter":
		cpuExpr = fmt.Sprintf(
			`max_over_time((100 * (1 - avg by (instance) (rate(windows_cpu_time_total{mode="idle"%s}[%s]))))[%s:%s])`,
			selComma, rateWin, lookback, step,
		)
		if selInner == "" {
			memExpr = fmt.Sprintf(
				`max_over_time((100 * (1 - windows_memory_available_bytes / windows_cs_physical_memory_bytes))[%s:%s])`,
				lookback, step,
			)
		} else {
			memExpr = fmt.Sprintf(
				`max_over_time((100 * (1 - windows_memory_available_bytes{%s} / windows_cs_physical_memory_bytes{%s}))[%s:%s])`,
				selInner, selInner, lookback, step,
			)
		}
	}

	out["promql_cpu_peak_percent"] = cpuExpr
	out["promql_memory_used_peak_percent"] = memExpr

	var cpuSamples []promSample
	cpuSamples, errCPU := d.promInstantVector(cpuExpr)
	if errCPU != nil {
		out["cpu_error"] = errCPU.Error()
	} else {
		out["cpu_breaches"] = filterBreaches(cpuSamples, cpuTh)
		out["cpu_peak_by_instance"] = samplesToMaps(cpuSamples)
	}

	memSamples, errMem := d.promInstantVector(memExpr)
	if errMem != nil {
		out["memory_error"] = errMem.Error()
		out["memory_note"] = "Memory series missing or query unsupported; CPU may still be valid."
	} else {
		out["memory_breaches"] = filterBreaches(memSamples, memTh)
		out["memory_peak_by_instance"] = samplesToMaps(memSamples)
	}

	// --- Filesystem space used % (node: mountpoint; Windows: volume) ---
	if incFS {
		var fsExpr string
		switch usedProfile {
		case "node_exporter":
			fsExpr = nodeFilesystemUsedPeakExpr(selComma, lookback, step)
		case "windows_exporter":
			fsExpr = windowsFilesystemUsedPeakExpr(selComma, lookback, step)
		}
		out["promql_filesystem_used_peak_percent"] = fsExpr
		fsSamples, errFS := d.promInstantVector(fsExpr)
		if errFS != nil {
			out["filesystem_error"] = errFS.Error()
		} else {
			out["filesystem_breaches"] = filterBreaches(fsSamples, fsTh)
			out["filesystem_peak_by_mount_or_volume"] = samplesToMaps(fsSamples)
		}
	}

	// --- Disk I/O busy %: Linux node_disk_io_time; Windows physical disk idle counter ---
	if incDisk && usedProfile == "node_exporter" {
		diskExpr := nodeDiskBusyPeakExpr(selComma, rateWin, lookback, step)
		out["promql_disk_busy_peak_percent"] = diskExpr
		diskSamples, errDisk := d.promInstantVector(diskExpr)
		if errDisk != nil {
			out["disk_error"] = errDisk.Error()
		} else {
			out["disk_breaches"] = filterBreaches(diskSamples, diskTh)
			out["disk_peak_by_instance"] = samplesToMaps(diskSamples)
		}
	} else if incDisk && usedProfile == "windows_exporter" {
		diskExpr := windowsDiskBusyPeakExpr(selComma, rateWin, lookback, step)
		out["promql_disk_busy_peak_percent"] = diskExpr
		diskSamples, errDisk := d.promInstantVector(diskExpr)
		if errDisk != nil {
			out["disk_error"] = errDisk.Error()
		} else {
			out["disk_breaches"] = filterBreaches(diskSamples, diskTh)
			out["disk_peak_by_instance"] = samplesToMaps(diskSamples)
		}
	}

	if incWinLogDisk && usedProfile == "windows_exporter" {
		ldExpr := windowsLogicalDiskTotalMbpsPeakExpr(selComma, rateWin, lookback, step)
		out["promql_windows_logical_disk_total_throughput_peak_mbps"] = ldExpr
		ldSamples, errLD := d.promInstantVector(ldExpr)
		if errLD != nil {
			out["windows_logical_disk_throughput_error"] = errLD.Error()
		} else {
			out["windows_logical_disk_throughput_peak_by_instance"] = samplesToMapsNamed(ldSamples, "peak_mbps")
			if in.WindowsLogicalDiskTotalMbpsThreshold != nil {
				out["windows_logical_disk_throughput_breaches"] = filterBreachesNamed(ldSamples, *in.WindowsLogicalDiskTotalMbpsThreshold, "peak_mbps")
			}
		}
	}

	winMbpsTh := in.WindowsNetworkTotalMbpsThreshold
	if winMbpsTh == nil {
		winMbpsTh = in.NetworkTotalMbpsThreshold
	}

	// --- Network errors + throughput: node_network_* or windows_net_* ---
	if incNet && usedProfile == "node_exporter" {
		netErrExpr := nodeNetworkErrorsPeakExpr(selComma, rateWin, lookback, step)
		out["promql_network_errors_peak_per_sec"] = netErrExpr
		netErrSamples, errNE := d.promInstantVector(netErrExpr)
		if errNE != nil {
			out["network_errors_error"] = errNE.Error()
		} else {
			out["network_errors_breaches"] = filterBreachesNamed(netErrSamples, netErrTh, "peak_errors_per_sec")
			out["network_errors_peak_by_instance"] = samplesToMapsNamed(netErrSamples, "peak_errors_per_sec")
		}

		mbpsExpr := nodeNetworkTotalMbpsPeakExpr(selComma, rateWin, lookback, step)
		out["promql_network_total_throughput_peak_mbps"] = mbpsExpr
		mbpsSamples, errTP := d.promInstantVector(mbpsExpr)
		if errTP != nil {
			out["network_throughput_error"] = errTP.Error()
		} else {
			out["network_throughput_peak_by_instance"] = samplesToMapsNamed(mbpsSamples, "peak_mbps")
			if in.NetworkTotalMbpsThreshold != nil {
				out["network_throughput_breaches"] = filterBreachesNamed(mbpsSamples, *in.NetworkTotalMbpsThreshold, "peak_mbps")
			}
		}
	} else if usedProfile == "windows_exporter" && incNet {
		netErrExpr := windowsNetworkErrorsPeakExpr(selComma, rateWin, lookback, step, winNicEx)
		out["promql_network_errors_peak_per_sec"] = netErrExpr
		netErrSamples, errNE := d.promInstantVector(netErrExpr)
		if errNE != nil {
			out["network_errors_error"] = errNE.Error()
		} else {
			out["network_errors_breaches"] = filterBreachesNamed(netErrSamples, netErrTh, "peak_errors_per_sec")
			out["network_errors_peak_by_instance"] = samplesToMapsNamed(netErrSamples, "peak_errors_per_sec")
		}

		mbpsExpr := windowsNetworkTotalMbpsPeakExpr(selComma, rateWin, lookback, step, winNicEx)
		out["promql_network_total_throughput_peak_mbps"] = mbpsExpr
		mbpsSamples, errTP := d.promInstantVector(mbpsExpr)
		if errTP != nil {
			out["network_throughput_error"] = errTP.Error()
		} else {
			out["network_throughput_peak_by_instance"] = samplesToMapsNamed(mbpsSamples, "peak_mbps")
			if winMbpsTh != nil {
				out["network_throughput_breaches"] = filterBreachesNamed(mbpsSamples, *winMbpsTh, "peak_mbps")
			}
		}
	}
	if usedProfile == "windows_exporter" && incLinkPct {
		linkExpr := windowsNetworkLinkPercentPeakExpr(selComma, rateWin, lookback, step, winNicEx)
		out["promql_network_link_utilization_peak_percent"] = linkExpr
		linkSamples, errL := d.promInstantVector(linkExpr)
		if errL != nil {
			out["network_link_utilization_error"] = errL.Error()
		} else {
			out["network_link_utilization_peak_by_instance"] = samplesToMaps(linkSamples)
			out["network_link_utilization_breaches"] = filterBreaches(linkSamples, linkPctTh)
		}
	}

	// --- Linux node_exporter extras ---
	if usedProfile == "node_exporter" {
		if incLoad {
			loadExpr := nodeLoadPerCorePeakExpr(selComma, lookback, step)
			out["promql_load_per_core_peak"] = loadExpr
			loadSamples, errL := d.promInstantVector(loadExpr)
			if errL != nil {
				out["load_per_core_error"] = errL.Error()
			} else {
				out["load_per_core_breaches"] = filterBreachesNamed(loadSamples, loadPerCoreTh, "peak_load_per_core")
				out["load_per_core_peak_by_instance"] = samplesToMapsNamed(loadSamples, "peak_load_per_core")
			}
		}
		if incPSI {
			psiExpr := nodePSIPeakExpr(selComma, rateWin, lookback, step)
			out["promql_psi_stall_rate_peak"] = psiExpr
			psiSamples, errP := d.promInstantVector(psiExpr)
			if errP != nil {
				out["psi_error"] = errP.Error()
			} else {
				out["psi_stall_rate_breaches"] = filterBreachesNamed(psiSamples, psiTh, "peak_stall_rate")
				out["psi_stall_rate_peak_by_instance"] = samplesToMapsNamed(psiSamples, "peak_stall_rate")
			}
		}
		if incInode {
			inExpr := nodeFilesystemInodeUsedPeakExpr(selComma, lookback, step)
			out["promql_filesystem_inode_used_peak_percent"] = inExpr
			inSamples, errI := d.promInstantVector(inExpr)
			if errI != nil {
				out["filesystem_inodes_error"] = errI.Error()
			} else {
				out["filesystem_inode_breaches"] = filterBreaches(inSamples, inodeTh)
				out["filesystem_inode_peak_by_mountpoint"] = samplesToMaps(inSamples)
			}
		}
		if incFD {
			fdExpr := nodeFileDescriptorPercentPeakExpr(selComma, lookback, step)
			out["promql_file_descriptor_used_peak_percent"] = fdExpr
			fdSamples, errF := d.promInstantVector(fdExpr)
			if errF != nil {
				out["file_descriptor_error"] = errF.Error()
			} else {
				out["file_descriptor_breaches"] = filterBreaches(fdSamples, fdTh)
				out["file_descriptor_peak_by_instance"] = samplesToMaps(fdSamples)
			}
		}
		if incConn {
			ctExpr := nodeConntrackPercentPeakExpr(selComma, lookback, step)
			out["promql_conntrack_used_peak_percent"] = ctExpr
			ctSamples, errC := d.promInstantVector(ctExpr)
			if errC != nil {
				out["conntrack_error"] = errC.Error()
			} else {
				out["conntrack_breaches"] = filterBreaches(ctSamples, connTh)
				out["conntrack_peak_by_instance"] = samplesToMaps(ctSamples)
			}
		}
		if incRetrans {
			rtExpr := nodeTCPRetransPeakExpr(selComma, rateWin, lookback, step)
			out["promql_tcp_retransmits_peak_per_sec"] = rtExpr
			rtSamples, errR := d.promInstantVector(rtExpr)
			if errR != nil {
				out["tcp_retransmits_error"] = errR.Error()
			} else {
				out["tcp_retransmits_breaches"] = filterBreachesNamed(rtSamples, retransTh, "peak_retrans_per_sec")
				out["tcp_retransmits_peak_by_instance"] = samplesToMapsNamed(rtSamples, "peak_retrans_per_sec")
			}
		}
		if incSoftnet {
			snExpr := nodeSoftnetDropsPeakExpr(selComma, rateWin, lookback, step)
			out["promql_softnet_drops_peak_per_sec"] = snExpr
			snSamples, errS := d.promInstantVector(snExpr)
			if errS != nil {
				out["softnet_drops_error"] = errS.Error()
			} else {
				out["softnet_drops_breaches"] = filterBreachesNamed(snSamples, softnetTh, "peak_drops_per_sec")
				out["softnet_drops_peak_by_instance"] = samplesToMapsNamed(snSamples, "peak_drops_per_sec")
			}
		}
		if incSwap {
			swExpr := nodeSwapUsedPeakExpr(selComma, lookback, step)
			out["promql_swap_used_peak_percent"] = swExpr
			swSamples, errSw := d.promInstantVector(swExpr)
			if errSw != nil {
				out["swap_error"] = errSw.Error()
			} else {
				out["swap_breaches"] = filterBreaches(swSamples, swapTh)
				out["swap_peak_by_instance"] = samplesToMaps(swSamples)
			}
		}
		if incIOWait {
			iwExpr := nodeCPUIOWaitPeakExpr(selComma, rateWin, lookback, step)
			out["promql_cpu_iowait_peak_percent"] = iwExpr
			iwSamples, errIw := d.promInstantVector(iwExpr)
			if errIw != nil {
				out["cpu_iowait_error"] = errIw.Error()
			} else {
				out["cpu_iowait_breaches"] = filterBreaches(iwSamples, iowaitTh)
				out["cpu_iowait_peak_by_instance"] = samplesToMaps(iwSamples)
			}
		}
		if incDiskTP {
			rdExpr := nodeDiskReadMBpsPeakExpr(selComma, rateWin, lookback, step)
			out["promql_disk_read_peak_megabytes_per_sec"] = rdExpr
			rdSamples, errR := d.promInstantVector(rdExpr)
			if errR != nil {
				out["disk_read_throughput_error"] = errR.Error()
			} else {
				out["disk_read_throughput_peak_by_instance"] = samplesToMapsNamed(rdSamples, "peak_megabytes_per_sec")
				if in.DiskReadMBpsThreshold != nil {
					out["disk_read_throughput_breaches"] = filterBreachesNamed(rdSamples, *in.DiskReadMBpsThreshold, "peak_megabytes_per_sec")
				}
			}
			wrExpr := nodeDiskWriteMBpsPeakExpr(selComma, rateWin, lookback, step)
			out["promql_disk_write_peak_megabytes_per_sec"] = wrExpr
			wrSamples, errW := d.promInstantVector(wrExpr)
			if errW != nil {
				out["disk_write_throughput_error"] = errW.Error()
			} else {
				out["disk_write_throughput_peak_by_instance"] = samplesToMapsNamed(wrSamples, "peak_megabytes_per_sec")
				if in.DiskWriteMBpsThreshold != nil {
					out["disk_write_throughput_breaches"] = filterBreachesNamed(wrSamples, *in.DiskWriteMBpsThreshold, "peak_megabytes_per_sec")
				}
			}
		}
		if incListen {
			tldExpr := nodeTCPListenDropsPeakExpr(selComma, rateWin, lookback, step)
			out["promql_tcp_listen_drops_peak_per_sec"] = tldExpr
			tldSamples, errTld := d.promInstantVector(tldExpr)
			if errTld != nil {
				out["tcp_listen_drops_error"] = errTld.Error()
			} else {
				out["tcp_listen_drops_breaches"] = filterBreachesNamed(tldSamples, listenDropTh, "peak_listen_drops_per_sec")
				out["tcp_listen_drops_peak_by_instance"] = samplesToMapsNamed(tldSamples, "peak_listen_drops_per_sec")
			}
		}
		if incSock {
			twExpr := nodeTCPTimewaitPeakExpr(selComma, lookback, step)
			out["promql_tcp_time_wait_peak"] = twExpr
			twSamples, errTw := d.promInstantVector(twExpr)
			if errTw != nil {
				out["tcp_time_wait_error"] = errTw.Error()
			} else {
				out["tcp_time_wait_peak_by_instance"] = samplesToMapsNamed(twSamples, "peak_tcp_time_wait")
				if in.TcpTimeWaitHighThreshold != nil {
					out["tcp_time_wait_breaches"] = filterBreachesNamed(twSamples, *in.TcpTimeWaitHighThreshold, "peak_tcp_time_wait")
				}
			}
			estExpr := nodeTCPEstablishedPeakExpr(selComma, lookback, step)
			out["promql_tcp_established_peak"] = estExpr
			estSamples, errEs := d.promInstantVector(estExpr)
			if errEs != nil {
				out["tcp_established_error"] = errEs.Error()
			} else {
				out["tcp_established_peak_by_instance"] = samplesToMapsNamed(estSamples, "peak_tcp_established")
				if in.TcpEstablishedHighThreshold != nil {
					out["tcp_established_breaches"] = filterBreachesNamed(estSamples, *in.TcpEstablishedHighThreshold, "peak_tcp_established")
				}
			}
		}
	}

	out["caveats"] = []string{
		"`instance` is the usual host key; map it to your CMDB. On Kubernetes, relabel `nodename` or internal node name onto targets if that is how you think about hosts.",
		"node_exporter DaemonSet metrics describe the **node OS**, not individual pods. For pod CPU/mem, restarts, PVC usage, Deployments—scrape **kube-state-metrics**, **kubelet/cAdvisor**, or the **Kubernetes mixin** metrics and use execute_query.",
		"Breaches are where *peak* utilization over the window reached the threshold (not necessarily still above it now).",
		"Subqueries are expensive; keep lookback reasonable.",
		"Linux disk busy % uses node_disk_io_time_seconds_total; Windows uses windows_physical_disk_idle_seconds_total (idle-time counter semantics from Perflib).",
		"Windows logical disk free/size can lag 10–15 minutes per windows_exporter docs.",
		"Network 'Mbps' uses bytes rate × 8 / 1e6. Windows NICs default-exclude pseudo interfaces (isatap/VPN); override with windows_nic_exclude_regex (empty string disables).",
		"Windows link utilization % uses windows_net_current_bandwidth_bytes (verify name matches your windows_exporter version).",
		"PSI requires kernel support; file descriptor, conntrack, and TcpExt collectors must be enabled in node_exporter where used.",
		"TCP listen drops use node_netstat_TcpExt_ListenDrops; swap/iowait/disk MB/s thresholds are optional—set explicit limits for noisy environments.",
	}

	if in.IncludeHealthyTopN != nil && *in.IncludeHealthyTopN > 0 && len(cpuSamples) > 0 {
		out["top_cpu_under_threshold"] = topNUnder(cpuSamples, cpuTh, *in.IncludeHealthyTopN)
	}

	return nil, out, nil
}

func ptrTrue(p *bool) bool {
	return p != nil && *p
}

// windowsNicExclude: nil WindowsNicExcludeRegex → default StarsL/Grafana-style pseudo-NIC filter; "" disables.
func windowsNicExclude(in operatorHostResourceReportIn) string {
	if in.WindowsNicExcludeRegex != nil {
		return *in.WindowsNicExcludeRegex
	}
	return `isatap.*|VPN.*`
}

func windowsNetLbl(selComma, nicExcludeRE string) string {
	var parts []string
	if s := strings.TrimPrefix(selComma, ","); s != "" {
		parts = append(parts, s)
	}
	if nicExcludeRE != "" {
		parts = append(parts, fmt.Sprintf(`nic!~%q`, nicExcludeRE))
	}
	if len(parts) == 0 {
		return ""
	}
	return "{" + strings.Join(parts, ",") + "}"
}

// nodeFilesystemUsedPeakExpr: peak used % per instance+mountpoint over lookback (excludes common pseudo filesystems).
func nodeFilesystemUsedPeakExpr(selComma, lookback, step string) string {
	nodeFS := `mountpoint!="",fstype!~"tmpfs|proc|sysfs|devtmpfs|overlay|squashfs|rpc_pipefs|autofs|binfmt_misc"`
	var inner string
	if selComma == "" {
		inner = fmt.Sprintf(
			`max by (instance, mountpoint) (100 * (1 - node_filesystem_avail_bytes{%s} / node_filesystem_size_bytes{%s}))`,
			nodeFS, nodeFS,
		)
	} else {
		inner = fmt.Sprintf(
			`max by (instance, mountpoint) (100 * (1 - node_filesystem_avail_bytes{%s%s} / node_filesystem_size_bytes{%s%s}))`,
			nodeFS, selComma, nodeFS, selComma,
		)
	}
	return fmt.Sprintf(`max_over_time((%s)[%s:%s])`, inner, lookback, step)
}

// windowsFilesystemUsedPeakExpr: peak used % per instance+volume (logical disk).
func windowsFilesystemUsedPeakExpr(selComma, lookback, step string) string {
	var inner string
	if selComma == "" {
		inner = `max by (instance, volume) (100 - 100 * (windows_logical_disk_free_bytes / clamp_min(windows_logical_disk_size_bytes, 1)))`
	} else {
		s := strings.TrimPrefix(selComma, ",")
		inner = fmt.Sprintf(
			`max by (instance, volume) (100 - 100 * (windows_logical_disk_free_bytes{%s} / clamp_min(windows_logical_disk_size_bytes{%s}, 1)))`,
			s, s,
		)
	}
	return fmt.Sprintf(`max_over_time((%s)[%s:%s])`, inner, lookback, step)
}

// nodeDiskBusyPeakExpr: per instance, max over disks of disk IO time utilization % (0–100).
func nodeDiskBusyPeakExpr(selComma, rateWin, lookback, step string) string {
	var inner string
	if selComma == "" {
		inner = fmt.Sprintf(
			`max by (instance) (100 * clamp_max(rate(node_disk_io_time_seconds_total{device!=""}[%s]), 1))`,
			rateWin,
		)
	} else {
		inner = fmt.Sprintf(
			`max by (instance) (100 * clamp_max(rate(node_disk_io_time_seconds_total{device!=""%s}[%s]), 1))`,
			selComma, rateWin,
		)
	}
	return fmt.Sprintf(`max_over_time((%s)[%s:%s])`, inner, lookback, step)
}

// nodeNetworkErrorsPeakExpr: sum of receive/transmit errs and receive drops per instance (excluding loopback).
func nodeNetworkErrorsPeakExpr(selComma, rateWin, lookback, step string) string {
	lo := `device!~"^lo$"`
	inner := fmt.Sprintf(`sum by (instance) (
  rate(node_network_receive_errs_total{%[1]s%[2]s}[%[3]s]) +
  rate(node_network_transmit_errs_total{%[1]s%[2]s}[%[3]s]) +
  rate(node_network_receive_drop_total{%[1]s%[2]s}[%[3]s])
)`, lo, selComma, rateWin)
	if selComma == "" {
		inner = fmt.Sprintf(`sum by (instance) (
  rate(node_network_receive_errs_total{%s}[%s]) +
  rate(node_network_transmit_errs_total{%s}[%s]) +
  rate(node_network_receive_drop_total{%s}[%s])
)`, lo, rateWin, lo, rateWin, lo, rateWin)
	}
	return fmt.Sprintf(`max_over_time((%s)[%s:%s])`, inner, lookback, step)
}

// nodeNetworkTotalMbpsPeakExpr: RX+TX bytes/sec summed over non-lo interfaces → Mbps.
func nodeNetworkTotalMbpsPeakExpr(selComma, rateWin, lookback, step string) string {
	lo := `device!~"^lo$"`
	inner := fmt.Sprintf(`sum by (instance) (
  rate(node_network_receive_bytes_total{%[1]s%[2]s}[%[3]s]) +
  rate(node_network_transmit_bytes_total{%[1]s%[2]s}[%[3]s])
) * 8 / 1e6`, lo, selComma, rateWin)
	if selComma == "" {
		inner = fmt.Sprintf(`sum by (instance) (
  rate(node_network_receive_bytes_total{%s}[%s]) +
  rate(node_network_transmit_bytes_total{%s}[%s])
) * 8 / 1e6`, lo, rateWin, lo, rateWin)
	}
	return fmt.Sprintf(`max_over_time((%s)[%s:%s])`, inner, lookback, step)
}

func nodeLoadPerCorePeakExpr(selComma, lookback, step string) string {
	s := strings.TrimPrefix(selComma, ",")
	var inner string
	if s == "" {
		inner = `node_load1 / count without (cpu, mode) (node_cpu_seconds_total{mode="idle"})`
	} else {
		inner = fmt.Sprintf(
			`node_load1{%s} / count without (cpu, mode) (node_cpu_seconds_total{mode="idle",%s})`,
			s, s,
		)
	}
	return fmt.Sprintf(`max_over_time((%s)[%s:%s])`, inner, lookback, step)
}

// nodePSIPeakExpr: max stall rate across CPU/memory/IO PSI "some" counters (requires CONFIG_PSI).
func nodePSIPeakExpr(selComma, rateWin, lookback, step string) string {
	s := strings.TrimPrefix(selComma, ",")
	r := func(metric string) string {
		if s == "" {
			return fmt.Sprintf(`rate(%s[%s])`, metric, rateWin)
		}
		return fmt.Sprintf(`rate(%s{%s}[%s])`, metric, s, rateWin)
	}
	inner := fmt.Sprintf(
		`max by (instance) (max(%s, %s, %s))`,
		r("node_pressure_cpu_waiting_seconds_total"),
		r("node_pressure_memory_waiting_seconds_total"),
		r("node_pressure_io_waiting_seconds_total"),
	)
	return fmt.Sprintf(`max_over_time((%s)[%s:%s])`, inner, lookback, step)
}

func nodeFilesystemInodeUsedPeakExpr(selComma, lookback, step string) string {
	nodeFS := `mountpoint!="",fstype!~"tmpfs|proc|sysfs|devtmpfs|overlay|squashfs|rpc_pipefs|autofs|binfmt_misc"`
	var inner string
	if selComma == "" {
		inner = fmt.Sprintf(
			`max by (instance, mountpoint) (100 * (1 - node_filesystem_files_free{%s} / clamp_min(node_filesystem_files{%s}, 1)))`,
			nodeFS, nodeFS,
		)
	} else {
		inner = fmt.Sprintf(
			`max by (instance, mountpoint) (100 * (1 - node_filesystem_files_free{%s%s} / clamp_min(node_filesystem_files{%s%s}, 1)))`,
			nodeFS, selComma, nodeFS, selComma,
		)
	}
	return fmt.Sprintf(`max_over_time((%s)[%s:%s])`, inner, lookback, step)
}

func nodeFileDescriptorPercentPeakExpr(selComma, lookback, step string) string {
	var inner string
	if selComma == "" {
		inner = `100 * (node_filefd_allocated / clamp_min(node_filefd_maximum, 1))`
	} else {
		s := strings.TrimPrefix(selComma, ",")
		inner = fmt.Sprintf(
			`100 * (node_filefd_allocated{%s} / clamp_min(node_filefd_maximum{%s}, 1))`,
			s, s,
		)
	}
	return fmt.Sprintf(`max_over_time((%s)[%s:%s])`, inner, lookback, step)
}

func nodeConntrackPercentPeakExpr(selComma, lookback, step string) string {
	var inner string
	if selComma == "" {
		inner = `100 * (node_nf_conntrack_entries / clamp_min(node_nf_conntrack_entries_limit, 1))`
	} else {
		s := strings.TrimPrefix(selComma, ",")
		inner = fmt.Sprintf(
			`100 * (node_nf_conntrack_entries{%s} / clamp_min(node_nf_conntrack_entries_limit{%s}, 1))`,
			s, s,
		)
	}
	return fmt.Sprintf(`max_over_time((%s)[%s:%s])`, inner, lookback, step)
}

func nodeTCPRetransPeakExpr(selComma, rateWin, lookback, step string) string {
	var inner string
	if selComma == "" {
		inner = fmt.Sprintf(`sum by (instance) (rate(node_netstat_Tcp_RetransSegs[%s]))`, rateWin)
	} else {
		inner = fmt.Sprintf(
			`sum by (instance) (rate(node_netstat_Tcp_RetransSegs{%s}[%s]))`,
			strings.TrimPrefix(selComma, ","), rateWin,
		)
	}
	return fmt.Sprintf(`max_over_time((%s)[%s:%s])`, inner, lookback, step)
}

func nodeSoftnetDropsPeakExpr(selComma, rateWin, lookback, step string) string {
	var inner string
	if selComma == "" {
		inner = fmt.Sprintf(`sum by (instance) (rate(node_softnet_dropped_total[%s]))`, rateWin)
	} else {
		inner = fmt.Sprintf(
			`sum by (instance) (rate(node_softnet_dropped_total{%s}[%s]))`,
			strings.TrimPrefix(selComma, ","), rateWin,
		)
	}
	return fmt.Sprintf(`max_over_time((%s)[%s:%s])`, inner, lookback, step)
}

func windowsDiskBusyPeakExpr(selComma, rateWin, lookback, step string) string {
	var inner string
	if selComma == "" {
		inner = fmt.Sprintf(
			`max by (instance) (100 * clamp_max(1 - rate(windows_physical_disk_idle_seconds_total{disk!=""}[%s]), 1))`,
			rateWin,
		)
	} else {
		inner = fmt.Sprintf(
			`max by (instance) (100 * clamp_max(1 - rate(windows_physical_disk_idle_seconds_total{disk!=""%s}[%s]), 1))`,
			selComma, rateWin,
		)
	}
	return fmt.Sprintf(`max_over_time((%s)[%s:%s])`, inner, lookback, step)
}

func windowsNetworkErrorsPeakExpr(selComma, rateWin, lookback, step, nicExcludeRE string) string {
	lbl := windowsNetLbl(selComma, nicExcludeRE)
	inner := fmt.Sprintf(`sum by (instance) (
  rate(windows_net_packets_outbound_errors_total%s[%s]) +
  rate(windows_net_packets_received_errors_total%s[%s]) +
  rate(windows_net_packets_outbound_discarded_total%s[%s]) +
  rate(windows_net_packets_received_discarded_total%s[%s])
)`, lbl, rateWin, lbl, rateWin, lbl, rateWin, lbl, rateWin)
	return fmt.Sprintf(`max_over_time((%s)[%s:%s])`, inner, lookback, step)
}

func windowsNetworkTotalMbpsPeakExpr(selComma, rateWin, lookback, step, nicExcludeRE string) string {
	lbl := windowsNetLbl(selComma, nicExcludeRE)
	inner := fmt.Sprintf(`sum by (instance) (
  rate(windows_net_bytes_received_total%s[%s]) +
  rate(windows_net_bytes_sent_total%s[%s])
) * 8 / 1e6`, lbl, rateWin, lbl, rateWin)
	return fmt.Sprintf(`max_over_time((%s)[%s:%s])`, inner, lookback, step)
}

func windowsNetworkLinkPercentPeakExpr(selComma, rateWin, lookback, step, nicExcludeRE string) string {
	lbl := windowsNetLbl(selComma, nicExcludeRE)
	inner := fmt.Sprintf(`max by (instance) (
  max by (instance, nic) (
    (rate(windows_net_bytes_total%s[%s]) * 8) / clamp_min(windows_net_current_bandwidth_bytes%s, 1) * 100
  )
)`, lbl, rateWin, lbl)
	return fmt.Sprintf(`max_over_time((%s)[%s:%s])`, inner, lookback, step)
}

func windowsLogicalDiskTotalMbpsPeakExpr(selComma, rateWin, lookback, step string) string {
	vol := `volume!="_Total"`
	var inner string
	if selComma == "" {
		inner = fmt.Sprintf(`max by (instance) (
  max by (instance, volume) (
    (rate(windows_logical_disk_read_bytes_total{%[1]s}[%[2]s]) + rate(windows_logical_disk_write_bytes_total{%[1]s}[%[2]s])) * 8 / 1e6
  )
)`, vol, rateWin)
	} else {
		s := strings.TrimPrefix(selComma, ",")
		inner = fmt.Sprintf(`max by (instance) (
  max by (instance, volume) (
    (rate(windows_logical_disk_read_bytes_total{%[1]s,%[2]s}[%[3]s]) + rate(windows_logical_disk_write_bytes_total{%[1]s,%[2]s}[%[3]s])) * 8 / 1e6
  )
)`, vol, s, rateWin)
	}
	return fmt.Sprintf(`max_over_time((%s)[%s:%s])`, inner, lookback, step)
}

func nodeSwapUsedPeakExpr(selComma, lookback, step string) string {
	var inner string
	if selComma == "" {
		inner = `100 * (1 - ((node_memory_SwapFree_bytes + 1) / (node_memory_SwapTotal_bytes + 1)))`
	} else {
		s := strings.TrimPrefix(selComma, ",")
		inner = fmt.Sprintf(
			`100 * (1 - ((node_memory_SwapFree_bytes{%[1]s} + 1) / (node_memory_SwapTotal_bytes{%[1]s} + 1)))`,
			s,
		)
	}
	return fmt.Sprintf(`max_over_time((%s)[%s:%s])`, inner, lookback, step)
}

func nodeCPUIOWaitPeakExpr(selComma, rateWin, lookback, step string) string {
	var inner string
	if selComma == "" {
		inner = fmt.Sprintf(
			`100 * avg by (instance) (rate(node_cpu_seconds_total{mode="iowait"}[%s]))`,
			rateWin,
		)
	} else {
		inner = fmt.Sprintf(
			`100 * avg by (instance) (rate(node_cpu_seconds_total{mode="iowait"%s}[%s]))`,
			selComma, rateWin,
		)
	}
	return fmt.Sprintf(`max_over_time((%s)[%s:%s])`, inner, lookback, step)
}

func nodeDiskReadMBpsPeakExpr(selComma, rateWin, lookback, step string) string {
	var inner string
	if selComma == "" {
		inner = fmt.Sprintf(
			`max by (instance) (rate(node_disk_read_bytes_total{device!=""}[%s]) / 1e6)`,
			rateWin,
		)
	} else {
		inner = fmt.Sprintf(
			`max by (instance) (rate(node_disk_read_bytes_total{device!=""%s}[%s]) / 1e6)`,
			selComma, rateWin,
		)
	}
	return fmt.Sprintf(`max_over_time((%s)[%s:%s])`, inner, lookback, step)
}

func nodeDiskWriteMBpsPeakExpr(selComma, rateWin, lookback, step string) string {
	var inner string
	if selComma == "" {
		inner = fmt.Sprintf(
			`max by (instance) (rate(node_disk_written_bytes_total{device!=""}[%s]) / 1e6)`,
			rateWin,
		)
	} else {
		inner = fmt.Sprintf(
			`max by (instance) (rate(node_disk_written_bytes_total{device!=""%s}[%s]) / 1e6)`,
			selComma, rateWin,
		)
	}
	return fmt.Sprintf(`max_over_time((%s)[%s:%s])`, inner, lookback, step)
}

func nodeTCPListenDropsPeakExpr(selComma, rateWin, lookback, step string) string {
	var inner string
	if selComma == "" {
		inner = fmt.Sprintf(`sum by (instance) (rate(node_netstat_TcpExt_ListenDrops[%s]))`, rateWin)
	} else {
		inner = fmt.Sprintf(
			`sum by (instance) (rate(node_netstat_TcpExt_ListenDrops{%s}[%s]))`,
			strings.TrimPrefix(selComma, ","), rateWin,
		)
	}
	return fmt.Sprintf(`max_over_time((%s)[%s:%s])`, inner, lookback, step)
}

func nodeTCPTimewaitPeakExpr(selComma, lookback, step string) string {
	var inner string
	if selComma == "" {
		inner = `node_sockstat_TCP_tw`
	} else {
		inner = fmt.Sprintf(`node_sockstat_TCP_tw{%s}`, strings.TrimPrefix(selComma, ","))
	}
	return fmt.Sprintf(`max_over_time((%s)[%s:%s])`, inner, lookback, step)
}

func nodeTCPEstablishedPeakExpr(selComma, lookback, step string) string {
	var inner string
	if selComma == "" {
		inner = `node_netstat_Tcp_CurrEstab`
	} else {
		inner = fmt.Sprintf(`node_netstat_Tcp_CurrEstab{%s}`, strings.TrimPrefix(selComma, ","))
	}
	return fmt.Sprintf(`max_over_time((%s)[%s:%s])`, inner, lookback, step)
}

func validateDuration(s string) error {
	// Prometheus durations: ms s m h d w y
	if s == "" {
		return fmt.Errorf("empty duration")
	}
	for _, allowed := range []string{
		"30m", "1h", "2h", "3h", "6h", "12h", "24h", "48h", "72h", "168h", "7d",
		"15m", "45m", "4h", "18h", "36h",
	} {
		if s == allowed {
			return nil
		}
	}
	// allow generic short forms: number + unit
	for _, u := range []string{"ms", "s", "m", "h", "d", "w", "y"} {
		if strings.HasSuffix(s, u) {
			num := strings.TrimSuffix(s, u)
			if num == "" {
				continue
			}
			if _, err := strconv.ParseFloat(num, 64); err == nil {
				return nil
			}
		}
	}
	return fmt.Errorf("lookback %q not allowed or invalid; use e.g. 3h, 24h, 30m", s)
}

func (d *Deps) metricPresent(selector string) (bool, error) {
	q := url.Values{}
	q.Set("query", "count("+selector+") > 0")
	data, err := d.Prom.MakePrometheusRequest("query", q)
	if err != nil {
		return false, err
	}
	samples, err := parseInstantVectorData(data)
	if err != nil {
		return false, err
	}
	for _, s := range samples {
		if s.Value > 0 {
			return true, nil
		}
	}
	return false, nil
}

func (d *Deps) promInstantVector(query string) ([]promSample, error) {
	q := url.Values{}
	q.Set("query", query)
	data, err := d.Prom.MakePrometheusRequest("query", q)
	if err != nil {
		return nil, err
	}
	return parseInstantVectorData(data)
}

type promSample struct {
	Metric map[string]string
	Value  float64
}

func parseInstantVectorData(data any) ([]promSample, error) {
	m, ok := data.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected query data shape")
	}
	rt, _ := m["resultType"].(string)
	if rt == "scalar" {
		if arr, ok := m["result"].([]any); ok && len(arr) >= 2 {
			v, err := promValueToFloat(arr[1])
			if err != nil {
				return nil, err
			}
			return []promSample{{Metric: map[string]string{}, Value: v}}, nil
		}
		return nil, fmt.Errorf("invalid scalar result")
	}
	raw, _ := m["result"].([]any)
	if rt != "vector" {
		if rt == "" && raw == nil {
			return nil, fmt.Errorf("empty query result")
		}
	}
	out := make([]promSample, 0, len(raw))
	for _, item := range raw {
		row, ok := item.(map[string]any)
		if !ok {
			continue
		}
		metric := map[string]string{}
		if mm, ok := row["metric"].(map[string]any); ok {
			for k, v := range mm {
				if s, ok := v.(string); ok {
					metric[k] = s
				}
			}
		}
		val := 0.0
		if va, ok := row["value"].([]any); ok && len(va) >= 2 {
			f, err := promValueToFloat(va[1])
			if err != nil {
				continue
			}
			val = f
		}
		out = append(out, promSample{Metric: metric, Value: val})
	}
	return out, nil
}

func promValueToFloat(v any) (float64, error) {
	switch n := v.(type) {
	case string:
		return strconv.ParseFloat(n, 64)
	case float64:
		return n, nil
	default:
		return 0, fmt.Errorf("unsupported sample value type %T", v)
	}
}

func filterBreaches(samples []promSample, threshold float64) []map[string]any {
	return filterBreachesNamed(samples, threshold, "peak_percent")
}

func filterBreachesNamed(samples []promSample, threshold float64, valueKey string) []map[string]any {
	var hits []map[string]any
	for _, s := range samples {
		if s.Value >= threshold {
			hits = append(hits, map[string]any{
				"labels": s.Metric,
				valueKey: s.Value,
			})
		}
	}
	sort.Slice(hits, func(i, j int) bool {
		return hits[i][valueKey].(float64) > hits[j][valueKey].(float64)
	})
	return hits
}

func samplesToMaps(samples []promSample) []map[string]any {
	return samplesToMapsNamed(samples, "peak_percent")
}

func samplesToMapsNamed(samples []promSample, valueKey string) []map[string]any {
	out := make([]map[string]any, 0, len(samples))
	for _, s := range samples {
		out = append(out, map[string]any{"labels": s.Metric, valueKey: s.Value})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i][valueKey].(float64) > out[j][valueKey].(float64)
	})
	return out
}

func topNUnder(samples []promSample, threshold float64, n int) []map[string]any {
	var under []promSample
	for _, s := range samples {
		if s.Value < threshold {
			under = append(under, s)
		}
	}
	sort.Slice(under, func(i, j int) bool {
		return under[i].Value > under[j].Value
	})
	if len(under) > n {
		under = under[:n]
	}
	out := make([]map[string]any, 0, len(under))
	for _, s := range under {
		out = append(out, map[string]any{"labels": s.Metric, "peak_percent": s.Value})
	}
	return out
}
