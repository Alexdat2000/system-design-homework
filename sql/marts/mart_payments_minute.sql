CREATE TABLE IF NOT EXISTS analytics.mart_payments_minute (
    minute_ts DateTime,
    tx_total UInt64,
    tx_success UInt64,
    tx_failed UInt64,
    tx_pending UInt64,
    amount_total Int64,
    amount_hold Int64,
    amount_clear Int64,
    amount_refund Int64
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(minute_ts)
ORDER BY (minute_ts);

TRUNCATE TABLE analytics.mart_payments_minute;

INSERT INTO analytics.mart_payments_minute
SELECT
    toStartOfMinute(created_at) AS minute_ts,
    count() AS tx_total,
    countIf(status = 'SUCCESS') AS tx_success,
    countIf(status = 'FAILED') AS tx_failed,
    countIf(status = 'PENDING') AS tx_pending,
    sum(toInt64(amount)) AS amount_total,
    sumIf(toInt64(amount), transaction_type = 'HOLD') AS amount_hold,
    sumIf(toInt64(amount), transaction_type = 'CLEAR') AS amount_clear,
    sumIf(toInt64(amount), transaction_type = 'REFUND') AS amount_refund
FROM analytics.dds_payment_transactions
GROUP BY minute_ts
ORDER BY minute_ts ASC;


