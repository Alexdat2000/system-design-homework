from __future__ import annotations

from datetime import datetime

from airflow import DAG
from airflow.operators.python import PythonOperator

from pipeline_tasks import (
    build_mart,
    load_orders_dds,
    load_orders_rps_minute_dds,
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
    schedule=None,
    catchup=False,
    is_paused_upon_creation=False,
) as dag:
    t_orders = PythonOperator(task_id="load_orders_dds", python_callable=load_orders_dds)
    t_rps = PythonOperator(
        task_id="load_orders_rps_minute_dds", python_callable=load_orders_rps_minute_dds
    )

    t_mart_rps = PythonOperator(
        task_id="build_mart_rps_minute",
        python_callable=build_mart,
        op_kwargs={"sql_filename": "mart_rps_minute.sql"},
    )
    t_mart_orders = PythonOperator(
        task_id="build_mart_orders_minute",
        python_callable=build_mart,
        op_kwargs={"sql_filename": "mart_orders_minute.sql"},
    )

    marts = [
        t_mart_rps,
        t_mart_orders,
    ]

    for upstream in (t_orders, t_rps):
        upstream >> marts


