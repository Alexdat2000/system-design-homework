CREATE TABLE IF NOT EXISTS analytics.mart_revenue_minute (
    minute_ts DateTime,
    rides_finished UInt64,
    revenue_total Int64,
    avg_revenue_per_ride Float64
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(minute_ts)
ORDER BY (minute_ts);

TRUNCATE TABLE analytics.mart_revenue_minute;

INSERT INTO analytics.mart_revenue_minute
WITH latest AS (
    SELECT
        id,
        argMax(status, updated_at) AS status,
        argMax(total_amount, updated_at) AS total_amount,
        argMax(finish_time, updated_at) AS finish_time
    FROM analytics.dds_orders
    GROUP BY id
)
SELECT
    toStartOfMinute(finish_time) AS minute_ts,
    count() AS rides_finished,
    sum(toInt64(total_amount)) AS revenue_total,
    if(rides_finished = 0, 0.0, revenue_total / toFloat64(rides_finished)) AS avg_revenue_per_ride
FROM latest
WHERE status = 'FINISHED' AND finish_time IS NOT NULL
GROUP BY minute_ts
ORDER BY minute_ts ASC;


