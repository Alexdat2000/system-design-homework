#!/usr/bin/env python3
import os
import uuid
import time
import argparse
import logging
from concurrent.futures import ThreadPoolExecutor, as_completed

import requests


# ----------------- CONFIG -----------------

PROJECT_ROOT = os.path.dirname(os.path.abspath(__file__))
CLIENT_SERVICE_URL = os.environ.get("CLIENT_SERVICE_URL", "http://localhost:8080")


# ----------------- HTTP CLIENTS -----------------


class ServiceClient:
    """HTTP client wrapper for testing services."""

    def __init__(self, base_url: str):
        self.base_url = base_url.rstrip("/")
        self.session = requests.Session()

    def health_check(self) -> bool:
        """Check if service is healthy."""
        try:
            response = self.session.get(f"{self.base_url}/health", timeout=5)
            return response.status_code == 200
        except requests.exceptions.RequestException:
            return False

    def post(self, path: str, json: dict = None, **kwargs) -> requests.Response:
        """Make POST request."""
        return self.session.post(f"{self.base_url}{path}", json=json, **kwargs)

    def get(self, path: str, **kwargs) -> requests.Response:
        """Make GET request."""
        return self.session.get(f"{self.base_url}{path}", **kwargs)

    def close(self):
        """Close session."""
        self.session.close()


class ClientServiceClient(ServiceClient):
    """Client service specific methods."""

    def create_offer(self, user_id: str, scooter_id: str) -> requests.Response:
        """POST /offers - Create an offer."""
        return self.post("/offers", json={
            "user_id": user_id,
            "scooter_id": scooter_id
        })

    def create_order(self, order_id: str, offer_id: str, user_id: str) -> requests.Response:
        """POST /orders - Create an order from an offer."""
        return self.post("/orders", json={
            "order_id": order_id,
            "offer_id": offer_id,
            "user_id": user_id
        })

    def get_order(self, order_id: str) -> requests.Response:
        """GET /orders/{order_id} - Get order information."""
        return self.get(f"/orders/{order_id}")

    def finish_order(self, order_id: str) -> requests.Response:
        """POST /orders/{order_id}/finish - Finish an order."""
        return self.post(f"/orders/{order_id}/finish")


# ----------------- LOAD SCENARIO -----------------


def run_order_scenario(
    scenario_id,
    user_id: str,
    gets_per_order: int = 100,
) -> dict:
    """
    Сценарий одного заказа для одного пользователя (всё строго линейно):

    1. Создать оффер
    2. Создать заказ
    3. gets_per_order раз сделать GET /orders/{order_id}
    4. Завершить заказ

    Для каждого сценария создаётся:
      - новый уникальный scooter_id,
      - новый оффер (offer_id),
      - новый заказ (order_id).
    """
    client = ClientServiceClient(CLIENT_SERVICE_URL)

    result = {
        "scenario_id": scenario_id,
        "success": False,
        "error": None,
    }

    # Уникальный самокат для данного сценария
    scooter_id = f"scooter-{user_id}-{uuid.uuid4()}"

    try:
        # 1. Создать оффер (НОВЫЙ для каждого сценария)
        offer_resp = client.create_offer(user_id=user_id, scooter_id=scooter_id)
        if offer_resp.status_code != 201:
            raise RuntimeError(
                f"create_offer failed: {offer_resp.status_code}, body={offer_resp.text}"
            )

        offer = offer_resp.json()
        offer_id = offer["id"]

        # 2. Создать заказ (РОВНО ОДИН раз для этого offer_id)
        order_id = f"order-{uuid.uuid4()}"
        order_resp = client.create_order(order_id=order_id, offer_id=offer_id, user_id=user_id)
        if order_resp.status_code != 201:
            raise RuntimeError(
                f"create_order failed: {order_resp.status_code}, body={order_resp.text}"
            )

        # 3. gets_per_order раз GET /orders/{order_id}
        for i in range(gets_per_order):
            get_resp = client.get_order(order_id=order_id)
            if get_resp.status_code != 200:
                raise RuntimeError(
                    f"get_order failed on iter {i}: {get_resp.status_code}, body={get_resp.text}"
                )

        # 4. Завершить заказ
        finish_resp = client.finish_order(order_id=order_id)
        if finish_resp.status_code not in (200, 409):
            # 409 - заказ уже завершён, тоже допустимо
            raise RuntimeError(
                f"finish_order failed: {finish_resp.status_code}, body={finish_resp.text}"
            )

        result["success"] = True
        return result

    except Exception as e:
        result["error"] = str(e)
        logging.exception("Scenario %s failed", scenario_id)
        return result

    finally:
        client.close()


