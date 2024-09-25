package pgxprom

import (
	"context"
	"regexp"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

var _ prometheus.Collector = (*PoolCollector)(nil)

// PoolCollector is a Prometheus pool collector for pgx metrics.
type PoolCollector struct {
	acquireConns            *prometheus.Desc
	canceledAcquireCount    *prometheus.Desc
	constructingConns       *prometheus.Desc
	emptyAcquireCount       *prometheus.Desc
	idleConns               *prometheus.Desc
	maxConns                *prometheus.Desc
	totalConns              *prometheus.Desc
	newConnsCount           *prometheus.Desc
	maxLifetimeDestroyCount *prometheus.Desc
	maxIdleDestroyCount     *prometheus.Desc
	pools                   []*pgxpool.Pool
}

// NewPoolCollector returns a new collector.
func NewPoolCollector() *PoolCollector {
	labels := []string{"database"}

	fqdn := func(v string) string {
		return prometheus.BuildFQName("pgx", "pool", v)
	}

	return &PoolCollector{
		// metrics
		acquireConns: prometheus.NewDesc(fqdn("acquire_connections"),
			"Number of connections currently in the process of being acquired", labels, nil),
		canceledAcquireCount: prometheus.NewDesc(fqdn("canceled_acquire_count"),
			"Number of times a connection acquire was canceled", labels, nil),
		constructingConns: prometheus.NewDesc(fqdn("constructing_connections"),
			"Number of connections currently in the process of being constructed", labels, nil),
		emptyAcquireCount: prometheus.NewDesc(fqdn("empty_acquire_count"),
			"Number of times a connection acquire was canceled", labels, nil),
		idleConns: prometheus.NewDesc(fqdn("idle_connections"),
			"Number of idle connections in the pool", labels, nil),
		maxConns: prometheus.NewDesc(fqdn("max_connections"),
			"Maximum number of connections allowed in the pool", labels, nil),
		totalConns: prometheus.NewDesc(fqdn("total_connections"),
			"Total number of connections in the pool", labels, nil),
		newConnsCount: prometheus.NewDesc(fqdn("new_connections_count"),
			"Number of new connections created", labels, nil),
		maxLifetimeDestroyCount: prometheus.NewDesc(fqdn("max_lifetime_destroy_count"),
			"Number of connections destroyed due to MaxLifetime", labels, nil),
		maxIdleDestroyCount: prometheus.NewDesc(fqdn("max_idle_destroy_count"),
			"Number of connections destroyed due to MaxIdleTime", labels, nil),
	}
}

// Add append the pool the collector
func (p *PoolCollector) Add(pool *pgxpool.Pool) {
	p.pools = append(p.pools, pool)
}

// Remove removes the pool from the collector
func (p *PoolCollector) Remove(pool *pgxpool.Pool) {
	p.pools = slices.DeleteFunc(p.pools, func(elem *pgxpool.Pool) bool {
		return pool == elem
	})
}

// Describe implements the prometheus.Collector interface.
func (p *PoolCollector) Describe(descs chan<- *prometheus.Desc) {
	descs <- p.acquireConns
	descs <- p.canceledAcquireCount
	descs <- p.constructingConns
	descs <- p.emptyAcquireCount
	descs <- p.idleConns
	descs <- p.maxConns
	descs <- p.totalConns
	descs <- p.newConnsCount
	descs <- p.maxLifetimeDestroyCount
	descs <- p.maxIdleDestroyCount
}

// Collect implements the prometheus.Collector interface.
func (p *PoolCollector) Collect(metrics chan<- prometheus.Metric) {
	for _, pool := range p.pools {
		var (
			stats  = pool.Stat()
			labels = []string{
				pool.Config().ConnConfig.Database,
			}
		)

		// collect the metrics
		metrics <- prometheus.MustNewConstMetric(p.acquireConns, prometheus.GaugeValue, float64(stats.AcquiredConns()), labels...)
		metrics <- prometheus.MustNewConstMetric(p.canceledAcquireCount, prometheus.CounterValue, float64(stats.CanceledAcquireCount()), labels...)
		metrics <- prometheus.MustNewConstMetric(p.constructingConns, prometheus.GaugeValue, float64(stats.ConstructingConns()), labels...)
		metrics <- prometheus.MustNewConstMetric(p.emptyAcquireCount, prometheus.CounterValue, float64(stats.EmptyAcquireCount()), labels...)
		metrics <- prometheus.MustNewConstMetric(p.idleConns, prometheus.GaugeValue, float64(stats.IdleConns()), labels...)
		metrics <- prometheus.MustNewConstMetric(p.maxConns, prometheus.GaugeValue, float64(stats.MaxConns()), labels...)
		metrics <- prometheus.MustNewConstMetric(p.totalConns, prometheus.GaugeValue, float64(stats.TotalConns()), labels...)
		metrics <- prometheus.MustNewConstMetric(p.newConnsCount, prometheus.CounterValue, float64(stats.NewConnsCount()), labels...)
		metrics <- prometheus.MustNewConstMetric(p.maxLifetimeDestroyCount, prometheus.CounterValue, float64(stats.MaxLifetimeDestroyCount()), labels...)
		metrics <- prometheus.MustNewConstMetric(p.maxIdleDestroyCount, prometheus.CounterValue, float64(stats.MaxIdleDestroyCount()), labels...)
	}
}

var (
	_ pgx.QueryTracer      = (*QueryCollector)(nil)
	_ pgx.BatchTracer      = (*QueryCollector)(nil)
	_ prometheus.Collector = (*QueryCollector)(nil)
)

// QueryCollector is a Prometheus query collector for pgx metrics.
type QueryCollector struct {
	// Request is the total number of database requests.
	requestTotal *prometheus.CounterVec
	// ErrorsTotal is the total number of database request errors.
	errorsTotal *prometheus.CounterVec
	// Duration is the time taken to complete a database request and process the response.
	duration *prometheus.HistogramVec
}

// NewQueryCollector creates a new Tracer.
func NewQueryCollector() *QueryCollector {
	labels := []string{"db_name", "db_operation", "db_statement", "db_pgx_operation"}

	return &QueryCollector{
		requestTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "pgx",
				Subsystem: "conn",
				Name:      "requests_total",
				Help:      "Total number of database requests.",
			},
			labels,
		),

		errorsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "pgx",
				Subsystem: "conn",
				Name:      "request_errors_total",
				Help:      "Total number of database request errors.",
			},
			labels,
		),
		duration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "pgx",
				Subsystem: "conn",
				Name:      "requests_duration_seconds",
				Help:      "Time taken to complete a database request and process the response.",
			},
			labels,
		),
	}
}

