package pgxprom_test

import (
	"context"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgx-contrib/pgxprom"
	"github.com/prometheus/client_golang/prometheus"
)

var count int

func ExampleTracer() {
	config, err := pgxpool.ParseConfig(os.Getenv("PGX_DATABASE_URL"))
	if err != nil {
		panic(err)
	}

	tracer := pgxprom.NewTracer()
	// Register the tracer with the default prometheus registerer
	tracer.Register(prometheus.DefaultRegisterer)
	// set the tracer on the config
	config.ConnConfig.Tracer = tracer

	conn, err := pgxpool.NewWithConfig(context.TODO(), config)
	if err != nil {
		panic(err)
	}

	row := conn.QueryRow(context.TODO(), "SELECT 1")
	if err := row.Scan(&count); err != nil {
		panic(err)
	}
}
