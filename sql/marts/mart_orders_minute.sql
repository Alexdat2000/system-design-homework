DROP TABLE IF EXISTS analytics.mart_orders_minute;

CREATE TABLE analytics.mart_orders_minute (
    minute_ts DateTime,
    orders_total UInt64,
    orders_finished UInt64,
    orders_active UInt64,
    orders_cancelled UInt64,
    orders_payment_failed UInt64,
    revenue_total Int64,
    avg_duration_seconds_finished Float64,
    avg_total_amount_finished Float64,
    avg_price_unlock_finished Float64,
    avg_price_per_minute_finished Float64
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(minute_ts)
ORDER BY (minute_ts);

INSERT INTO analytics.mart_orders_minute
WITH latest AS (
    SELECT
        id,
        argMax(status, updated_at) AS status,
        argMax(total_amount, updated_at) AS total_amount,
        argMax(duration_seconds, updated_at) AS duration_seconds,
        argMax(price_unlock, updated_at) AS price_unlock,
        argMax(price_per_minute, updated_at) AS price_per_minute,
        argMax(created_at, updated_at) AS created_at
    FROM analytics.dds_orders
    GROUP BY id
)
SELECT
    toStartOfMinute(created_at) AS minute_ts,
    count() AS orders_total,
    countIf(status = 'FINISHED') AS orders_finished,
    countIf(status = 'ACTIVE') AS orders_active,
    countIf(status = 'CANCELLED') AS orders_cancelled,
    countIf(status = 'PAYMENT_FAILED') AS orders_payment_failed,
    sumIf(toInt64(total_amount), status = 'FINISHED') AS revenue_total,
    avgIf(toFloat64(duration_seconds), status = 'FINISHED' AND duration_seconds IS NOT NULL) AS avg_duration_seconds_finished,
    avgIf(toFloat64(total_amount), status = 'FINISHED') AS avg_total_amount_finished,
    avgIf(toFloat64(price_unlock), status = 'FINISHED') AS avg_price_unlock_finished,
    avgIf(toFloat64(price_per_minute), status = 'FINISHED') AS avg_price_per_minute_finished
FROM latest
GROUP BY minute_ts
ORDER BY minute_ts ASC;
