package pgxprom

import (
	"context"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	_ pgx.QueryTracer = (*Tracer)(nil)
	_ pgx.BatchTracer = (*Tracer)(nil)
)

// Tracer is a Prometheus query collector for pgx metrics.
type Tracer struct {
	// Request is the total number of database requests.
	requestTotal *prometheus.CounterVec
	// ErrorsTotal is the total number of database request errors.
	errorsTotal *prometheus.CounterVec
	// Duration is the time taken to complete a database request and process the response.
	duration *prometheus.HistogramVec
}

// NewTracer creates a new Tracer.
func NewTracer() *Tracer {
	labels := []string{"db_name", "db_operation", "db_operation_phase"}

	return &Tracer{
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

// Register registers the metrics.
func (q *Tracer) Register(r prometheus.Registerer) {
	for _, item := range q.collectors() {
		r.MustRegister(item)
	}
}

// Unregister unregisters the metrics.
func (q *Tracer) Unregister(r prometheus.Registerer) {
	for _, item := range q.collectors() {
		r.Unregister(item)
	}
}

// TraceQueryStart implements pgx.QueryTracer.
func (q *Tracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, args pgx.TraceQueryStartData) context.Context {
	labels := prometheus.Labels{
		"db_name":            conn.Config().Database,
		"db_operation":       q.name(args.SQL),
		"db_operation_phase": "query_start",
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
func (q *Tracer) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, args pgx.TraceQueryEndData) {
	if data, ok := ctx.Value(TraceQueryKey).(*TraceQueryData); ok {
		labels := prometheus.Labels{
			"db_name":            conn.Config().Database,
			"db_operation":       q.name(data.SQL),
			"db_operation_phase": "query_end",
		}

		if args.Err != nil {
			q.errorsTotal.With(labels).Inc()
		}

		q.duration.With(labels).Observe(time.Since(data.StartedAt).Seconds())
	}
}

// TraceBatchStart implements pgx.BatchTracer.
func (q *Tracer) TraceBatchStart(ctx context.Context, conn *pgx.Conn, args pgx.TraceBatchStartData) context.Context {
	data := &TraceBatchData{
		StartedAt: time.Now(),
		Batch:     args.Batch,
	}

	for _, query := range args.Batch.QueuedQueries {
		labels := prometheus.Labels{
			"db_name":            conn.Config().Database,
			"db_operation":       q.name(query.SQL),
			"db_operation_phase": "batch_start",
		}

		q.requestTotal.With(labels).Inc()
	}

	return context.WithValue(ctx, TraceBatchKey, data)
}

// TraceBatchQuery implements pgx.BatchTracer.
func (q *Tracer) TraceBatchQuery(ctx context.Context, conn *pgx.Conn, data pgx.TraceBatchQueryData) {
	labels := prometheus.Labels{
		"db_name":            conn.Config().Database,
		"db_operation":       q.name(data.SQL),
		"db_operation_phase": "batch_query",
	}

	if data.Err != nil {
		q.errorsTotal.With(labels).Inc()
	}

	if data, ok := ctx.Value(TraceQueryKey).(*TraceBatchData); ok {
		q.duration.With(labels).Observe(time.Since(data.StartedAt).Seconds())
	}
}

// TraceBatchEnd implements pgx.BatchTracer.
func (q *Tracer) TraceBatchEnd(ctx context.Context, conn *pgx.Conn, args pgx.TraceBatchEndData) {
	if data, ok := ctx.Value(TraceQueryKey).(*TraceBatchData); ok {
		for _, query := range data.Batch.QueuedQueries {
			labels := prometheus.Labels{
				"db_name":            conn.Config().Database,
				"db_operation":       q.name(query.SQL),
				"db_operation_phase": "batch_end",
			}

			if args.Err != nil {
				q.errorsTotal.With(labels).Inc()
			}

			q.duration.With(labels).Observe(time.Since(data.StartedAt).Seconds())
		}
	}
}

var pattern = regexp.MustCompile(`^--\s+name:\s+(\w+)`)

func (q *Tracer) name(v string) string {
	if match := pattern.FindStringSubmatch(v); len(match) == 2 {
		return match[1]
	}

	return "unknown"
}

func (q *Tracer) collectors() []prometheus.Collector {
	return []prometheus.Collector{
		q.requestTotal,
		q.errorsTotal,
		q.duration,
	}
}
