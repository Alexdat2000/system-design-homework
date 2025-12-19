FROM apache/airflow:2.10.3

# Keep it minimal: only ClickHouse client needed for our custom DAGs.
USER airflow
RUN pip install --no-cache-dir "clickhouse-connect==0.7.19"


