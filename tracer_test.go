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
	defer conn.Close()

	rows, err := conn.Query(context.TODO(), "SELECT * from customer")
	if err != nil {
		panic(err)
	}
	// close the rows
	defer rows.Close()

	// Customer struct must be defined
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
