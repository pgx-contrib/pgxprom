package pgxprom_test

import (
	"context"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgx-contrib/pgxprom"
)

var count int

func ExampleTracer() {
	config, err := pgxpool.ParseConfig(os.Getenv("PGX_DATABASE_URL"))
	if err != nil {
		panic(err)
	}

	config.ConnConfig.Tracer = &pgxprom.Tracer{}

	conn, err := pgxpool.NewWithConfig(context.TODO(), config)
	if err != nil {
		panic(err)
	}

	row := conn.QueryRow(context.TODO(), "SELECT 1")
	if err := row.Scan(&count); err != nil {
		panic(err)
	}
}
