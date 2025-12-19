-- MART tables for dashboarding (materialized via Airflow SQL tasks).

CREATE TABLE IF NOT EXISTS analytics.mart_rps_minute (
    minute_ts DateTime,
    get_rps Float64,
    post_rps Float64,
    get_orders_count UInt64,
    post_orders_count UInt64
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(minute_ts)
ORDER BY (minute_ts);

CREATE TABLE IF NOT EXISTS analytics.mart_orders_minute (
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


