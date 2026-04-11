package pgxprom

import (
	"context"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// newPool creates a pool from PGX_DATABASE_URL, optionally attaching a tracer.
func newPool(tracer pgx.QueryTracer) (*pgxpool.Pool, string) {
	config, err := pgxpool.ParseConfig(os.Getenv("PGX_DATABASE_URL"))
	Expect(err).NotTo(HaveOccurred())
	if tracer != nil {
		config.ConnConfig.Tracer = tracer
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	Expect(err).NotTo(HaveOccurred())
	return pool, config.ConnConfig.Database
}

var _ = Describe("PoolCollector", func() {
	// -------------------------------------------------------------------------
	Describe("NewPoolCollector", func() {
		It("returns a non-nil collector", func() {
			Expect(NewPoolCollector()).NotTo(BeNil())
		})

		It("Describe sends 10 descriptors", func() {
			ch := make(chan *prometheus.Desc, 20)
			NewPoolCollector().Describe(ch)
			close(ch)
			Expect(ch).To(HaveLen(10))
		})

		It("registers on a fresh registry without error", func() {
			Expect(prometheus.NewRegistry().Register(NewPoolCollector())).To(Succeed())
		})
	})

	// -------------------------------------------------------------------------
	Describe("Integration", Ordered, func() {
		var (
			pool      *pgxpool.Pool
			collector *PoolCollector
			reg       *prometheus.Registry
		)

		BeforeAll(func() {
			if os.Getenv("PGX_DATABASE_URL") == "" {
				Skip("PGX_DATABASE_URL not set")
			}

			pool, _ = newPool(nil)

			collector = NewPoolCollector()
			collector.Add(pool)

			reg = prometheus.NewRegistry()
			Expect(reg.Register(collector)).To(Succeed())
		})

		AfterAll(func() {
			if pool != nil {
				pool.Close()
			}
		})

		It("Collect emits one metric per descriptor (10 total)", func() {
			count, err := testutil.GatherAndCount(reg)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(10))
		})

		It("emits pgx_pool_max_connections with the database label", func() {
			count, err := testutil.GatherAndCount(reg, "pgx_pool_max_connections")
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))
		})

		It("emits pgx_pool_total_connections gauge", func() {
			count, err := testutil.GatherAndCount(reg, "pgx_pool_total_connections")
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))
		})

		It("Remove stops emitting metrics for the pool", func() {
			collector.Remove(pool)
			defer collector.Add(pool)

			count, err := testutil.GatherAndCount(reg, "pgx_pool_max_connections")
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(0))
		})
	})
})

