package trade

import "fmt"

// generate ContinuousAggregationQuery
func genCAViewQuery(view, table, period string) string {
	return fmt.Sprintf(`CREATE MATERIALIZED VIEW %v WITH (timescaledb.continuous) AS select time_bucket('%v', time) AS period, first(rate,time) open, last(rate,time) "close",avg(rate) as Average,max(rate) as high,min(rate) as low, sum(rate) as volume, count(*) as "set", sum(actual_token1_amount) as volume_tk1, sum(actual_token2_amount) as volume_tk2, pool_id from %v GROUP BY period, pool_id WITH NO DATA`, view, period, table)
}

func genConAggPolicyQuery(view string) string {
	return fmt.Sprintf(`SELECT add_continuous_aggregate_policy('%v',
	start_offset => INTERVAL '1 day',
	end_offset   => INTERVAL '1 minute',
	schedule_interval => INTERVAL '1 minutes');`, view)
}

func genRetentionPolicyQuery(view, days string) string {
	return fmt.Sprintf(`SELECT add_retention_policy('%v', INTERVAL '%v days');`, view, days)
}

func genCompressPolicyQuery(view, days string) string {
	return fmt.Sprintf(`SELECT add_compression_policy('%v', compress_after=>'%v days'::interval);`, view, days)
}
