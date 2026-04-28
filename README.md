# Prometheus MCP Server

[![GitHub Container Registry](https://img.shields.io/badge/ghcr.io-pab1it0%2Fprometheus--mcp--server-blue?logo=docker)](https://github.com/users/pab1it0/packages/container/package/prometheus-mcp-server)
[![Helm Chart](https://img.shields.io/badge/helm%20chart-ghcr.io-blue?logo=helm)](https://github.com/pab1it0/prometheus-mcp-server/pkgs/container/charts%2Fprometheus-mcp-server)
[![GitHub Release](https://img.shields.io/github/v/release/pab1it0/prometheus-mcp-server)](https://github.com/pab1it0/prometheus-mcp-server/releases)
[![Codecov](https://codecov.io/gh/pab1it0/prometheus-mcp-server/branch/main/graph/badge.svg)](https://codecov.io/gh/pab1it0/prometheus-mcp-server)
![Go](https://img.shields.io/badge/go-1.25%2B-00ADD8)
[![License](https://img.shields.io/github/license/pab1it0/prometheus-mcp-server)](https://github.com/pab1it0/prometheus-mcp-server/blob/main/LICENSE)

Give AI assistants the power to query metrics over a **Prometheus-compatible HTTP API**—including **[VictoriaMetrics][vm] open source** on-premises (typically `vmselect` URLs such as `http://vmselect:8481/select/0/prometheus`) and plain Prometheus.

A [Model Context Protocol][mcp] (MCP) server that exposes that API to MCP clients so assistants can run PromQL/MetricsQL queries (VictoriaMetrics accepts both), list metrics, read metadata, and inspect targets where the backend exposes them.

[vm]: https://docs.victoriametrics.com/

[mcp]: https://modelcontextprotocol.io

## Getting Started

### Prerequisites

- A reachable **Prometheus-compatible** query endpoint (Prometheus, or VictoriaMetrics `vmselect` with the `/select/.../prometheus` path your deployment uses)
- MCP-compatible client (Claude Desktop, VS Code, Cursor, Windsurf, etc.)

### Installation Methods

<details>
<summary><b>Claude Desktop</b></summary>

Add to your Claude Desktop configuration:

```json
{
  "mcpServers": {
    "prometheus": {
      "command": "docker",
      "args": [
        "run",
        "-i",
        "--rm",
        "-e",
        "PROMETHEUS_URL",
        "ghcr.io/pab1it0/prometheus-mcp-server:latest"
      ],
      "env": {
        "PROMETHEUS_URL": "<your-prometheus-url>"
      }
    }
  }
}
```
</details>

<details>
<summary><b>Claude Code</b></summary>

Install via the Claude Code CLI:

```bash
claude mcp add prometheus --env PROMETHEUS_URL=http://your-prometheus:9090 -- docker run -i --rm -e PROMETHEUS_URL ghcr.io/pab1it0/prometheus-mcp-server:latest
```
</details>

<details>
<summary><b>VS Code / Cursor / Windsurf</b></summary>

Add to your MCP settings in the respective IDE:

```json
{
  "prometheus": {
    "command": "docker",
    "args": [
      "run",
      "-i",
      "--rm",
      "-e",
      "PROMETHEUS_URL",
      "ghcr.io/pab1it0/prometheus-mcp-server:latest"
    ],
    "env": {
      "PROMETHEUS_URL": "<your-prometheus-url>"
    }
  }
}
```
</details>

<details>
<summary><b>Docker Desktop</b></summary>

The easiest way to run the Prometheus MCP server is through Docker Desktop:

<a href="https://hub.docker.com/open-desktop?url=https://open.docker.com/dashboard/mcp/servers/id/prometheus/config?enable=true">
  <img src="https://img.shields.io/badge/+%20Add%20to-Docker%20Desktop-2496ED?style=for-the-badge&logo=docker&logoColor=white" alt="Add to Docker Desktop" />
</a>

1. **Via MCP Catalog**: Visit the [Prometheus MCP Server on Docker Hub](https://hub.docker.com/mcp/server/prometheus/overview) and click the button above

2. **Via MCP Toolkit**: Use Docker Desktop's MCP Toolkit extension to discover and install the server

3. Configure your connection using environment variables (see Configuration Options below)

</details>

<details>
<summary><b>Manual Docker Setup</b></summary>

Run directly with Docker:

```bash
# With environment variables
docker run -i --rm \
  -e PROMETHEUS_URL="http://your-prometheus:9090" \
  ghcr.io/pab1it0/prometheus-mcp-server:latest

# With authentication
docker run -i --rm \
  -e PROMETHEUS_URL="http://your-prometheus:9090" \
  -e PROMETHEUS_USERNAME="admin" \
  -e PROMETHEUS_PASSWORD="password" \
  ghcr.io/pab1it0/prometheus-mcp-server:latest
```
</details>

<details>
<summary><b>Helm Chart (Kubernetes)</b></summary>

Deploy to Kubernetes using the Helm chart from the OCI registry:

```bash
helm install prometheus-mcp-server \
  oci://ghcr.io/pab1it0/charts/prometheus-mcp-server \
  --version 1.0.0 \
  --set prometheus.url="http://prometheus:9090"
```

With authentication:

```bash
helm install prometheus-mcp-server \
  oci://ghcr.io/pab1it0/charts/prometheus-mcp-server \
  --version 1.0.0 \
  --set prometheus.url="http://prometheus:9090" \
  --set auth.username="admin" \
  --set auth.password="secret"
```

With a custom values file:

```bash
helm install prometheus-mcp-server \
  oci://ghcr.io/pab1it0/charts/prometheus-mcp-server \
  --version 1.0.0 \
  -f values.yaml
```

See the [chart values](charts/prometheus-mcp-server/values.yaml) for all available configuration options.
</details>

### Configuration Options

| Variable | Description | Required |
|----------|-------------|----------|
| `PROMETHEUS_URL` | Base URL for the Prometheus HTTP API (`/api/v1/...`). Use this **or** one of the VictoriaMetrics variables below | One of URL vars |
| `VICTORIAMETRICS_URL` | Same as `PROMETHEUS_URL` for VictoriaMetrics on-prem (e.g. vmselect `http://host:8481/select/0/prometheus`). Ignored if `PROMETHEUS_URL` is set | One of URL vars |
| `VM_SELECT_URL` | Third alias used only if both above are empty | One of URL vars |
| `PROMETHEUS_URL_SSL_VERIFY` | Set to False to disable SSL verification | No |
| `PROMETHEUS_DISABLE_LINKS` | Set to True to disable Prometheus UI links in query results (saves context tokens) | No |
| `PROMETHEUS_REQUEST_TIMEOUT` | Request timeout in seconds to prevent hanging requests (DDoS protection) | No (default: 30) |
| `PROMETHEUS_USERNAME` | Username for basic authentication | No |
| `PROMETHEUS_PASSWORD` | Password for basic authentication | No |
| `PROMETHEUS_TOKEN` | Bearer token for authentication | No |
| `PROMETHEUS_CLIENT_CERT` | Path to client certificate file for mutual TLS authentication | No |
| `PROMETHEUS_CLIENT_KEY` | Path to client private key file for mutual TLS authentication | No |
| `REQUESTS_CA_BUNDLE` | Path to CA bundle file for verifying the server's TLS certificate (standard `requests` library env var) | No |
| `ORG_ID` | Sent as `X-Scope-OrgID` (VictoriaMetrics cluster multi-tenancy, or other proxies that expect it) | No |
| `VMALERT_URL` | Base URL for vmalert when rules/alerts are not under `{PROMETHEUS_URL}/vmalert` (cluster default adds `/vmalert` under `/select/.../prometheus` automatically) | No |
| `PROMETHEUS_EXPORT_MAX_BYTES` | Max bytes read for `/api/v1/export` responses (default 2097152) | No |
| `PROMETHEUS_MCP_SERVER_TRANSPORT` | Transport mode (stdio, http, sse) | No (default: stdio) |
| `PROMETHEUS_MCP_BIND_HOST` | Host for HTTP transport | No (default: 127.0.0.1) |
| `PROMETHEUS_MCP_BIND_PORT` | Port for HTTP transport | No (default: 8052) |
| `PROMETHEUS_CUSTOM_HEADERS` | Custom headers as JSON string | No |
| `TOOL_PREFIX` | Prefix for all tool names (e.g., `staging` results in `staging_execute_query`). Useful for running multiple instances targeting different environments in Cursor | No |

## Available Tools

Core tools match the original Prometheus MCP server. Additional tools mirror **[VictoriaMetrics mcp-victoriametrics](https://github.com/VictoriaMetrics/mcp-victoriametrics)** where the backend exposes the same HTTP paths (open source / on‑prem). **Not included:** VictoriaMetrics Cloud-only tools (`tenants`, `deployments`, `access_tokens`, …) and in-process **`test_rules`** (vmalert-tool); use `vmalert-tool` locally if you need that.

| Tool | Category | Description |
| --- | --- | --- |
| `health_check` | System | Health check and connectivity probe |
| `execute_query` | Query | Instant query |
| `execute_range_query` | Query | Range query |
| `list_metrics` | Discovery | Metric names (cached helper over `__name__` values) |
| `get_metric_metadata` | Discovery | `/api/v1/metadata` |
| `get_targets` | Discovery | `/api/v1/targets` |
| `series` | Discovery | `/api/v1/series` |
| `labels` | Discovery | `/api/v1/labels` |
| `label_values` | Discovery | `/api/v1/label/{name}/values` |
| `metrics` | Discovery | `__name__` values with match/start/end/limit (VM-style) |
| `rules` | Alerting | `/api/v1/rules` (tries vmalert prefix first, then Prometheus URL) |
| `alerts` | Alerting | `/api/v1/alerts` with optional state/group/limit/offset filtering |
| `export` | Data | `/api/v1/export` or `/api/v1/export/csv` (raw body, size-capped) |
| `tsdb_status` | Ops | `/api/v1/status/tsdb` |
| `top_queries` | Ops | `/api/v1/status/top_queries` |
| `metric_statistics` | Ops | `/api/v1/status/metric_names_stats` (VictoriaMetrics) |
| `active_queries` | Ops | `/api/v1/status/active_queries` |
| `get_build_info` | Ops | `/api/v1/status/buildinfo` |
| `get_runtime_info` | Ops | `/api/v1/status/runtimeinfo` |
| `get_config_yaml` | Ops | `/api/v1/status/config` |
| `get_flags` | Ops | `/api/v1/status/flags`, or vmselect `/flags` text fallback |
| `prettify_query` | Query helpers | `metricsql` library or `prettify-query` HTTP |
| `explain_query` | Query helpers | Parse via `metricsql` + canonical form |
| `documentation` | Docs | Links to MetricsQL / Prometheus API docs |
| `metric_relabel_debug` | Debug | `/metric-relabel-debug` (VictoriaMetrics) |
| `retention_filters_debug` | Debug | `/retention-filters-debug` |
| `downsampling_filters_debug` | Debug | `/downsampling-filters-debug` |
| `operator_host_resource_report` | Operations | **SRE runbook:** CPU, memory, filesystem (space + Linux inode %), disk busy %, network errors + throughput. **Linux:** load/core, PSI, **swap %**, **CPU iowait %**, **disk read/write MB/s**, **TCP listen drops**, **time-wait / established** peaks, filefd, conntrack, retrans, softnet. **Windows:** physical + optional **logical-disk** throughput, **NIC exclude** (default isatap/VPN), optional **link utilization %** vs `windows_net_current_bandwidth_bytes`. **`include_all_standard_metrics`** enables all optional blocks. **K8s objects** need other scrapes—see below. |
| `operator_process_exporter_report` | Operations | **Linux:** [process-exporter](https://github.com/ncabatoff/process-exporter) `namedprocess_namegroup_*` — peak **CPU per group** (~cores), **RSS**, **num_procs**, **worst FD ratio**, **zombie** thread counts, **scrape** error rates. Use **`label_selector`** (e.g. `job="process-exporter"`). **`include_all`** turns on every block. Complements host-level `operator_host_resource_report`. |

Some endpoints are VictoriaMetrics-specific or Enterprise-only; callers may receive HTTP errors on plain Prometheus.

**Why `operator_host_resource_report` exists:** generic `execute_query` is not enough for on-call work if nobody remembers PromQL. This tool encodes proven recipes (with `max_over_time` subqueries), discovers common agent metric families, and returns **structured breach lists** for “hosts over 80% CPU/memory in the last 3h/24h” style questions—similar in spirit to other Prometheus MCP servers that ship **SRE “golden signal” helpers** instead of only raw query passthrough.

**Kubernetes + node_exporter:** metrics from the **node exporter DaemonSet** describe the **node’s Linux** (CPU, memory, disks, NICs on that VM/bare metal). They do **not** replace **kube-state-metrics** (object state: pod phase, deployment replicas, PVC bound), **cAdvisor/kubelet** (container CPU/mem throttle, OOM), or **API-server / etcd** health. Point the MCP at the same VictoriaMetrics where you already store those series, and use **`execute_query`** (or add more curated tools later) for `kube_*`, `container_*`, etc.

**process-exporter:** **`operator_process_exporter_report`** is for **application/process groups** you configure in process-exporter (not every host will have series). Use it together with **`operator_host_resource_report`** when triaging “which JVM/pg/redis group is eating the node.”

**Possible extensions** (not all implemented): **RAID / SMART** if you export them; **predict_linear** on free space for “full in N days”; **goldilocks**-style right-sizing from container requests vs usage.

Use a `TOOL_PREFIX` if you run multiple MCP server instances and need unique tool names per environment.

### Example prompts (plain English)

In Cursor, Claude Desktop, or any MCP client, you describe intent in natural language; the assistant picks tools and fills arguments. Examples:

| You might say | Tools the assistant will often use |
| --- | --- |
| “Is Prometheus / VictoriaMetrics reachable and can you run a simple query?” | `health_check`, maybe `execute_query` with `up` |
| “What’s the current value of `process_resident_memory_bytes` for `job="api"`?” | `execute_query` (instant PromQL/MetricsQL) |
| “Plot request rate over the last 24 hours with a 5-minute step.” | `execute_range_query` with `query`, `start`, `end`, `step` |
| “List metrics whose names look like `http`.” | `list_metrics` with `filter_pattern`, or `metrics` / `label_values` on `__name__` |
| “What label names exist for series matching `{job="node"}`?” | `labels` with optional `match` |
| “What are the values of the `instance` label for `up`?” | `label_values` with `label_name`: `instance`, `match`: `up` |
| “Show me series that match `{__name__=~"container_.*"}`.” | `series` with `match` (and optional `start` / `end` / `limit`) |
| “What metadata do we have for `node_cpu_seconds_total`?” | `get_metric_metadata` |
| “Which scrape targets are up or down?” | `get_targets`, often plus `execute_query` on `up` |
| “What rules and alerts are defined? Anything firing?” | `rules`, `alerts` (e.g. filter `state`: `firing`) |
| “How heavy is cardinality / what dominates TSDB?” | `tsdb_status` (VictoriaMetrics / newer Prometheus) |
| “What queries are slow or most frequent?” | `top_queries`, `active_queries`, `metric_statistics` (where supported) |
| “Pretty-print this query: `sum(rate(http_requests_total[5m])) by (job)`” | `prettify_query`, sometimes `explain_query` |
| “Give me doc links for MetricsQL vs Prometheus HTTP API.” | `documentation` |
| “Export raw samples for `{job="prometheus"}` as JSON lines.” | `export` with `match`, `format`: `json` or `csv` |
| “Which hosts hit **≥80% CPU** or **≥80% memory used** in the **last 24 hours**? Show peaks.” | `operator_host_resource_report` with `lookback`: `24h`, thresholds optional |
| “Who pegged **disk** or had **network errors / high traffic**?” | Same tool with `include_disk_io` / `include_network`, or `include_all_standard_metrics`: true; optional `network_total_mbps_threshold` (Linux) or `windows_network_total_mbps_threshold` / `network_total_mbps_threshold` (Windows) for peak RX+TX Mbps |
| “Which **process groups** (process-exporter) used the most **CPU / RSS** or have **zombies / FD pressure**?” | `operator_process_exporter_report` with `label_selector` for the exporter job, `include_all`: true or individual `include_*` flags |

If you set `TOOL_PREFIX`, the same flows apply but tool names become e.g. `prod_execute_query` instead of `execute_query`.

### Example tool arguments (reference)

Illustrative JSON for `tools/call`, API debugging, or pasting into MCP inspector clients. Natural-language chat normally fills these for you. If you use **`TOOL_PREFIX`**, prefix every `name` (e.g. `prod_execute_query`).

#### Instant and range queries

```json
{
  "name": "execute_query",
  "arguments": {
    "query": "sum(rate(http_requests_total[5m])) by (job)"
  }
}
```

```json
{
  "name": "execute_range_query",
  "arguments": {
    "query": "up",
    "start": "2026-04-26T00:00:00Z",
    "end": "2026-04-27T00:00:00Z",
    "step": "5m"
  }
}
```

#### Discovery: series and labels

```json
{
  "name": "series",
  "arguments": {
    "match": "{job=\"prometheus\"}",
    "limit": 100
  }
}
```

```json
{
  "name": "label_values",
  "arguments": {
    "label_name": "instance",
    "match": "up"
  }
}
```

#### Alerts and rules

```json
{
  "name": "alerts",
  "arguments": {
    "state": "firing",
    "limit": 50
  }
}
```

```json
{
  "name": "rules",
  "arguments": {
    "type": "alert"
  }
}
```

#### Operations (VictoriaMetrics-friendly)

```json
{
  "name": "tsdb_status",
  "arguments": {
    "topN": 15,
    "focusLabel": "__name__"
  }
}
```

```json
{
  "name": "top_queries",
  "arguments": {
    "topN": 10,
    "maxLifetime": "10m"
  }
}
```

#### Query helpers

```json
{
  "name": "prettify_query",
  "arguments": {
    "query": "sum(rate(foo[5m]))by(job)"
  }
}
```

---

#### `operator_host_resource_report` — response shape (what to expect)

The tool returns JSON with **`metric_profile_used`** (`node_exporter` or `windows_exporter`), embedded **PromQL** strings (e.g. `promql_cpu_peak_percent`), **`*_breaches`** arrays (hosts or groups that crossed thresholds during the lookback), **`*_peak_by_instance`** (or by mount/volume) for ranking, and **`caveats`**. Optional blocks appear only when enabled (directly or via **`include_all_standard_metrics`**).

| Argument | Role |
| --- | --- |
| `lookback` | Window for `max_over_time` subqueries (e.g. `3h`, `24h`) |
| `subquery_step` | Resolution inside subquery (default `5m`) |
| `rate_window` | Inner `rate()` / `irate()` window (default `5m`) |
| `label_selector` | Extra matchers **without** outer braces, comma-separated, e.g. `job="node-exporter",env="prod"` |
| `metric_profile` | `auto` (probe metrics), `node_exporter`, or `windows_exporter` |
| `include_all_standard_metrics` | If `true`, enables every optional signal for the detected OS; when `false`, turn blocks on individually with `include_*` |

**Linux — minimal (CPU + memory only)**

```json
{
  "name": "operator_host_resource_report",
  "arguments": {
    "lookback": "24h",
    "metric_profile": "node_exporter",
    "label_selector": "job=\"node-exporter\"",
    "cpu_percent_threshold": 80,
    "memory_percent_threshold": 80
  }
}
```

**Linux — full standard signals (recommended for on-call)**

Single flag turns on filesystem, disk busy %, network errors + Mbps, load/core, PSI, inodes, file descriptors, conntrack, TCP retrans, softnet, swap, CPU iowait, disk throughput (MB/s), TCP listen drops, socket stats, etc.

```json
{
  "name": "operator_host_resource_report",
  "arguments": {
    "lookback": "6h",
    "metric_profile": "node_exporter",
    "label_selector": "job=\"node-exporter\"",
    "cpu_percent_threshold": 85,
    "memory_percent_threshold": 85,
    "include_all_standard_metrics": true,
    "filesystem_used_percent_threshold": 85,
    "disk_percent_threshold": 80,
    "network_errors_per_sec_threshold": 1,
    "network_total_mbps_threshold": 900,
    "load_per_core_threshold": 1.2,
    "psi_stall_rate_threshold": 0.3,
    "swap_used_percent_threshold": 50,
    "cpu_iowait_percent_threshold": 35,
    "disk_read_megabytes_per_sec_threshold": 400,
    "disk_write_megabytes_per_sec_threshold": 400,
    "tcp_listen_drops_per_sec_threshold": 0.1,
    "tcp_time_wait_high_threshold": 200000,
    "tcp_established_high_threshold": 50000
  }
}
```

Omit optional `*_threshold` keys you do not need; breaches for disk MB/s and TCP established/time-wait only appear when those thresholds are set.

**Linux — VictoriaMetrics / federation (`origin_prometheus` or extra labels)**

```json
{
  "name": "operator_host_resource_report",
  "arguments": {
    "lookback": "3h",
    "metric_profile": "node_exporter",
    "label_selector": "job=\"node\",origin_prometheus=\"eu-1\"",
    "include_all_standard_metrics": true
  }
}
```

**Windows — host saturation + logical disk + network**

```json
{
  "name": "operator_host_resource_report",
  "arguments": {
    "lookback": "24h",
    "metric_profile": "windows_exporter",
    "label_selector": "job=\"windows_exporter\"",
    "cpu_percent_threshold": 80,
    "memory_percent_threshold": 80,
    "include_all_standard_metrics": true,
    "windows_network_total_mbps_threshold": 800,
    "windows_logical_disk_total_mbps_threshold": 500,
    "windows_nic_exclude_regex": "isatap.*|VPN.*"
  }
}
```

Set **`windows_nic_exclude_regex`** to `""` to include all interfaces (default when omitted is still `isatap.*|VPN.*`). **`include_network_link_percent`** (on by default with `include_all_standard_metrics`) adds peak **link utilization %** vs `windows_net_current_bandwidth_bytes`.

**Windows — auto-detect profile**

```json
{
  "name": "operator_host_resource_report",
  "arguments": {
    "lookback": "12h",
    "metric_profile": "auto",
    "label_selector": "env=\"prod\"",
    "include_disk_io": true,
    "include_network": true
  }
}
```

**`network_total_mbps_threshold`** (Linux) and **`windows_network_total_mbps_threshold`** / **`network_total_mbps_threshold`** (Windows) are optional: when set, throughput breach lists use **peak** RX+TX Mbps over the lookback. That is **not** the same as NIC **utilization %**; use **`include_network_link_percent`** on Windows for utilization-style alerting when bandwidth metrics exist.

---

#### `operator_process_exporter_report` — process groups (Linux)

Requires **`namedprocess_namegroup_*`** series. Use **`label_selector`** to match the scrape job (and any other labels). If the probe finds no data, the tool returns **`error`** + **`hint`** instead of failing the MCP call.

**Full report (all blocks)**

```json
{
  "name": "operator_process_exporter_report",
  "arguments": {
    "lookback": "6h",
    "label_selector": "job=\"process-exporter\"",
    "include_all": true,
    "cpu_seconds_per_sec_threshold": 2,
    "memory_rss_bytes_threshold": 8589934592,
    "num_procs_high_threshold": 500,
    "worst_fd_ratio_threshold": 0.85,
    "zombie_count_high_threshold": 1,
    "scrape_errors_per_sec_threshold": 0.01
  }
}
```

**CPU-focused (cheap query set)**

```json
{
  "name": "operator_process_exporter_report",
  "arguments": {
    "lookback": "3h",
    "label_selector": "job=\"process-exporter\",cluster=\"prod\"",
    "include_cpu_per_group": true,
    "cpu_seconds_per_sec_threshold": 1.5
  }
}
```

**Peak CPU** is approximately **cores** used by each `groupname` on each `instance`. **RSS** / **num_procs** breaches only appear when you set the corresponding **`*_threshold`** fields.

## Features

- Execute queries against any Prometheus-compatible `/api/v1` implementation (Prometheus or VictoriaMetrics OSS)
- Discover and explore metrics
  - List available metrics
  - Get metadata for specific metrics
  - Search metric metadata by name or description in a single call
  - View instant query results
  - View range query results with different step intervals
- Authentication support
  - Basic auth from environment variables
  - Bearer token auth from environment variables
- Docker containerization support
- Provide interactive tools for AI assistants

## Development

Contributions are welcome! Please see our [Contributing Guide](CONTRIBUTING.md) for detailed information on how to get started, coding standards, and the pull request process.

Install [Go](https://go.dev/dl/) (1.25+ recommended; the module sets the minimum version).

```bash
go build -o prometheus-mcp-server ./cmd/prometheus-mcp-server
```

### Testing

Run the Go tests:

```bash
go test ./... -count=1
```

With coverage:

```bash
go test ./... -count=1 -coverprofile=coverage.out && go tool cover -func=coverage.out
```

When adding new features, please also add corresponding tests.

## License

MIT

---