var _ = Describe("QueryCollector", func() {
	// -------------------------------------------------------------------------
	Describe("NewQueryCollector", func() {
		It("returns a non-nil collector", func() {
			Expect(NewQueryCollector()).NotTo(BeNil())
		})

		It("Describe sends 3 descriptors", func() {
			ch := make(chan *prometheus.Desc, 10)
			NewQueryCollector().Describe(ch)
			close(ch)
			Expect(ch).To(HaveLen(3))
		})

		It("registers on a fresh registry without error", func() {
			Expect(prometheus.NewRegistry().Register(NewQueryCollector())).To(Succeed())
		})
	})

	// -------------------------------------------------------------------------
	Describe("name", func() {
		q := NewQueryCollector()

		DescribeTable("extracts operation name from SQL",
			func(sql, expected string) {
				Expect(q.name(sql)).To(Equal(expected))
			},
			Entry("-- name: comment", "-- name: FindUser\nSELECT 1", "FindUser"),
			Entry("plain SQL falls back to unknown", "SELECT * FROM users", "unknown"),
			Entry("empty string falls back to unknown", "", "unknown"),
			Entry("missing space after --", "--name: Foo\nSELECT 1", "unknown"),
			Entry("missing space after colon", "-- name:Foo\nSELECT 1", "unknown"),
			Entry("underscore in identifier", "-- name: get_user\nSELECT 1", "get_user"),
			Entry("digit in identifier", "-- name: query2\nSELECT 1", "query2"),
		)
	})

	// -------------------------------------------------------------------------
	Describe("TraceQueryStart / TraceQueryEnd", Ordered, func() {
		var (
			pool      *pgxpool.Pool
			collector *QueryCollector
			dbName    string
		)

		BeforeAll(func() {
			if os.Getenv("PGX_DATABASE_URL") == "" {
				Skip("PGX_DATABASE_URL not set")
			}

			collector = NewQueryCollector()
			pool, dbName = newPool(collector)
		})

		AfterAll(func() {
			if pool != nil {
				pool.Close()
			}
		})

		It("increments requests_total on each query", func() {
			labels := prometheus.Labels{"database": dbName, "db_operation": "unknown"}
			before := testutil.ToFloat64(collector.requestTotal.With(labels))

			rows, err := pool.Query(context.Background(), "SELECT 1")
			Expect(err).NotTo(HaveOccurred())
			rows.Close()

			Expect(testutil.ToFloat64(collector.requestTotal.With(labels))).To(Equal(before + 1))
		})

		It("uses -- name: comment as the db_operation label value", func() {
			labels := prometheus.Labels{"database": dbName, "db_operation": "GetOne"}
			before := testutil.ToFloat64(collector.requestTotal.With(labels))

			rows, err := pool.Query(context.Background(), "-- name: GetOne\nSELECT 1")
			Expect(err).NotTo(HaveOccurred())
			rows.Close()

			Expect(testutil.ToFloat64(collector.requestTotal.With(labels))).To(Equal(before + 1))
		})

		It("increments request_errors_total on a query error", func() {
			labels := prometheus.Labels{"database": dbName, "db_operation": "unknown"}
			before := testutil.ToFloat64(collector.errorsTotal.With(labels))

			var val int
			err := pool.QueryRow(context.Background(), "SELECT 1/0").Scan(&val)
			Expect(err).To(HaveOccurred())

			Expect(testutil.ToFloat64(collector.errorsTotal.With(labels))).To(Equal(before + 1))
		})

		It("observes request_duration_seconds after a query", func() {
			rows, err := pool.Query(context.Background(), "SELECT 1")
			Expect(err).NotTo(HaveOccurred())
			rows.Close()

			Expect(testutil.CollectAndCount(collector.duration)).To(BeNumerically(">", 0))
		})
	})

	// -------------------------------------------------------------------------
	Describe("TraceBatchStart / TraceBatchQuery / TraceBatchEnd", Ordered, func() {
		var (
			pool      *pgxpool.Pool
			collector *QueryCollector
			dbName    string
		)

		BeforeAll(func() {
			if os.Getenv("PGX_DATABASE_URL") == "" {
				Skip("PGX_DATABASE_URL not set")
			}

			collector = NewQueryCollector()
			pool, dbName = newPool(collector)
		})

		AfterAll(func() {
			if pool != nil {
				pool.Close()
			}
		})

		It("increments requests_total for each queued query in the batch", func() {
			labels1 := prometheus.Labels{"database": dbName, "db_operation": "unknown"}
			labels2 := prometheus.Labels{"database": dbName, "db_operation": "BatchItem"}
			before1 := testutil.ToFloat64(collector.requestTotal.With(labels1))
			before2 := testutil.ToFloat64(collector.requestTotal.With(labels2))

			batch := &pgx.Batch{}
			batch.Queue("SELECT 1")
			batch.Queue("-- name: BatchItem\nSELECT 2")

			results := pool.SendBatch(context.Background(), batch)
			_, err := results.Exec()
			Expect(err).NotTo(HaveOccurred())
			_, err = results.Exec()
			Expect(err).NotTo(HaveOccurred())
			Expect(results.Close()).To(Succeed())

			Expect(testutil.ToFloat64(collector.requestTotal.With(labels1))).To(Equal(before1 + 1))
			Expect(testutil.ToFloat64(collector.requestTotal.With(labels2))).To(Equal(before2 + 1))
		})

		It("observes request_duration_seconds for each query in the batch", func() {
			batch := &pgx.Batch{}
			batch.Queue("SELECT 1")
			batch.Queue("SELECT 2")

			results := pool.SendBatch(context.Background(), batch)
			_, err := results.Exec()
			Expect(err).NotTo(HaveOccurred())
			_, err = results.Exec()
			Expect(err).NotTo(HaveOccurred())
			Expect(results.Close()).To(Succeed())

			Expect(testutil.CollectAndCount(collector.duration)).To(BeNumerically(">", 0))
		})

		It("increments request_errors_total for a failing batch query", func() {
			labels := prometheus.Labels{"database": dbName, "db_operation": "unknown"}
			before := testutil.ToFloat64(collector.errorsTotal.With(labels))

			batch := &pgx.Batch{}
			batch.Queue("SELECT 1/0")

			results := pool.SendBatch(context.Background(), batch)
			_, err := results.Exec()
			Expect(err).To(HaveOccurred())
			_ = results.Close()

			Expect(testutil.ToFloat64(collector.errorsTotal.With(labels))).To(Equal(before + 1))
		})
	})
})
