from __future__ import annotations

import logging
import os
from pathlib import Path
from typing import List

from airflow.providers.postgres.hooks.postgres import PostgresHook

logger = logging.getLogger(__name__)


def ch_client():
    import clickhouse_connect  # installed in custom Airflow image

    host = os.getenv("CLICKHOUSE_HOST", "clickhouse")
    port = int(os.getenv("CLICKHOUSE_HTTP_PORT", "8123"))
    database = os.getenv("CLICKHOUSE_DATABASE", "analytics")
    user = os.getenv("CLICKHOUSE_USER", "airflow")
    password = os.getenv("CLICKHOUSE_PASSWORD", "airflow")
    return clickhouse_connect.get_client(
        host=host, port=port, username=user, password=password, database=database
    )


def get_watermark(ch, pipeline: str, default_value: str) -> str:
    rows = ch.query(
        "SELECT last_value FROM analytics.etl_state WHERE pipeline = %(p)s "
        "ORDER BY updated_at DESC LIMIT 1",
        parameters={"p": pipeline},
    ).result_rows
    if not rows:
        return default_value
    return rows[0][0]


def set_watermark(ch, pipeline: str, value: str) -> None:
    ch.command(
        "INSERT INTO analytics.etl_state (pipeline, last_value) VALUES (%(p)s, %(v)s)",
        parameters={"p": pipeline, "v": value},
    )


def load_orders_dds(batch_size: int = 5000) -> None:
    """
    Incrementally load orders from Postgres into ClickHouse DDS by orders.updated_at watermark.
    """
    ch = ch_client()
    pg = PostgresHook(postgres_conn_id="postgres_default")

    watermark = get_watermark(ch, "orders.updated_at", "1970-01-01 00:00:00.000000")
    logger.info("orders watermark: %s", watermark)

    total = 0
    while True:
        with pg.get_conn() as conn:
            with conn.cursor() as cur:
                cur.execute(
                    """
                    SELECT
                        id, user_id, scooter_id, offer_id,
                        price_per_minute, price_unlock, deposit, total_amount,
                        status, start_time, finish_time, duration_seconds,
                        created_at, updated_at
                    FROM orders
                    WHERE updated_at > %s
                    ORDER BY updated_at ASC
                    LIMIT %s
                    """,
                    (watermark, batch_size),
                )
                rows = cur.fetchall()

        if not rows:
            break

        ch.insert(
            "analytics.dds_orders",
            rows,
            column_names=[
                "id",
                "user_id",
                "scooter_id",
                "offer_id",
                "price_per_minute",
                "price_unlock",
                "deposit",
                "total_amount",
                "status",
                "start_time",
                "finish_time",
                "duration_seconds",
                "created_at",
                "updated_at",
            ],
        )

        total += len(rows)
        watermark = rows[-1][-1].strftime("%Y-%m-%d %H:%M:%S.%f")
        set_watermark(ch, "orders.updated_at", watermark)

    logger.info("orders loaded rows: %d", total)


def load_payments_dds(batch_size: int = 5000) -> None:
    """
    Incrementally load payment_transactions from Postgres into ClickHouse DDS by created_at watermark.
    """
    ch = ch_client()
    pg = PostgresHook(postgres_conn_id="postgres_default")

    watermark = get_watermark(
        ch, "payment_transactions.created_at", "1970-01-01 00:00:00.000000"
    )
    logger.info("payment_transactions watermark: %s", watermark)

    total = 0
    while True:
        with pg.get_conn() as conn:
            with conn.cursor() as cur:
                cur.execute(
                    """
                    SELECT
                        id, order_id, user_id,
                        transaction_type, amount, status,
                        external_transaction_id, error_message,
                        created_at
                    FROM payment_transactions
                    WHERE created_at > %s
                    ORDER BY created_at ASC
                    LIMIT %s
                    """,
                    (watermark, batch_size),
                )
                rows = cur.fetchall()

        if not rows:
            break

        ch.insert(
            "analytics.dds_payment_transactions",
            rows,
            column_names=[
                "id",
                "order_id",
                "user_id",
                "transaction_type",
                "amount",
                "status",
                "external_transaction_id",
                "error_message",
                "created_at",
            ],
        )

        total += len(rows)
        watermark = rows[-1][-1].strftime("%Y-%m-%d %H:%M:%S.%f")
        set_watermark(ch, "payment_transactions.created_at", watermark)

    logger.info("payment_transactions loaded rows: %d", total)


def load_orders_rps_minute_dds(batch_size: int = 5000) -> None:
    """
    Incrementally load orders_rps_minute from Postgres into ClickHouse DDS by minute_ts watermark.
    """
    ch = ch_client()
    pg = PostgresHook(postgres_conn_id="postgres_default")

    watermark = get_watermark(ch, "orders_rps_minute.minute_ts", "1970-01-01 00:00:00")
    logger.info("orders_rps_minute watermark: %s", watermark)

    total = 0
    while True:
        with pg.get_conn() as conn:
            with conn.cursor() as cur:
                cur.execute(
                    """
                    SELECT
                        minute_ts,
                        get_orders_count,
                        post_orders_count,
                        updated_at
                    FROM orders_rps_minute
                    WHERE minute_ts > %s
                    ORDER BY minute_ts ASC
                    LIMIT %s
                    """,
                    (watermark, batch_size),
                )
                rows = cur.fetchall()

        if not rows:
            break

        ch.insert(
            "analytics.dds_orders_rps_minute",
            rows,
            column_names=["minute_ts", "get_orders_count", "post_orders_count", "updated_at"],
        )

        total += len(rows)
        watermark = rows[-1][0].strftime("%Y-%m-%d %H:%M:%S")
        set_watermark(ch, "orders_rps_minute.minute_ts", watermark)

    logger.info("orders_rps_minute loaded rows: %d", total)


def _split_sql(sql: str) -> List[str]:
    return [s.strip() for s in sql.split(";") if s.strip()]


def run_sql_file_in_clickhouse(path: Path) -> None:
    sql = path.read_text(encoding="utf-8")
    statements = _split_sql(sql)
    ch = ch_client()
    logger.info("running sql file: %s (%d statements)", path, len(statements))
    for stmt in statements:
        ch.command(stmt)


def build_marts() -> None:
    # Airflow container has repo SQL mounted at /opt/airflow/sql
    base = Path("/opt/airflow/sql/marts")
    for name in ["mart_rps_minute.sql", "mart_orders_daily.sql"]:
        run_sql_file_in_clickhouse(base / name)


