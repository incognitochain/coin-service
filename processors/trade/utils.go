package trade

import "fmt"

// generate ContinuousAggregationQuery
func genCAViewQuery(view, table, period string) string {
	return fmt.Sprintf(`CREATE MATERIALIZED VIEW %v WITH (timescaledb.continuous) AS select time_bucket('%v', time) AS period, first(price,time) open, last(price,time) "close",avg(price) as Average,max(price) as high,min(price) as low, count(*) as "set" from %v GROUP BY period WITH NO DATA`, view, period, table)
}

func genAutoCAViewQuery(view string) string {
	return fmt.Sprintf(`SELECT add_continuous_aggregate_policy('%v',
	start_offset => INTERVAL '1 week',
	end_offset   => INTERVAL '1 minute',
	schedule_interval => INTERVAL '1 minutes');`, view)
}
