TRUNCATE TABLE IF EXISTS analytics.mart_orders_daily;

INSERT INTO analytics.mart_orders_daily
WITH latest AS (
    SELECT
        id,
        argMax(status, updated_at) AS status,
        argMax(total_amount, updated_at) AS total_amount,
        argMax(created_at, updated_at) AS created_at
    FROM analytics.dds_orders
    GROUP BY id
)
SELECT
    toDate(created_at) AS day,
    count() AS orders_total,
    countIf(status = 'FINISHED') AS orders_finished,
    countIf(status = 'ACTIVE') AS orders_active,
    countIf(status = 'CANCELLED') AS orders_cancelled,
    countIf(status = 'PAYMENT_FAILED') AS orders_payment_failed,
    sumIf(toInt64(total_amount), status = 'FINISHED') AS revenue_total
FROM latest
GROUP BY day
ORDER BY day ASC;


