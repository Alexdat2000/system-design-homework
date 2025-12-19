from __future__ import annotations

from datetime import datetime, timedelta
from pathlib import Path
import os
import logging

from airflow import DAG
from airflow.operators.python import PythonOperator


logger = logging.getLogger(__name__)


def _ch_client():
    import clickhouse_connect

    host = os.getenv("CLICKHOUSE_HOST", "clickhouse")
    port = int(os.getenv("CLICKHOUSE_HTTP_PORT", "8123"))
    database = os.getenv("CLICKHOUSE_DATABASE", "analytics")
    user = os.getenv("CLICKHOUSE_USER", "airflow")
    password = os.getenv("CLICKHOUSE_PASSWORD", "airflow")
    return clickhouse_connect.get_client(host=host, port=port, username=user, password=password, database=database)


def _run_sql_file(path: Path) -> None:
    sql = path.read_text(encoding="utf-8")
    ch = _ch_client()
    logger.info("running sql file: %s", path)
    # ClickHouse HTTP interface commonly disallows multi-statements.
    # Our SQL files can contain multiple statements separated by ';' (e.g. TRUNCATE + INSERT),
    # so execute them one by one.
    statements = [s.strip() for s in sql.split(";") if s.strip()]
    for stmt in statements:
        ch.command(stmt)


def build_marts() -> None:
    repo_root = Path(__file__).resolve().parents[1]
    sql_dir = repo_root / "sql" / "marts"

    for name in ["mart_rps_minute.sql", "mart_orders_daily.sql"]:
        _run_sql_file(sql_dir / name)


default_args = {
    "owner": "airflow",
    "depends_on_past": False,
    "start_date": datetime(2024, 1, 1),
    "retries": 1,
    "retry_delay": timedelta(minutes=2),
}

with DAG(
    dag_id="ch_transform_marts",
    default_args=default_args,
    description="Build dashboard-ready marts in ClickHouse from DDS",
    schedule_interval="*/10 * * * *",
    catchup=False,
    is_paused_upon_creation=False,
) as dag:
    PythonOperator(task_id="build_marts", python_callable=build_marts)


