from __future__ import annotations

import logging
import os
from pathlib import Path
from typing import List

from airflow.providers.postgres.hooks.postgres import PostgresHook

logger = logging.getLogger(__name__)


def ch_client():
    import clickhouse_connect

    host = os.getenv("CLICKHOUSE_HOST", "clickhouse")
    port = int(os.getenv("CLICKHOUSE_HTTP_PORT", "8123"))
    database = os.getenv("CLICKHOUSE_DATABASE", "analytics")
    user = os.getenv("CLICKHOUSE_USER", "airflow")
    password = os.getenv("CLICKHOUSE_PASSWORD", "airflow")
    return clickhouse_connect.get_client(
        host=host, port=port, username=user, password=password, database=database
    )


def _truncate(ch, table: str) -> None:
    ch.command(f"TRUNCATE TABLE IF EXISTS {table}")


def load_orders_dds(batch_size: int = 5000) -> None:
    ch = ch_client()
    pg = PostgresHook(postgres_conn_id="postgres_default")

    _truncate(ch, "analytics.dds_orders")
    watermark = "1970-01-01 00:00:00.000000"

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

    logger.info("orders loaded rows: %d", total)

def load_orders_rps_minute_dds(batch_size: int = 5000) -> None:
    ch = ch_client()
    pg = PostgresHook(postgres_conn_id="postgres_default")

    _truncate(ch, "analytics.dds_orders_rps_minute")
    watermark = "1970-01-01 00:00:00"

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

    logger.info("orders_rps_minute loaded rows: %d", total)


def _split_sql(sql: str) -> List[str]:
    filtered_lines: List[str] = []
    for line in sql.splitlines():
        stripped = line.strip()
        if not stripped:
            continue
        if stripped.startswith("--"):
            continue
        filtered_lines.append(line)
    filtered = "\n".join(filtered_lines)
    return [s.strip() for s in filtered.split(";") if s.strip()]


def run_sql_file_in_clickhouse(path: Path) -> None:
    sql = path.read_text(encoding="utf-8")
    statements = _split_sql(sql)
    ch = ch_client()
    logger.info("running sql file: %s (%d statements)", path, len(statements))
    for stmt in statements:
        ch.command(stmt)


def build_mart(sql_filename: str) -> None:
    base = Path("/opt/airflow/sql/marts")
    run_sql_file_in_clickhouse(base / sql_filename)


