#!/usr/bin/env python3
import os
import uuid
import time
import argparse
import logging
from concurrent.futures import ThreadPoolExecutor, as_completed

import requests


PROJECT_ROOT = os.path.dirname(os.path.abspath(__file__))
CLIENT_SERVICE_URL = os.environ.get("CLIENT_SERVICE_URL", "http://localhost:8080")


class ServiceClient:
    """HTTP client wrapper for testing services."""

    def __init__(self, base_url: str):
        self.base_url = base_url.rstrip("/")
        adapter = requests.adapters.HTTPAdapter(
            pool_connections=100,   
            pool_maxsize=200,           
            max_retries=0,              
            pool_block=False,           
        )
        self.session = requests.Session()
        self.session.mount('http://', adapter)
        self.session.mount('https://', adapter)
        self.timeout = 10

    def health_check(self) -> bool:
        """Check if service is healthy."""
        try:
            response = self.session.get(f"{self.base_url}/health", timeout=self.timeout)
            return response.status_code == 200
        except requests.exceptions.RequestException:
            return False

    def post(self, path: str, json: dict = None, **kwargs) -> requests.Response:
        """Make POST request."""
        if 'timeout' not in kwargs:
            kwargs['timeout'] = self.timeout
        return self.session.post(f"{self.base_url}{path}", json=json, **kwargs)

    def get(self, path: str, **kwargs) -> requests.Response:
        """Make GET request."""
        if 'timeout' not in kwargs:
            kwargs['timeout'] = self.timeout
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


def run_order_scenario(
    scenario_id,
    user_id: str,
    gets_per_order: int = 100,
) -> dict:
    client = ClientServiceClient(CLIENT_SERVICE_URL)
    try:
        return run_order_scenario_with_client(client, scenario_id, user_id, gets_per_order)
    finally:
        client.close()


def run_order_scenario_with_client(
    client: ClientServiceClient,
    scenario_id: str,
    user_id: str,
    gets_per_order: int = 100,
    rate_limiter=None,
) -> dict:
    result = {
        "scenario_id": scenario_id,
        "success": False,
        "error": None,
    }

    scooter_id = f"load-scooter-{user_id}-{uuid.uuid4()}"

    try:
        if rate_limiter:
            rate_limiter()
        
        offer_resp = client.create_offer(user_id=user_id, scooter_id=scooter_id)
        if offer_resp.status_code != 201:
            raise RuntimeError(
                f"create_offer failed: {offer_resp.status_code}, body={offer_resp.text}"
            )

        offer = offer_resp.json()
        offer_id = offer["id"]

        if rate_limiter:
            rate_limiter()
        order_id = f"order-{uuid.uuid4()}"
        order_resp = client.create_order(order_id=order_id, offer_id=offer_id, user_id=user_id)
        if order_resp.status_code != 201:
            raise RuntimeError(
                f"create_order failed: {order_resp.status_code}, body={order_resp.text}"
            )

        for i in range(gets_per_order):
            if rate_limiter:
                rate_limiter()
            get_resp = client.get_order(order_id=order_id)
            if get_resp.status_code != 200:
                raise RuntimeError(
                    f"get_order failed on iter {i}: {get_resp.status_code}, body={get_resp.text}"
                )

        if rate_limiter:
            rate_limiter()
        finish_resp = client.finish_order(order_id=order_id)
        if finish_resp.status_code not in (200, 409):
            raise RuntimeError(
                f"finish_order failed: {finish_resp.status_code}, body={finish_resp.text}"
            )

        result["success"] = True
        return result

    except Exception as e:
        result["error"] = str(e)
        logging.exception("Scenario %s failed", scenario_id)
        return result


def run_user_scenarios(
    user_index: int,
    orders_for_user: int,
    gets_per_order: int,
    rate_limiter=None,
) -> dict:
    user_id = f"load-user-{user_index}"
    client = ClientServiceClient(CLIENT_SERVICE_URL)

    successes = 0
    failures = 0

    try:
        for order_idx in range(orders_for_user):
            scenario_id = f"user-{user_index}-order-{order_idx}"
            res = run_order_scenario_with_client(
                client=client,
                scenario_id=scenario_id,
                user_id=user_id,
                gets_per_order=gets_per_order,
                rate_limiter=rate_limiter,
            )
            if res["success"]:
                successes += 1
            else:
                failures += 1
                logging.warning(
                    "User %s, scenario %s failed: %s",
                    user_id, scenario_id, res["error"]
                )
    finally:
        client.close()

    return {
        "user_index": user_index,
        "successes": successes,
        "failures": failures,
    }


def main():
    parser = argparse.ArgumentParser(
        description="Load test for client service"
    )
    parser.add_argument(
        "--concurrency",
        type=int,
        default=200,
    )
    parser.add_argument(
        "--orders",
        type=int,
        default=100,
    )
    parser.add_argument(
        "--gets-per-order",
        type=int,
        default=100,
    )
    parser.add_argument(
        "--log-level",
        default="INFO",
    )
    parser.add_argument(
        "--rate",
        type=int,
        default=0,
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

    base_orders_per_user = total_orders // users if users > 0 else 0
    remainder = total_orders % users if users > 0 else 0

    logging.info(
        "Users=%d, total_orders=%d, base_orders_per_user=%d, remainder=%d",
        users, total_orders, base_orders_per_user, remainder,
    )

    successes = 0
    failures = 0
    
    rate_limiter = None
    if args.rate > 0:
        from threading import Lock
        import time as time_module
        rate_lock = Lock()
        min_interval = 1.0 / args.rate
        last_request_time = [0.0]
        
        def rate_limit():
            with rate_lock:
                now = time_module.time()
                elapsed = now - last_request_time[0]
                if elapsed < min_interval:
                    time_module.sleep(min_interval - elapsed)
                last_request_time[0] = time_module.time()
        rate_limiter = rate_limit
        logging.info("Rate limiting enabled: %d RPS", args.rate)

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
                    rate_limiter,
                )
            )

        for f in as_completed(futures):
            res = f.result()
            successes += res["successes"]
            failures += res["failures"]

    elapsed = time.time() - start_time
    total_requests = successes + failures

    logging.info(
        "Done. total_orders=%d, successes=%d, failures=%d, time=%.2fs",
        total_orders, successes, failures, elapsed,
    )
    if elapsed > 0:
        rps = total_orders / elapsed
        logging.info("Throughput: %.2f scenarios/sec", rps)
        logging.info("Success rate: %.2f%%", (successes / total_orders * 100) if total_orders > 0 else 0)
        
        if args.rate > 0:
            logging.info("Target rate: %d RPS, Actual: %.2f RPS", args.rate, rps)


if __name__ == "__main__":
    main()
