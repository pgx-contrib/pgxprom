# pgxprom

[![CI](https://github.com/pgx-contrib/pgxprom/actions/workflows/ci.yml/badge.svg)](https://github.com/pgx-contrib/pgxprom/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/pgx-contrib/pgxprom?include_prereleases)](https://github.com/pgx-contrib/pgxprom/releases)
[![Go Reference](https://pkg.go.dev/badge/github.com/pgx-contrib/pgxprom.svg)](https://pkg.go.dev/github.com/pgx-contrib/pgxprom)
[![License](https://img.shields.io/github/license/pgx-contrib/pgxprom)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![pgx](https://img.shields.io/badge/pgx-v5-blue)](https://github.com/jackc/pgx)
[![Prometheus](https://img.shields.io/badge/Prometheus-enabled-e6522c?logo=prometheus&logoColor=white)](https://prometheus.io)

Prometheus instrumentation for [pgx v5](https://github.com/jackc/pgx). Provides
two collectors: `PoolCollector` exposes connection pool metrics, and
`QueryCollector` records per-query request counts, error counts, and latency
histograms as Prometheus metrics.

## Installation

```bash
go get github.com/pgx-contrib/pgxprom
```

## Usage

### PoolCollector

`PoolCollector` implements `prometheus.Collector` and exposes connection pool
statistics for one or more `pgxpool.Pool` instances.

```go
config, err := pgxpool.ParseConfig(os.Getenv("PGX_DATABASE_URL"))
if err != nil {
    panic(err)
}

pool, err := pgxpool.NewWithConfig(context.Background(), config)
if err != nil {
    panic(err)
}

collector := pgxprom.NewPoolCollector()
// register the pool with the collector
collector.Add(pool)
// register the collector with Prometheus
if err := prometheus.Register(collector); err != nil {
    panic(err)
}
```

To stop tracking a pool (e.g. on graceful shutdown):

```go
collector.Remove(pool)
```

### QueryCollector

`QueryCollector` implements both `pgx.QueryTracer` (and `pgx.BatchTracer`) and
`prometheus.Collector`. Attach it to `ConnConfig.Tracer` to record metrics for
every query and batch operation.

```go
config, err := pgxpool.ParseConfig(os.Getenv("PGX_DATABASE_URL"))
if err != nil {
    panic(err)
}

collector := pgxprom.NewQueryCollector()
// register with Prometheus
if err := prometheus.Register(collector); err != nil {
    panic(err)
}
// attach as the pgx tracer
config.ConnConfig.Tracer = collector

pool, err := pgxpool.NewWithConfig(context.Background(), config)
if err != nil {
    panic(err)
}
defer pool.Close()

rows, err := pool.Query(context.Background(), "SELECT * FROM customer")
if err != nil {
    panic(err)
}
defer rows.Close()
```

### Named queries

Prefix any SQL string with `-- name: <Identifier>` to set a low-cardinality
`db_operation` label instead of `unknown`:

```go
rows, err := pool.Query(ctx,
    "-- name: ListActiveCustomers\nSELECT * FROM customer WHERE active = true",
)
```

This records the metric with `db_operation="ListActiveCustomers"`.

## Metrics reference

### PoolCollector — `pgx_pool_*`

All pool metrics carry a single `database` label (the database name from the
connection config).

| Metric | Type | Description |
|--------|------|-------------|
| `pgx_pool_acquire_connections` | Gauge | Connections currently being acquired |
| `pgx_pool_canceled_acquires_total` | Counter | Acquire attempts that were canceled |
| `pgx_pool_constructing_connections` | Gauge | Connections currently being constructed |
| `pgx_pool_empty_acquires_total` | Counter | Acquire attempts that waited on an empty pool |
| `pgx_pool_idle_connections` | Gauge | Idle connections in the pool |
| `pgx_pool_max_connections` | Gauge | Maximum connections allowed in the pool |
| `pgx_pool_total_connections` | Gauge | Total connections in the pool |
| `pgx_pool_new_connections_total` | Counter | New connections created |
| `pgx_pool_max_lifetime_destroys_total` | Counter | Connections destroyed due to MaxLifetime |
| `pgx_pool_max_idle_destroys_total` | Counter | Connections destroyed due to MaxIdleTime |

### QueryCollector — `pgx_conn_*`

All query metrics carry two labels:

| Label | Description |
|-------|-------------|
| `database` | Database name from the connection config |
| `db_operation` | Name extracted from `-- name: <Identifier>` comment, or `unknown` |

| Metric | Type | Description |
|--------|------|-------------|
| `pgx_conn_requests_total` | Counter | Total database requests |
| `pgx_conn_request_errors_total` | Counter | Total database request errors |
| `pgx_conn_request_duration_seconds` | Histogram | Request latency in seconds |

## Contributing

Contributions are welcome! Please open an issue or pull request.

To set up a development environment with [Nix](https://nixos.org):

```bash
nix develop
```

Or using the provided dev container:

```bash
devcontainer up --workspace-folder . --remove-existing-container
```

Then run the tests:

```bash
go test ./...
```

## License

[MIT](LICENSE)
