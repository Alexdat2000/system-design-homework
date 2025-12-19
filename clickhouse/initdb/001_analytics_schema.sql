CREATE DATABASE IF NOT EXISTS analytics;

CREATE TABLE IF NOT EXISTS analytics.dds_orders (
    id String,
    user_id String,
    scooter_id String,
    offer_id String,
    price_per_minute Int32,
    price_unlock Int32,
    deposit Int32,
    total_amount Int32,
    status String,
    start_time DateTime,
    finish_time Nullable(DateTime),
    duration_seconds Nullable(Int32),
    created_at DateTime,
    updated_at DateTime
)
ENGINE = ReplacingMergeTree(updated_at)
PARTITION BY toYYYYMM(updated_at)
ORDER BY (id);

CREATE TABLE IF NOT EXISTS analytics.dds_orders_rps_minute (
    minute_ts DateTime,
    get_orders_count UInt64,
    post_orders_count UInt64,
    updated_at DateTime
)
ENGINE = ReplacingMergeTree(updated_at)
PARTITION BY toYYYYMM(minute_ts)
ORDER BY (minute_ts);


