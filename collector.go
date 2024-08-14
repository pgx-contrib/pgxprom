package pgxprom

import (
	"slices"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

var _ prometheus.Collector = (*PoolStatsCollector)(nil)

// PoolStatsCollector is a Prometheus pool collector for pgx metrics.
type PoolStatsCollector struct {
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

// NewPoolStatsCollector returns a new collector.
func NewPoolStatsCollector() *PoolStatsCollector {
	labels := []string{"database"}

	fqdn := func(v string) string {
		return prometheus.BuildFQName("pgx", "pool", v)
	}

	return &PoolStatsCollector{
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

// Register append the pool the collector
func (p *PoolStatsCollector) Register(pool *pgxpool.Pool) {
	p.pools = append(p.pools, pool)
}

// Unregister removes the pool from the collector
func (p *PoolStatsCollector) Unregister(pool *pgxpool.Pool) {
	p.pools = slices.DeleteFunc(p.pools, func(elem *pgxpool.Pool) bool {
		return pool == elem
	})
}

// Describe implements the prometheus.Collector interface.
func (p *PoolStatsCollector) Describe(descs chan<- *prometheus.Desc) {
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
func (p *PoolStatsCollector) Collect(metrics chan<- prometheus.Metric) {
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
