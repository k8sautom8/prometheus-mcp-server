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
| `operator_host_resource_report` | Operations | **Curated runbook:** peak **CPU** / **memory** / optional **filesystem** (space + Linux inode %), **disk I/O busy %** (Linux `node_disk_*`, Windows `windows_physical_disk_*`), **network** errors/drops + Mbps (`node_network_*` / `windows_net_*`). Linux extras: **load/core**, **PSI**, **file descriptors %**, **conntrack %**, **TCP retrans/sec**, **softnet drops/sec**. Set **`include_all_standard_metrics`** to turn on every optional block for the detected profile. **K8s cluster objects** need other scrapes—see below. |

Some endpoints are VictoriaMetrics-specific or Enterprise-only; callers may receive HTTP errors on plain Prometheus.

**Why `operator_host_resource_report` exists:** generic `execute_query` is not enough for on-call work if nobody remembers PromQL. This tool encodes proven recipes (with `max_over_time` subqueries), discovers common agent metric families, and returns **structured breach lists** for “hosts over 80% CPU/memory in the last 3h/24h” style questions—similar in spirit to other Prometheus MCP servers that ship **SRE “golden signal” helpers** instead of only raw query passthrough.

**Kubernetes + node_exporter:** metrics from the **node exporter DaemonSet** describe the **node’s Linux** (CPU, memory, disks, NICs on that VM/bare metal). They do **not** replace **kube-state-metrics** (object state: pod phase, deployment replicas, PVC bound), **cAdvisor/kubelet** (container CPU/mem throttle, OOM), or **API-server / etcd** health. Point the MCP at the same VictoriaMetrics where you already store those series, and use **`execute_query`** (or add more curated tools later) for `kube_*`, `container_*`, etc.

**Possible extensions** (not all implemented): **RAID / SMART** if you export them; **predict_linear** on free space for “full in N days”; **goldilocks**-style right-sizing from container requests vs usage; NIC **utilization %** on Windows via `windows_net_current_bandwidth_bytes` in custom PromQL.

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

If you set `TOOL_PREFIX`, the same flows apply but tool names become e.g. `prod_execute_query` instead of `execute_query`.

### Example tool arguments (reference)

Illustrative JSON for `tools/call` / debugging; your client normally builds this from chat.

**Instant query**

```json
{
  "name": "execute_query",
  "arguments": {
    "query": "sum(rate(http_requests_total[5m])) by (job)"
  }
}
```

**Range query**

```json
{
  "name": "execute_range_query",
  "arguments": {
    "query": "up",
    "start": "2025-04-26T00:00:00Z",
    "end": "2025-04-27T00:00:00Z",
    "step": "5m"
  }
}
```

**Series and labels**

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

**Alerts and rules**

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

**Operations (VictoriaMetrics-friendly)**

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

**Query helpers**

```json
{
  "name": "prettify_query",
  "arguments": {
    "query": "sum(rate(foo[5m]))by(job)"
  }
}
```

**Operator breach report (node_exporter example)**

```json
{
  "name": "operator_host_resource_report",
  "arguments": {
    "lookback": "24h",
    "cpu_percent_threshold": 80,
    "memory_percent_threshold": 80,
    "metric_profile": "auto",
    "label_selector": "job=\"node-exporter\"",
    "include_filesystem": true,
    "filesystem_used_percent_threshold": 85,
    "include_disk_io": true,
    "disk_percent_threshold": 80,
    "include_network": true,
    "network_errors_per_sec_threshold": 1,
    "network_total_mbps_threshold": 900,
    "include_all_standard_metrics": false
  }
}
```

`network_total_mbps_threshold` is optional; when set, `network_throughput_breaches` lists hosts whose **peak** combined RX+TX exceeded that value (Linux sums non-loopback interfaces; Windows sums `windows_net_*` per host). It is **not** NIC utilization % unless you add link-speed normalization in custom PromQL.

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
