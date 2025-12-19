from datetime import datetime, timedelta
import logging

from airflow import DAG
from airflow.operators.python import PythonOperator
from airflow.providers.postgres.hooks.postgres import PostgresHook


# Настройка логирования
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Параметры DAG
default_args = {
    'owner': 'airflow',
    'depends_on_past': False,
    'start_date': datetime(2024, 1, 1),
    'email_on_failure': False,
    'email_on_retry': False,
    'retries': 1,
    'retry_delay': timedelta(minutes=5),
}

dag = DAG(
    'read_and_log_orders',
    default_args=default_args,
    description='Читает данные из orders и payment_transactions и выводит в лог',
    schedule_interval='@daily',
    catchup=False,
    is_paused_upon_creation=False,
)

def log_recent_orders():
    """Читает 10 последних заказов и выводит в лог."""
    hook = PostgresHook(postgres_conn_id='postgres_default')
    connection = hook.get_conn()
    cursor = connection.cursor()

    query = """
        SELECT
            id, user_id, scooter_id, offer_id, total_amount, status, start_time, finish_time
        FROM orders
        ORDER BY created_at DESC
        LIMIT 10;
    """
    cursor.execute(query)
    rows = cursor.fetchall()

    logger.info("=== Последние 10 заказов ===")
    if rows:
        for row in rows:
            logger.info(f"Order ID: {row[0]}, User ID: {row[1]}, "
                       f"Scooter ID: {row[2]}, Offer ID: {row[3]}, "
                       f"Total: {row[4]}, Status: {row[5]}, "
                       f"Start: {row[6]}, Finish: {row[7]}")
    else:
        logger.info("Нет данных в таблице orders.")

    cursor.close()
    connection.close()

def log_recent_payments():
    """Читает платежи за последние 24 часа и выводит в лог."""
    hook = PostgresHook(postgres_conn_id='postgres_default')
    connection = hook.get_conn()
    cursor = connection.cursor()

    query = """
        SELECT
            id, order_id, user_id, transaction_type, amount, status, created_at
        FROM payment_transactions
        WHERE created_at >= NOW() - INTERVAL '24 hours'
        ORDER BY created_at DESC;
    """
    cursor.execute(query)
    rows = cursor.fetchall()

    logger.info("=== Платежи за последние 24 часа ===")
    if rows:
        for row in rows:
            logger.info(f"Transaction ID: {row[0]}, Order ID: {row[1]}, "
                       f"User ID: {row[2]}, Type: {row[3]}, "
                       f"Amount: {row[4]}, Status: {row[5]}, Created: {row[6]}")
    else:
        logger.info("Нет данных в payment_transactions за последние 24 часа.")

    cursor.close()
    connection.close()

def log_order_stats():
    """Выводит статистику по статусам заказов."""
    hook = PostgresHook(postgres_conn_id='postgres_default')
    connection = hook.get_conn()
    cursor = connection.cursor()

    query = """
        SELECT
            status, COUNT(*) AS count, SUM(total_amount) AS total_revenue
        FROM orders
        GROUP BY status
        ORDER BY count DESC;
    """
    cursor.execute(query)
    rows = cursor.fetchall()

    logger.info("=== Статистика по статусам заказов ===")
    if rows:
        for row in rows:
            logger.info(f"Status: {row[0]}, Count: {row[1]}, Revenue: {row[2]}")
    else:
        logger.info("Нет данных для агрегации.")

    cursor.close()
    connection.close()

# Задачи DAG
task_log_orders = PythonOperator(
    task_id='log_recent_orders',
    python_callable=log_recent_orders,
    dag=dag,
)

task_log_payments = PythonOperator(
    task_id='log_recent_payments',
    python_callable=log_recent_payments,
    dag=dag,
)

task_log_stats = PythonOperator(
    task_id='log_order_stats',
    python_callable=log_order_stats,
    dag=dag,
)

# Порядок выполнения
task_log_orders >> task_log_payments >> task_log_stats
