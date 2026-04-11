package pgxprom

import (
	"context"
	"regexp"
	"slices"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

var _ prometheus.Collector = (*PoolCollector)(nil)

// PoolCollector is a Prometheus pool collector for pgx metrics.
type PoolCollector struct {
	mu                      sync.RWMutex
	acquireConns            *prometheus.Desc
	canceledAcquiresTotal   *prometheus.Desc
	constructingConns       *prometheus.Desc
	emptyAcquiresTotal      *prometheus.Desc
	idleConns               *prometheus.Desc
	maxConns                *prometheus.Desc
	totalConns              *prometheus.Desc
	newConnectionsTotal     *prometheus.Desc
	maxLifetimeDestroysTotal *prometheus.Desc
	maxIdleDestroysTotal    *prometheus.Desc
	pools                   []*pgxpool.Pool
}

// NewPoolCollector returns a new collector.
func NewPoolCollector() *PoolCollector {
	labels := []string{"database"}

	fqdn := func(v string) string {
		return prometheus.BuildFQName("pgx", "pool", v)
	}

	return &PoolCollector{
		acquireConns: prometheus.NewDesc(fqdn("acquire_connections"),
			"Number of connections currently in the process of being acquired.", labels, nil),
		canceledAcquiresTotal: prometheus.NewDesc(fqdn("canceled_acquires_total"),
			"Total number of connection acquires that were canceled.", labels, nil),
		constructingConns: prometheus.NewDesc(fqdn("constructing_connections"),
			"Number of connections currently in the process of being constructed.", labels, nil),
		emptyAcquiresTotal: prometheus.NewDesc(fqdn("empty_acquires_total"),
			"Total number of connection acquires that waited on an empty pool.", labels, nil),
		idleConns: prometheus.NewDesc(fqdn("idle_connections"),
			"Number of idle connections in the pool.", labels, nil),
		maxConns: prometheus.NewDesc(fqdn("max_connections"),
			"Maximum number of connections allowed in the pool.", labels, nil),
		totalConns: prometheus.NewDesc(fqdn("total_connections"),
			"Total number of connections in the pool.", labels, nil),
		newConnectionsTotal: prometheus.NewDesc(fqdn("new_connections_total"),
			"Total number of new connections created.", labels, nil),
		maxLifetimeDestroysTotal: prometheus.NewDesc(fqdn("max_lifetime_destroys_total"),
			"Total number of connections destroyed due to MaxLifetime.", labels, nil),
		maxIdleDestroysTotal: prometheus.NewDesc(fqdn("max_idle_destroys_total"),
			"Total number of connections destroyed due to MaxIdleTime.", labels, nil),
	}
}

// Add appends the pool to the collector.
func (p *PoolCollector) Add(pool *pgxpool.Pool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.pools = append(p.pools, pool)
}

// Remove removes the pool from the collector.
func (p *PoolCollector) Remove(pool *pgxpool.Pool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.pools = slices.DeleteFunc(p.pools, func(elem *pgxpool.Pool) bool {
		return pool == elem
	})
}

// Describe implements the prometheus.Collector interface.
func (p *PoolCollector) Describe(descs chan<- *prometheus.Desc) {
	descs <- p.acquireConns
	descs <- p.canceledAcquiresTotal
	descs <- p.constructingConns
	descs <- p.emptyAcquiresTotal
	descs <- p.idleConns
	descs <- p.maxConns
	descs <- p.totalConns
	descs <- p.newConnectionsTotal
	descs <- p.maxLifetimeDestroysTotal
	descs <- p.maxIdleDestroysTotal
}

// Collect implements the prometheus.Collector interface.
func (p *PoolCollector) Collect(metrics chan<- prometheus.Metric) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, pool := range p.pools {
		var (
			stats  = pool.Stat()
			labels = []string{pool.Config().ConnConfig.Database}
		)

		metrics <- prometheus.MustNewConstMetric(p.acquireConns, prometheus.GaugeValue, float64(stats.AcquiredConns()), labels...)
		metrics <- prometheus.MustNewConstMetric(p.canceledAcquiresTotal, prometheus.CounterValue, float64(stats.CanceledAcquireCount()), labels...)
		metrics <- prometheus.MustNewConstMetric(p.constructingConns, prometheus.GaugeValue, float64(stats.ConstructingConns()), labels...)
		metrics <- prometheus.MustNewConstMetric(p.emptyAcquiresTotal, prometheus.CounterValue, float64(stats.EmptyAcquireCount()), labels...)
		metrics <- prometheus.MustNewConstMetric(p.idleConns, prometheus.GaugeValue, float64(stats.IdleConns()), labels...)
		metrics <- prometheus.MustNewConstMetric(p.maxConns, prometheus.GaugeValue, float64(stats.MaxConns()), labels...)
		metrics <- prometheus.MustNewConstMetric(p.totalConns, prometheus.GaugeValue, float64(stats.TotalConns()), labels...)
		metrics <- prometheus.MustNewConstMetric(p.newConnectionsTotal, prometheus.CounterValue, float64(stats.NewConnsCount()), labels...)
		metrics <- prometheus.MustNewConstMetric(p.maxLifetimeDestroysTotal, prometheus.CounterValue, float64(stats.MaxLifetimeDestroyCount()), labels...)
		metrics <- prometheus.MustNewConstMetric(p.maxIdleDestroysTotal, prometheus.CounterValue, float64(stats.MaxIdleDestroyCount()), labels...)
	}
}

