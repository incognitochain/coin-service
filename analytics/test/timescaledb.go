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

	queryCreateHypertable := `CREATE TABLE IF NOT EXISTS trade_data (
		time TIMESTAMPTZ NOT NULL,
		trade_id INTEGER,
		price DOUBLE PRECISION,
		pair_id TEXT,
		pool_id TEXT,
		actual_token1_amount INTEGER,
		actual_token2_amount INTEGER
		);
		SELECT create_hypertable('trade_data', 'time');
		`

	//execute statement
	_, err = dbpool.Exec(ctx, queryCreateHypertable)
	if err != nil {
		if !strings.Contains(err.Error(), "is already a hypertable") {
			fmt.Fprintf(os.Stderr, "Unable to create trade_data hypertable: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Println("Successfully created hypertable trade_data")

	// Generate data to insert

	//SQL query to generate sample data
	queryDataGeneration := `
       SELECT generate_series(now() - interval '24 hour', now(), interval '7 minute') AS time,
       floor(random() * (3) + 1)::int as trade_id,
       random()*100 AS price,
	   md5(floor(random() * (3) + 1)::text) as pair_id,
	   md5(floor(random() * (3) + 1)::text) as pool_id,
       floor(random()*100) AS actual_token1_amount,
       floor(random()*100) AS actual_token2_amount
       `
	for i := 0; i < 10; i++ {
		//Execute query to generate samples for sensor_data hypertable
		rows, err := dbpool.Query(ctx, queryDataGeneration)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to generate sensor data: %v\n", err)
			os.Exit(1)
		}
		defer rows.Close()
		fmt.Println("Successfully generated trade data")

		//Store data generated in slice results
		type result struct {
			Time         time.Time
			TradeId      int
			Price        float64
			PairID       string
			PoolID       string
			Token1Amount int
			Token2Amount int
		}
		var results []result
		for rows.Next() {
			var r result
			err = rows.Scan(&r.Time, &r.TradeId, &r.Price, &r.PairID, &r.PoolID, &r.Token1Amount, &r.Token2Amount)
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
			fmt.Printf("Time: %s | ID: %d | Price: %f | PoolID: %v |\n", &r.Time, r.TradeId, r.Price, r.PoolID)
		}
		//Insert contents of results slice into TimescaleDB
		//SQL query to generate sample data
		queryInsertTimeseriesData := `
   INSERT INTO trade_data (time, trade_id, price, pair_id, pool_id, actual_token1_amount, actual_token2_amount) VALUES ($1, $2, $3, $4, $5, $6, $7);
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
			batch.Queue(queryInsertTimeseriesData, r.Time, r.TradeId, r.Price, r.PairID, r.PoolID, r.Token1Amount, r.Token2Amount)
		}
		// batch.Queue("select count(*) from trade_data")

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
	}
	queryCreateView15m := `CREATE MATERIALIZED VIEW trade_view_15m WITH (timescaledb.continuous) AS select time_bucket('15 minutes', time) AS period, first(price,time) open, last(price,time) "close",avg(price) as Average,max(price) as high,min(price) as low, count(*) as "set", sum(actual_token1_amount) as volume_tk1, sum(actual_token2_amount) as volume_tk2, pool_id from trade_data GROUP BY period,pool_id WITH NO DATA`

	queryCreateView30m := `CREATE MATERIALIZED VIEW trade_view_30m WITH (timescaledb.continuous) AS select time_bucket('30 minutes', time) AS period, first(price,time) open, last(price,time) "close",avg(price) as Average,max(price) as high,min(price) as low, count(*) as "set" from trade_data GROUP BY period WITH NO DATA`

	queryCreateView1H := `CREATE MATERIALIZED VIEW trade_view_1H WITH (timescaledb.continuous) AS select time_bucket('1h', time) AS period, first(price,time) open, last(price,time) "close",avg(price) as Average,max(price) as high,min(price) as low, count(*) as "set" from trade_data GROUP BY period WITH NO DATA`

	queryCreateView4H := `CREATE MATERIALIZED VIEW trade_view_4H WITH (timescaledb.continuous) AS select time_bucket('4h', time) AS period, first(price,time) open, last(price,time) "close",avg(price) as Average,max(price) as high,min(price) as low, count(*) as "set" from trade_data GROUP BY period WITH NO DATA`

	queryCreateView1D := `CREATE MATERIALIZED VIEW trade_view_1D WITH (timescaledb.continuous) AS select time_bucket('1 days', time) AS period, first(price,time) open, last(price,time) "close",avg(price) as Average,max(price) as high,min(price) as low, count(*) as "set" from trade_data GROUP BY period WITH NO DATA`

	queryCreateView1W := `CREATE MATERIALIZED VIEW trade_view_1W WITH (timescaledb.continuous) AS select time_bucket('1 weeks', time) AS period, first(price,time) open, last(price,time) "close",avg(price) as Average,max(price) as high,min(price) as low, count(*) as "set" from trade_data GROUP BY period WITH NO DATA`

	queryContinuousAgg15m := `SELECT add_continuous_aggregate_policy('trade_view_15m',
	 start_offset => INTERVAL '3 week',
	 end_offset   => INTERVAL '1 minute',
	 schedule_interval => INTERVAL '5 minutes');`

	queryContinuousAgg30m := `SELECT add_continuous_aggregate_policy('trade_view_30m',
	 start_offset => INTERVAL '3 week',
	 end_offset   => INTERVAL '1 minute',
	 schedule_interval => INTERVAL '5 minutes');`

	queryContinuousAgg1h := `SELECT add_continuous_aggregate_policy('trade_view_1H',
	 start_offset => INTERVAL '3 week',
	 end_offset   => INTERVAL '1 minute',
	 schedule_interval => INTERVAL '5 minutes');`

	queryContinuousAgg4h := `SELECT add_continuous_aggregate_policy('trade_view_4H',
	 start_offset => INTERVAL '3 week',
	 end_offset   => INTERVAL '1 minute',
	 schedule_interval => INTERVAL '5 minutes');`

	queryContinuousAgg1d := `SELECT add_continuous_aggregate_policy('trade_view_1D',
	 start_offset => INTERVAL '3 week',
	 end_offset   => INTERVAL '1 minute',
	 schedule_interval => INTERVAL '5 minutes');`

	queryContinuousAgg1w := `SELECT add_continuous_aggregate_policy('trade_view_1W',
	 start_offset => INTERVAL '3 week',
	 end_offset   => INTERVAL '1 minute',
	 schedule_interval => INTERVAL '5 minutes');`

	_, err = dbpool.Exec(context.Background(), queryCreateView15m)
	if err != nil {
		fmt.Println(err)
	}
	_, err = dbpool.Exec(context.Background(), queryCreateView30m)
	if err != nil {
		fmt.Println(err)
	}
	_, err = dbpool.Exec(context.Background(), queryCreateView1H)
	if err != nil {
		fmt.Println(err)
	}
	_, err = dbpool.Exec(context.Background(), queryCreateView4H)
	if err != nil {
		fmt.Println(err)
	}
	_, err = dbpool.Exec(context.Background(), queryCreateView1D)
	if err != nil {
		fmt.Println(err)
	}
	_, err = dbpool.Exec(context.Background(), queryCreateView1W)
	if err != nil {
		fmt.Println(err)
	}

	_, err = dbpool.Exec(context.Background(), queryContinuousAgg15m)
	if err != nil {
		fmt.Println(err)
	}
	_, err = dbpool.Exec(context.Background(), queryContinuousAgg30m)
	if err != nil {
		fmt.Println(err)
	}
	_, err = dbpool.Exec(context.Background(), queryContinuousAgg1h)
	if err != nil {
		fmt.Println(err)
	}
	_, err = dbpool.Exec(context.Background(), queryContinuousAgg4h)
	if err != nil {
		fmt.Println(err)
	}
	_, err = dbpool.Exec(context.Background(), queryContinuousAgg1d)
	if err != nil {
		fmt.Println(err)
	}
	_, err = dbpool.Exec(context.Background(), queryContinuousAgg1w)
	if err != nil {
		fmt.Println(err)
	}
}

// select time_bucket('1h', time) AS oneHour, first(temperature,time) starttemp, last(temperature,time) lasttemp,avg(temperature) as Average,max(temperature) as maxtemp,min(temperature) as mintemp from sensor_data GROUP BY time_bucket('1h', time)
// SELECT add_continuous_aggregate_policy('temp_view',
//   start_offset => INTERVAL '1 week',
//   end_offset   => INTERVAL '1 hour',
//   schedule_interval => INTERVAL '30 minutes');
