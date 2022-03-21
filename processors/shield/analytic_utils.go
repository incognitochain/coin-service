package shield

import "fmt"

// generate ContinuousAggregationQuery
func genCAViewQuery(view, table, period string) string {
	return fmt.Sprintf(`CREATE MATERIALIZED VIEW %v WITH (timescaledb.continuous, timescaledb.materialized_only = true) AS select time_bucket('%v', time) AS period, sum(actual_token1_amount) as total_contribute_tk1, sum(actual_token2_amount) as total_contribute_tk2, pool_id from %v GROUP BY period, pool_id`, view, period, table)
}

func genConAggPolicyQuery(view string) string {
	return fmt.Sprintf(`SELECT add_continuous_aggregate_policy('%v',
	start_offset => INTERVAL '1 month',
	end_offset => INTERVAL '1 minute',
	schedule_interval => INTERVAL '1 minute');`, view)
}

func genRetentionPolicyQuery(view, days string) string {
	return fmt.Sprintf(`SELECT add_retention_policy('%v', INTERVAL '%v days');`, view, days)
}

func genCompressPolicyQuery(view, days string) string {
	return fmt.Sprintf(`SELECT add_compression_policy('%v', compress_after=>'%v days'::interval);`, view, days)
}

// SELECT time_bucket_gapfill('15 minute', period) as bucket,
// avg(open) as open,avg(close) as close, avg(high) as high, avg(low) as low,
// count(*) as datas,
// sell_pool_id
// FROM trade_price_15m
// WHERE period >= '2022-03-01' AND period < '2022-03-14'
// GROUP BY bucket,sell_pool_id ORDER BY sell_pool_id DESC,bucket DESC,datas DESC;
// SELECT *
// FROM trade_price_15m
// WHERE period >= '2022-01-01' AND period < '2022-03-14' ORDER BY sell_pool_id DESC,period DESC;
// CALL refresh_continuous_aggregate('trade_price_1d', NULL, localtimestamp - INTERVAL '2 month');
