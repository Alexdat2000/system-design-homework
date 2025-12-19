CREATE TABLE IF NOT EXISTS analytics.mart_duration_minute (
    minute_ts DateTime,
    rides_finished UInt64,
    avg_duration_seconds Float64,
    p50_duration_seconds Float64,
    p90_duration_seconds Float64
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(minute_ts)
ORDER BY (minute_ts);

TRUNCATE TABLE analytics.mart_duration_minute;

INSERT INTO analytics.mart_duration_minute
WITH latest AS (
    SELECT
        id,
        argMax(status, updated_at) AS status,
        argMax(duration_seconds, updated_at) AS duration_seconds,
        argMax(finish_time, updated_at) AS finish_time
    FROM analytics.dds_orders
    GROUP BY id
)
SELECT
    toStartOfMinute(finish_time) AS minute_ts,
    count() AS rides_finished,
    avg(toFloat64(duration_seconds)) AS avg_duration_seconds,
    quantileExact(0.50)(toFloat64(duration_seconds)) AS p50_duration_seconds,
    quantileExact(0.90)(toFloat64(duration_seconds)) AS p90_duration_seconds
FROM latest
WHERE status = 'FINISHED' AND finish_time IS NOT NULL AND duration_seconds IS NOT NULL
GROUP BY minute_ts
ORDER BY minute_ts ASC;


