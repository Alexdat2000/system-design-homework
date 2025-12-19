CREATE TABLE IF NOT EXISTS analytics.mart_pricing_minute (
    minute_ts DateTime,
    offers_avg_price_per_minute Float64,
    offers_avg_price_unlock Float64,
    offers_avg_deposit Float64,
    offers_count UInt64
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(minute_ts)
ORDER BY (minute_ts);

TRUNCATE TABLE analytics.mart_pricing_minute;

INSERT INTO analytics.mart_pricing_minute
WITH latest AS (
    SELECT
        id,
        argMax(price_per_minute, updated_at) AS price_per_minute,
        argMax(price_unlock, updated_at) AS price_unlock,
        argMax(deposit, updated_at) AS deposit,
        argMax(created_at, updated_at) AS created_at
    FROM analytics.dds_orders
    GROUP BY id
)
SELECT
    toStartOfMinute(created_at) AS minute_ts,
    avg(toFloat64(price_per_minute)) AS offers_avg_price_per_minute,
    avg(toFloat64(price_unlock)) AS offers_avg_price_unlock,
    avg(toFloat64(deposit)) AS offers_avg_deposit,
    count() AS offers_count
FROM latest
GROUP BY minute_ts
ORDER BY minute_ts ASC;


