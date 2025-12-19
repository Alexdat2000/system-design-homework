from __future__ import annotations

from datetime import datetime

from airflow import DAG
from airflow.operators.python import PythonOperator

from pipeline_tasks import (
    build_marts,
    load_orders_dds,
    load_orders_rps_minute_dds,
    load_payments_dds,
)


default_args = {
    "owner": "airflow",
    "depends_on_past": False,
    "start_date": datetime(2024, 1, 1),
    "retries": 0,
}

with DAG(
    dag_id="full_pipeline_manual",
    default_args=default_args,
    description="Manual-only pipeline: Postgres -> ClickHouse DDS -> ClickHouse MARTs",
    schedule=None,  # manual only
    catchup=False,
    is_paused_upon_creation=False,
) as dag:
    # DDS loads (can run in parallel)
    t_orders = PythonOperator(task_id="load_orders_dds", python_callable=load_orders_dds)
    t_payments = PythonOperator(task_id="load_payments_dds", python_callable=load_payments_dds)
    t_rps = PythonOperator(
        task_id="load_orders_rps_minute_dds", python_callable=load_orders_rps_minute_dds
    )

    # MART build (sequential after all DDS loads finish)
    t_marts = PythonOperator(task_id="build_marts", python_callable=build_marts)

    [t_orders, t_payments, t_rps] >> t_marts


