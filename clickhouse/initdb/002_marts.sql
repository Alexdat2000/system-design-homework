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

CREATE TABLE IF NOT EXISTS analytics.mart_orders_daily (
    day Date,
    orders_total UInt64,
    orders_finished UInt64,
    orders_active UInt64,
    orders_cancelled UInt64,
    orders_payment_failed UInt64,
    revenue_total Int64
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(day)
ORDER BY (day);