// Collect implements prometheus.Collector.
func (q *QueryCollector) Collect(metrics chan<- prometheus.Metric) {
	q.requestTotal.Collect(metrics)
	q.errorsTotal.Collect(metrics)
	q.duration.Collect(metrics)
}

// Describe implements prometheus.Collector.
func (q *QueryCollector) Describe(descs chan<- *prometheus.Desc) {
	q.requestTotal.Describe(descs)
	q.errorsTotal.Describe(descs)
	q.duration.Describe(descs)
}

// TraceQueryStart implements pgx.QueryTracer.
func (q *QueryCollector) TraceQueryStart(ctx context.Context, conn *pgx.Conn, args pgx.TraceQueryStartData) context.Context {
	labels := prometheus.Labels{
		"db_name":          conn.Config().Database,
		"db_statement":     args.SQL,
		"db_operation":     q.name(args.SQL),
		"db_pgx_operation": "query_start",
	}

	q.requestTotal.With(labels).Inc()

	data := &TraceQueryData{
		StartedAt: time.Now(),
		SQL:       args.SQL,
		Args:      args.Args,
	}

	return context.WithValue(ctx, TraceQueryKey, data)
}

// TraceQueryEnd implements pgx.QueryTracer.
func (q *QueryCollector) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, args pgx.TraceQueryEndData) {
	if data, ok := ctx.Value(TraceQueryKey).(*TraceQueryData); ok {
		labels := prometheus.Labels{
			"db_name":          conn.Config().Database,
			"db_statement":     data.SQL,
			"db_operation":     q.name(data.SQL),
			"db_pgx_operation": "query_end",
		}

		if args.Err != nil {
			q.errorsTotal.With(labels).Inc()
		}

		q.duration.With(labels).Observe(time.Since(data.StartedAt).Seconds())
	}
}

// TraceBatchStart implements pgx.BatchTracer.
func (q *QueryCollector) TraceBatchStart(ctx context.Context, conn *pgx.Conn, args pgx.TraceBatchStartData) context.Context {
	data := &TraceBatchData{
		StartedAt: time.Now(),
		Batch:     args.Batch,
	}

	for _, query := range args.Batch.QueuedQueries {
		labels := prometheus.Labels{
			"db_name":          conn.Config().Database,
			"db_statement":     query.SQL,
			"db_operation":     q.name(query.SQL),
			"db_pgx_operation": "batch_start",
		}

		q.requestTotal.With(labels).Inc()
	}

	return context.WithValue(ctx, TraceBatchKey, data)
}

// TraceBatchQuery implements pgx.BatchTracer.
func (q *QueryCollector) TraceBatchQuery(ctx context.Context, conn *pgx.Conn, data pgx.TraceBatchQueryData) {
	labels := prometheus.Labels{
		"db_name":          conn.Config().Database,
		"db_statement":     data.SQL,
		"db_operation":     q.name(data.SQL),
		"db_pgx_operation": "batch_query",
	}

	if data.Err != nil {
		q.errorsTotal.With(labels).Inc()
	}

	if data, ok := ctx.Value(TraceQueryKey).(*TraceBatchData); ok {
		q.duration.With(labels).Observe(time.Since(data.StartedAt).Seconds())
	}
}

// TraceBatchEnd implements pgx.BatchTracer.
func (q *QueryCollector) TraceBatchEnd(ctx context.Context, conn *pgx.Conn, args pgx.TraceBatchEndData) {
	if data, ok := ctx.Value(TraceQueryKey).(*TraceBatchData); ok {
		for _, query := range data.Batch.QueuedQueries {
			labels := prometheus.Labels{
				"db_name":          conn.Config().Database,
				"db_statement":     query.SQL,
				"db_operation":     q.name(query.SQL),
				"db_pgx_operation": "batch_end",
			}

			if args.Err != nil {
				q.errorsTotal.With(labels).Inc()
			}

			q.duration.With(labels).Observe(time.Since(data.StartedAt).Seconds())
		}
	}
}

var pattern = regexp.MustCompile(`^--\s+name:\s+(\w+)`)

func (q *QueryCollector) name(v string) string {
	if match := pattern.FindStringSubmatch(v); len(match) == 2 {
		return match[1]
	}

	return "unknown"
}
