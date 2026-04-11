package pgxprom_test

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgx-contrib/pgxprom"
	"github.com/prometheus/client_golang/prometheus"
)

func ExamplePoolCollector() {
	config, err := pgxpool.ParseConfig(os.Getenv("PGX_DATABASE_URL"))
	if err != nil {
		panic(err)
	}

	pool, err := pgxpool.NewWithConfig(context.TODO(), config)
	if err != nil {
		panic(err)
	}

	collector := pgxprom.NewPoolCollector()
	// register the pool
	collector.Add(pool)
	// register the collector
	if err := prometheus.Register(collector); err != nil {
		panic(err)
	}
}

func ExampleQueryCollector() {
	config, err := pgxpool.ParseConfig(os.Getenv("PGX_DATABASE_URL"))
	if err != nil {
		panic(err)
	}

	collector := pgxprom.NewQueryCollector()
	// register the collector
	prometheus.MustRegister(collector)
	// attach as the pgx tracer
	config.ConnConfig.Tracer = collector

	conn, err := pgxpool.NewWithConfig(context.TODO(), config)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	rows, err := conn.Query(context.TODO(), "SELECT * from customer")
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	type Customer struct {
		FirstName string `db:"first_name"`
		LastName  string `db:"last_name"`
	}

	for rows.Next() {
		customer, err := pgx.RowToStructByName[Customer](rows)
		if err != nil {
			panic(err)
		}

		fmt.Println(customer.FirstName)
	}
}
