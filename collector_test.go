package pgxprom_test

import (
	"context"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgx-contrib/pgxprom"
	"github.com/prometheus/client_golang/prometheus"
)

func ExamplePoolStatsCollector() {
	config, err := pgxpool.ParseConfig(os.Getenv("PGX_DATABASE_URL"))
	if err != nil {
		panic(err)
	}

	pool, err := pgxpool.NewWithConfig(context.TODO(), config)
	if err != nil {
		panic(err)
	}

	collector := pgxprom.NewPoolStatsCollector()
	// register the pool
	collector.Register(pool)
	// register the collector
	if err := prometheus.Register(collector); err != nil {
		panic(err)
	}
}