var (
	_ pgx.QueryTracer      = (*QueryCollector)(nil)
	_ pgx.BatchTracer      = (*QueryCollector)(nil)
	_ prometheus.Collector = (*QueryCollector)(nil)
)

// QueryCollector is a Prometheus query collector for pgx metrics.
type QueryCollector struct {
	requestTotal *prometheus.CounterVec
	errorsTotal  *prometheus.CounterVec
	duration     *prometheus.HistogramVec
}

// NewQueryCollector creates a new QueryCollector.
func NewQueryCollector() *QueryCollector {
	labels := []string{"database", "db_operation"}

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
				Name:      "request_duration_seconds",
				Help:      "Time taken to complete a database request.",
				Buckets:   []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 10},
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
		"database":     conn.Config().Database,
		"db_operation": q.name(args.SQL),
	}

	q.requestTotal.With(labels).Inc()

	return context.WithValue(ctx, TraceQueryKey, &TraceQueryData{
		StartedAt: time.Now(),
		SQL:       args.SQL,
		Args:      args.Args,
	})
}

// TraceQueryEnd implements pgx.QueryTracer.
func (q *QueryCollector) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, args pgx.TraceQueryEndData) {
	data, ok := ctx.Value(TraceQueryKey).(*TraceQueryData)
	if !ok {
		return
	}

	labels := prometheus.Labels{
		"database":     conn.Config().Database,
		"db_operation": q.name(data.SQL),
	}

	if args.Err != nil {
		q.errorsTotal.With(labels).Inc()
	}

	q.duration.With(labels).Observe(time.Since(data.StartedAt).Seconds())
}

// TraceBatchStart implements pgx.BatchTracer.
func (q *QueryCollector) TraceBatchStart(ctx context.Context, conn *pgx.Conn, args pgx.TraceBatchStartData) context.Context {
	for _, query := range args.Batch.QueuedQueries {
		labels := prometheus.Labels{
			"database":     conn.Config().Database,
			"db_operation": q.name(query.SQL),
		}

		q.requestTotal.With(labels).Inc()
	}

	return context.WithValue(ctx, TraceBatchKey, &TraceBatchData{
		StartedAt: time.Now(),
		Batch:     args.Batch,
	})
}

// TraceBatchQuery implements pgx.BatchTracer.
func (q *QueryCollector) TraceBatchQuery(ctx context.Context, conn *pgx.Conn, data pgx.TraceBatchQueryData) {
	if data.Err == nil {
		return
	}

	labels := prometheus.Labels{
		"database":     conn.Config().Database,
		"db_operation": q.name(data.SQL),
	}

	q.errorsTotal.With(labels).Inc()
}

// TraceBatchEnd implements pgx.BatchTracer.
func (q *QueryCollector) TraceBatchEnd(ctx context.Context, conn *pgx.Conn, args pgx.TraceBatchEndData) {
	data, ok := ctx.Value(TraceBatchKey).(*TraceBatchData)
	if !ok {
		return
	}

	elapsed := time.Since(data.StartedAt).Seconds()

	for _, query := range data.Batch.QueuedQueries {
		labels := prometheus.Labels{
			"database":     conn.Config().Database,
			"db_operation": q.name(query.SQL),
		}

		q.duration.With(labels).Observe(elapsed)
	}
}

var pattern = regexp.MustCompile(`^--\s+name:\s+(\w+)`)

func (q *QueryCollector) name(v string) string {
	if match := pattern.FindStringSubmatch(v); len(match) == 2 {
		return match[1]
	}

	return "unknown"
}
