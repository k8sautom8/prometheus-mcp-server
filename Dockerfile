FROM golang:1.23-bookworm AS builder

ENV GOTOOLCHAIN=auto

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /prometheus-mcp-server ./cmd/prometheus-mcp-server

FROM debian:bookworm-slim

WORKDIR /app

RUN apt-get update && \
    apt-get upgrade -y && \
    apt-get install -y --no-install-recommends \
        curl \
        procps \
        ca-certificates && \
    rm -rf /var/lib/apt/lists/* && \
    apt-get clean && \
    apt-get autoremove -y

RUN groupadd -r -g 1000 app && \
    useradd -r -g app -u 1000 -d /app -s /bin/false app && \
    chown -R app:app /app && \
    chmod 755 /app && \
    chmod -R go-w /app

COPY --from=builder --chown=app:app /prometheus-mcp-server /usr/local/bin/prometheus-mcp-server

ENV PROMETHEUS_MCP_BIND_HOST=0.0.0.0 \
    PROMETHEUS_MCP_BIND_PORT=8052

USER app

EXPOSE 8052

HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD if [ "$PROMETHEUS_MCP_SERVER_TRANSPORT" = "http" ] || [ "$PROMETHEUS_MCP_SERVER_TRANSPORT" = "sse" ]; then \
            curl -fsS "http://127.0.0.1:${PROMETHEUS_MCP_BIND_PORT}/" >/dev/null || exit 1; \
        else \
            pgrep -f prometheus-mcp-server >/dev/null 2>&1 || exit 1; \
        fi

CMD ["/usr/local/bin/prometheus-mcp-server"]

LABEL org.opencontainers.image.title="Prometheus MCP Server" \
      org.opencontainers.image.description="Model Context Protocol server for Prometheus integration, enabling AI assistants to query metrics and monitor system health" \
      org.opencontainers.image.version="1.6.0" \
      org.opencontainers.image.authors="Pavel Shklovsky <pavel@cloudefined.com>" \
      org.opencontainers.image.source="https://github.com/pab1it0/prometheus-mcp-server" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.url="https://github.com/pab1it0/prometheus-mcp-server" \
      org.opencontainers.image.documentation="https://github.com/pab1it0/prometheus-mcp-server/blob/main/docs/" \
      org.opencontainers.image.vendor="Pavel Shklovsky" \
      org.opencontainers.image.base.name="debian:bookworm-slim" \
      org.opencontainers.image.created="" \
      org.opencontainers.image.revision="" \
      io.modelcontextprotocol.server.name="io.github.pab1it0/prometheus-mcp-server" \
      mcp.server.name="prometheus-mcp-server" \
      mcp.server.category="monitoring" \
      mcp.server.tags="prometheus,monitoring,metrics,observability" \
      mcp.server.transport.stdio="true" \
      mcp.server.transport.http="true" \
      mcp.server.transport.sse="true"
