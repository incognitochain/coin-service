package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

//connect to database using a single connection
func main() {
	/***********************************************/
	/* Single Connection to TimescaleDB/ PostresQL */
	/***********************************************/
	ctx := context.Background()
	connStr := "postgres://postgres:lam123@0.0.0.0:5432/postgres"
	dbpool, err := pgxpool.Connect(ctx, connStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer dbpool.Close()

	//run a simple query to check our connection
	var greeting string
	err = dbpool.QueryRow(ctx, "select 'Hello, Timescale!'").Scan(&greeting)
	if err != nil {
		fmt.Fprintf(os.Stderr, "QueryRow failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(greeting)

	queryCreateHypertable := `CREATE TABLE IF NOT EXISTS sensor_data (
		time TIMESTAMPTZ NOT NULL,
		sensor_id INTEGER,
		temperature DOUBLE PRECISION,
		cpu DOUBLE PRECISION,
		FOREIGN KEY (sensor_id) REFERENCES sensors (id)
		);
		SELECT create_hypertable('sensor_data', 'time');
		`

	//execute statement
	_, err = dbpool.Exec(ctx, queryCreateHypertable)
	if err != nil {
		if !strings.Contains(err.Error(), "is already a hypertable") {
			fmt.Fprintf(os.Stderr, "Unable to create SENSOR_DATA hypertable: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Println("Successfully created hypertable SENSOR_DATA")

	// Generate data to insert

	//SQL query to generate sample data
	queryDataGeneration := `
       SELECT generate_series(now() - interval '24 hour', now(), interval '7 minute') AS time,
       floor(random() * (3) + 1)::int as sensor_id,
       random()*100 AS temperature,
       random() AS cpu
       `
	//Execute query to generate samples for sensor_data hypertable
	rows, err := dbpool.Query(ctx, queryDataGeneration)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to generate sensor data: %v\n", err)
		os.Exit(1)
	}
	defer rows.Close()
	fmt.Println("Successfully generated sensor data")

	//Store data generated in slice results
	type result struct {
		Time        time.Time
		SensorId    int
		Temperature float64
		CPU         float64
	}
	var results []result
	for rows.Next() {
		var r result
		err = rows.Scan(&r.Time, &r.SensorId, &r.Temperature, &r.CPU)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to scan %v\n", err)
			os.Exit(1)
		}
		results = append(results, r)
	}
	// Any errors encountered by rows.Next or rows.Scan are returned here
	if rows.Err() != nil {
		fmt.Fprintf(os.Stderr, "rows Error: %v\n", rows.Err())
		os.Exit(1)
	}

	// Check contents of results slice
	fmt.Println("Contents of RESULTS slice")
	for i := range results {
		r := results[i]
		fmt.Printf("Time: %s | ID: %d | Temperature: %f | CPU: %f |\n", &r.Time, r.SensorId, r.Temperature, r.CPU)
	}
	//Insert contents of results slice into TimescaleDB
	//SQL query to generate sample data
	queryInsertTimeseriesData := `
   INSERT INTO sensor_data (time, sensor_id, temperature, cpu) VALUES ($1, $2, $3, $4);
   `
	/********************************************/
	/* Batch Insert into TimescaleDB            */
	/********************************************/
	//create batch
	batch := &pgx.Batch{}
	numInserts := len(results)
	//load insert statements into batch queue
	for i := range results {
		r := results[i]
		batch.Queue(queryInsertTimeseriesData, r.Time, r.SensorId, r.Temperature, r.CPU)
	}
	batch.Queue("select count(*) from sensor_data")

	//send batch to connection pool
	br := dbpool.SendBatch(ctx, batch)
	//execute statements in batch queue
	_, err = br.Exec()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to execute statement in batch queue %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Successfully batch inserted data", numInserts)

	//Compare length of results slice to size of table
	fmt.Printf("size of results: %d\n", len(results))
	//check size of table for number of rows inserted
	// result of last SELECT statement
	// var rowsInserted int
	// err = br.QueryRow().Scan(&rowsInserted)
	// if err != nil {
	// 	fmt.Fprintf(os.Stderr, "Unable to closer batch %v\n", err)
	// 	os.Exit(1)
	// }
	// fmt.Printf("size of table: %d\n", rowsInserted)

	err = br.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to closer batch %v\n", err)
		os.Exit(1)
	}

	/********************************************/
	/* Execute a query                          */
	/********************************************/

	// Formulate query in SQL
	// Note the use of prepared statement placeholders $1 and $2
	queryTimebucketFiveMin := `
       SELECT time_bucket('10 minutes', time) AS five_min, avg(cpu)
       FROM sensor_data
       JOIN sensors ON sensors.id = sensor_data.sensor_id
       WHERE sensors.location = $1 AND sensors.type = $2
       GROUP BY five_min
       ORDER BY five_min DESC;
       `

	//Execute query on TimescaleDB
	rows, err = dbpool.Query(ctx, queryTimebucketFiveMin, "ceiling", "a")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to execute query %v\n", err)
		os.Exit(1)
	}
	defer rows.Close()
	fmt.Println("Successfully executed query")

	//Do something with the results of query
	// Struct for results
	type result2 struct {
		Bucket time.Time
		Avg    float64
	}

	// Print rows returned and fill up results slice for later use
	var results2 []result2
	for rows.Next() {
		var r result2
		err = rows.Scan(&r.Bucket, &r.Avg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to scan %v\n", err)
			os.Exit(1)
		}
		results2 = append(results2, r)
		fmt.Printf("Time bucket: %s | Avg: %f\n", &r.Bucket, r.Avg)
	}
	// Any errors encountered by rows.Next or rows.Scan are returned here
	if rows.Err() != nil {
		fmt.Fprintf(os.Stderr, "rows Error: %v\n", rows.Err())
		os.Exit(1)
	}

}