def run_user_scenarios(
    user_index: int,
    orders_for_user: int,
    gets_per_order: int,
) -> dict:
    """
    Выполнить несколько сценариев (заказов) подряд для одного пользователя.
    Для одного пользователя все заказы идут строго линейно.
    """
    user_id = f"user-{user_index}"

    successes = 0
    failures = 0

    for order_idx in range(orders_for_user):
        scenario_id = f"user-{user_index}-order-{order_idx}"
        res = run_order_scenario(
            scenario_id=scenario_id,
            user_id=user_id,
            gets_per_order=gets_per_order,
        )
        if res["success"]:
            successes += 1
        else:
            failures += 1
            logging.warning(
                "User %s, scenario %s failed: %s",
                user_id, scenario_id, res["error"]
            )

    return {
        "user_index": user_index,
        "successes": successes,
        "failures": failures,
    }


# ----------------- MAIN -----------------


def main():
    parser = argparse.ArgumentParser(
        description="Нагрузочный тест клиентского сервиса самокатов"
    )
    parser.add_argument(
        "--concurrency",
        type=int,
        default=20,
        help="Количество параллельных пользователей",
    )
    parser.add_argument(
        "--orders",
        type=int,
        default=100,
        help="Общее количество сценариев (заказов), которые нужно выполнить",
    )
    parser.add_argument(
        "--gets-per-order",
        type=int,
        default=100,
        help="Количество GET /orders/{id} на один заказ",
    )
    parser.add_argument(
        "--log-level",
        default="INFO",
        help="Уровень логирования (DEBUG, INFO, WARNING, ERROR)",
    )
    args = parser.parse_args()

    logging.basicConfig(
        level=getattr(logging, args.log_level.upper(), logging.INFO),
        format="%(asctime)s [%(levelname)s] %(message)s",
    )

    logging.info("CLIENT_SERVICE_URL=%s", CLIENT_SERVICE_URL)

    start_time = time.time()

    total_orders = args.orders
    users = args.concurrency

    # Распределяем общее количество заказов по пользователям
    base_orders_per_user = total_orders // users if users > 0 else 0
    remainder = total_orders % users if users > 0 else 0

    logging.info(
        "Users=%d, total_orders=%d, base_orders_per_user=%d, remainder=%d",
        users, total_orders, base_orders_per_user, remainder,
    )

    successes = 0
    failures = 0

    with ThreadPoolExecutor(max_workers=users) as executor:
        futures = []
        for user_index in range(users):
            orders_for_user = base_orders_per_user + (1 if user_index < remainder else 0)
            if orders_for_user == 0:
                continue
            futures.append(
                executor.submit(
                    run_user_scenarios,
                    user_index,
                    orders_for_user,
                    args.gets_per_order,
                )
            )

        for f in as_completed(futures):
            res = f.result()
            successes += res["successes"]
            failures += res["failures"]

    elapsed = time.time() - start_time

    logging.info(
        "Done. total_orders=%d, successes=%d, failures=%d, time=%.2fs",
        total_orders, successes, failures, elapsed,
    )
    if elapsed > 0:
        logging.info("Throughput: %.2f scenarios/sec", total_orders / elapsed)


if __name__ == "__main__":
    main()
