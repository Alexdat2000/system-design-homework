-- Initialize ClickHouse OLAP database and base tables.

CREATE DATABASE IF NOT EXISTS analytics;

-- Track incremental ETL state (watermarks) inside ClickHouse for simplicity.
CREATE TABLE IF NOT EXISTS analytics.etl_state (
    pipeline String,
    last_value String,
    updated_at DateTime DEFAULT now()
)
ENGINE = ReplacingMergeTree(updated_at)
ORDER BY (pipeline);

-- DDS: orders (supports updates via updated_at).
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

-- DDS: payment_transactions (append-mostly).
CREATE TABLE IF NOT EXISTS analytics.dds_payment_transactions (
    id String,
    order_id String,
    user_id String,
    transaction_type String,
    amount Int32,
    status String,
    external_transaction_id Nullable(String),
    error_message Nullable(String),
    created_at DateTime
)
ENGINE = ReplacingMergeTree(created_at)
PARTITION BY toYYYYMM(created_at)
ORDER BY (id);

-- DDS: minute-level RPS aggregates for orders endpoints.
CREATE TABLE IF NOT EXISTS analytics.dds_orders_rps_minute (
    minute_ts DateTime,
    get_orders_count UInt64,
    post_orders_count UInt64,
    updated_at DateTime
)
ENGINE = ReplacingMergeTree(updated_at)
PARTITION BY toYYYYMM(minute_ts)
ORDER BY (minute_ts);


