TRUNCATE TABLE IF EXISTS analytics.mart_rps_minute;

INSERT INTO analytics.mart_rps_minute
SELECT
    minute_ts,
    get_orders_count / 60.0 AS get_rps,
    post_orders_count / 60.0 AS post_rps,
    get_orders_count,
    post_orders_count
FROM analytics.dds_orders_rps_minute
ORDER BY minute_ts ASC;


