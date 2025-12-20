#!/usr/bin/env python3
from __future__ import annotations

import argparse
import logging
import random
import time
import uuid
from concurrent.futures import ThreadPoolExecutor, wait, FIRST_COMPLETED
from dataclasses import dataclass
from typing import Optional

import requests


CLIENT_SERVICE_URL = "http://localhost:8080"


class ClientServiceClient:
    def __init__(self, base_url: str):
        self.base_url = base_url.rstrip("/")
        adapter = requests.adapters.HTTPAdapter(
            pool_connections=200,
            pool_maxsize=400,
            max_retries=0,
            pool_block=False,
        )
        self.session = requests.Session()
        self.session.mount("http://", adapter)
        self.session.mount("https://", adapter)
        self.timeout = 15

    def close(self) -> None:
        self.session.close()

    def health_check(self) -> bool:
        try:
            r = self.session.get(f"{self.base_url}/health", timeout=5)
            return r.status_code == 200
        except requests.RequestException:
            return False

    def create_offer(self, user_id: str, scooter_id: str) -> requests.Response:
        return self.session.post(
            f"{self.base_url}/offers",
            json={"user_id": user_id, "scooter_id": scooter_id},
            timeout=self.timeout,
        )

    def create_order(self, order_id: str, offer_id: str, user_id: str) -> requests.Response:
        return self.session.post(
            f"{self.base_url}/orders",
            json={"order_id": order_id, "offer_id": offer_id, "user_id": user_id},
            timeout=self.timeout,
        )

    def get_order(self, order_id: str) -> requests.Response:
        return self.session.get(f"{self.base_url}/orders/{order_id}", timeout=self.timeout)

    def finish_order(self, order_id: str) -> requests.Response:
        return self.session.post(f"{self.base_url}/orders/{order_id}/finish", timeout=self.timeout)


@dataclass
class ScenarioResult:
    ok: bool
    order_id: str
    scooter_id: str
    user_id: str
    duration_s: int
    finished: bool
    final_status: Optional[str] = None
    final_amount: Optional[int] = None
    error: Optional[str] = None


def _sleep_jitter(base_seconds: float, jitter: float) -> None:
    if base_seconds <= 0:
        return
    delta = random.uniform(-jitter, jitter)
    time.sleep(max(0.0, base_seconds + delta))


def run_long_lived_order(
    idx: int,
    scooters: list[str],
    scooter_id_mode: str,
    min_duration_s: int,
    max_duration_s: int,
    gets_per_order: int,
    finish_ratio: float,
    get_jitter_s: float,
) -> ScenarioResult:
    user_id = f"load-user-{idx}-{uuid.uuid4()}"
    if scooter_id_mode == "load":
        scooter_id = f"load-scooter-{idx}-{uuid.uuid4()}"
    else:
        scooter_id = random.choice(scooters)
    duration_s = random.randint(min_duration_s, max_duration_s)
    do_finish = random.random() < finish_ratio

    client = ClientServiceClient(CLIENT_SERVICE_URL)
    try:
        offer_resp = client.create_offer(user_id=user_id, scooter_id=scooter_id)
        if offer_resp.status_code != 201:
            return ScenarioResult(
                ok=False,
                order_id="",
                scooter_id=scooter_id,
                user_id=user_id,
                duration_s=duration_s,
                finished=False,
                error=f"create_offer failed: {offer_resp.status_code} {offer_resp.text}",
            )
        offer_id = offer_resp.json()["id"]

        order_id = f"seed-order-{uuid.uuid4()}"
        order_resp = client.create_order(order_id=order_id, offer_id=offer_id, user_id=user_id)
        if order_resp.status_code != 201:
            return ScenarioResult(
                ok=False,
                order_id=order_id,
                scooter_id=scooter_id,
                user_id=user_id,
                duration_s=duration_s,
                finished=False,
                error=f"create_order failed: {order_resp.status_code} {order_resp.text}",
            )

        segment = max(1.0, duration_s / float(gets_per_order + 1))
        for _ in range(gets_per_order):
            _sleep_jitter(segment, get_jitter_s)
            get_resp = client.get_order(order_id)
            if get_resp.status_code != 200:
                return ScenarioResult(
                    ok=False,
                    order_id=order_id,
                    scooter_id=scooter_id,
                    user_id=user_id,
                    duration_s=duration_s,
                    finished=False,
                    error=f"get_order failed: {get_resp.status_code} {get_resp.text}",
                )

        remaining = max(0.0, duration_s - (segment * gets_per_order))
        _sleep_jitter(remaining, get_jitter_s)

        final_status = None
        final_amount = None

        if do_finish:
            finish_resp = client.finish_order(order_id)
            if finish_resp.status_code not in (200, 409):
                return ScenarioResult(
                    ok=False,
                    order_id=order_id,
                    scooter_id=scooter_id,
                    user_id=user_id,
                    duration_s=duration_s,
                    finished=False,
                    error=f"finish_order failed: {finish_resp.status_code} {finish_resp.text}",
                )
            try:
                data = finish_resp.json()
                final_status = data.get("status")
                final_amount = data.get("current_amount")
            except Exception:
                pass
            return ScenarioResult(
                ok=True,
                order_id=order_id,
                scooter_id=scooter_id,
                user_id=user_id,
                duration_s=duration_s,
                finished=True,
                final_status=final_status,
                final_amount=final_amount,
            )

        last_get = client.get_order(order_id)
        if last_get.status_code == 200:
            data = last_get.json()
            final_status = data.get("status")
            final_amount = data.get("current_amount")
        return ScenarioResult(
            ok=True,
            order_id=order_id,
            scooter_id=scooter_id,
            user_id=user_id,
            duration_s=duration_s,
            finished=False,
            final_status=final_status,
            final_amount=final_amount,
        )

    finally:
        client.close()


def main() -> None:
    parser = argparse.ArgumentParser(description="Seed OLTP data with long-lived concurrent orders")
    parser.add_argument("--orders", type=int, default=200, help="Total orders to create")
    parser.add_argument("--concurrency", type=int, default=50, help="Number of concurrent worker threads")
    parser.add_argument("--gets-per-order", type=int, default=5, help="Exactly how many GETs per order")
    parser.add_argument("--min-duration", type=int, default=90, help="Min ride duration (seconds)")
    parser.add_argument("--max-duration", type=int, default=420, help="Max ride duration (seconds)")
    parser.add_argument("--finish-ratio", type=float, default=0.9, help="Fraction of orders to finish (0..1)")
    parser.add_argument(
        "--scooters",
        default="scooter-1,scooter-2,scooter-3,scooter-4",
        help="Comma-separated scooter ids (use real ids for pricing diversity)",
    )
    parser.add_argument(
        "--scooter-id-mode",
        choices=["load", "real"],
        default="load",
        help="How to generate scooter_id. 'load' uses load-scooter-* to leverage external-service variety.",
    )
    parser.add_argument("--seed", type=int, default=0, help="Random seed (0 means time-based)")
    parser.add_argument("--log-level", default="INFO")
    parser.add_argument("--get-jitter", type=float, default=0.8, help="Random jitter added/subtracted to GET sleeps (seconds)")
    parser.add_argument(
        "--start-window",
        type=int,
        default=0,
        help="Spread order starts uniformly across this many seconds (0 = start immediately). "
             "Useful to avoid all created_at landing in the same minute.",
    )
    parser.add_argument(
        "--start-pattern",
        choices=["uniform", "even", "waves"],
        default="uniform",
        help="How to distribute order start times within start-window.",
    )
    parser.add_argument(
        "--wave-period",
        type=int,
        default=120,
        help="For start-pattern=waves: seconds between wave peaks.",
    )
    parser.add_argument(
        "--wave-width",
        type=float,
        default=10.0,
        help="For start-pattern=waves: standard deviation (seconds) around each peak.",
    )
    parser.add_argument(
        "--wave-strength",
        type=float,
        default=0.85,
        help="For start-pattern=waves: fraction of orders placed near peaks (rest are uniform).",
    )
    args = parser.parse_args()

    logging.basicConfig(
        level=getattr(logging, args.log_level.upper(), logging.INFO),
        format="%(asctime)s [%(levelname)s] %(message)s",
    )

    if args.seed == 0:
        random.seed()
    else:
        random.seed(args.seed)

    if args.min_duration <= 0 or args.max_duration < args.min_duration:
        raise SystemExit("invalid duration range")
    if args.gets_per_order <= 0:
        raise SystemExit("gets-per-order must be >= 1")
    if not (0.0 <= args.finish_ratio <= 1.0):
        raise SystemExit("finish-ratio must be within [0,1]")

    scooters = [s.strip() for s in args.scooters.split(",") if s.strip()]
    if args.scooter_id_mode == "real" and not scooters:
        raise SystemExit("no scooters provided")

    logging.info("CLIENT_SERVICE_URL=%s", CLIENT_SERVICE_URL)
    client = ClientServiceClient(CLIENT_SERVICE_URL)
    try:
        if not client.health_check():
            raise SystemExit("client-service is not healthy (GET /health failed)")
    finally:
        client.close()

    started = time.time()
    results: list[ScenarioResult] = []

    planned_offsets: list[float] = []
    if args.start_window <= 0:
        planned_offsets = [0.0 for _ in range(args.orders)]
    elif args.start_pattern == "even":
        if args.orders == 1:
            planned_offsets = [0.0]
        else:
            step = args.start_window / float(args.orders - 1)
            planned_offsets = [i * step for i in range(args.orders)]
    elif args.start_pattern == "waves":
        if args.wave_period <= 0:
            raise SystemExit("wave-period must be > 0")
        if args.wave_width <= 0:
            raise SystemExit("wave-width must be > 0")
        if not (0.0 <= args.wave_strength <= 1.0):
            raise SystemExit("wave-strength must be within [0,1]")

        peaks: list[float] = []
        t = 0.0
        while t <= args.start_window:
            peaks.append(t)
            t += float(args.wave_period)
        if not peaks:
            peaks = [0.0]

        for _ in range(args.orders):
            if random.random() < args.wave_strength:
                peak = random.choice(peaks)
                off = peak + random.gauss(0.0, args.wave_width)
            else:
                off = random.uniform(0.0, float(args.start_window))
            off = max(0.0, min(float(args.start_window), off))
            planned_offsets.append(off)
    else:
        planned_offsets = [random.uniform(0.0, float(args.start_window)) for _ in range(args.orders)]

    planned_offsets.sort()

    logging.info(
        "Seeding: orders=%d concurrency=%d gets_per_order=%d duration=[%d..%d] finish_ratio=%.2f scooter_id_mode=%s scooters=%s start_window=%ss pattern=%s",
        args.orders,
        args.concurrency,
        args.gets_per_order,
        args.min_duration,
        args.max_duration,
        args.finish_ratio,
        args.scooter_id_mode,
        scooters,
        args.start_window,
        args.start_pattern,
    )
    if args.start_pattern == "waves" and args.start_window > 0:
        logging.info(
            "Waves: period=%ss width=%.1fs strength=%.2f",
            args.wave_period,
            args.wave_width,
            args.wave_strength,
        )

    with ThreadPoolExecutor(max_workers=args.concurrency) as ex:
        futures = set()
        for i, off in enumerate(planned_offsets):
            target_ts = started + off
            now = time.time()
            if target_ts > now:
                time.sleep(target_ts - now)

            while len(futures) >= args.concurrency:
                done, futures = wait(futures, return_when=FIRST_COMPLETED)
                for f in done:
                    res = f.result()
                    results.append(res)
                    if not res.ok:
                        logging.warning(
                            "failed: scooter=%s duration=%ss err=%s",
                            res.scooter_id, res.duration_s, res.error,
                        )

            futures.add(
                ex.submit(
                    run_long_lived_order,
                    i,
                    scooters,
                    args.scooter_id_mode,
                    args.min_duration,
                    args.max_duration,
                    args.gets_per_order,
                    args.finish_ratio,
                    args.get_jitter,
                )
            )

        if futures:
            done, _ = wait(futures)
            for f in done:
                res = f.result()
                results.append(res)
                if not res.ok:
                    logging.warning(
                        "failed: scooter=%s duration=%ss err=%s",
                        res.scooter_id, res.duration_s, res.error,
                    )

    elapsed = time.time() - started
    ok = [r for r in results if r.ok]
    failed = [r for r in results if not r.ok]
    finished = [r for r in ok if r.finished]
    active = [r for r in ok if not r.finished]

    amounts = [r.final_amount for r in finished if isinstance(r.final_amount, int)]
    amounts_sorted = sorted(amounts)
    def _p(pct: float) -> Optional[int]:
        if not amounts_sorted:
            return None
        idx = int(round((pct / 100.0) * (len(amounts_sorted) - 1)))
        return amounts_sorted[idx]

    logging.info(
        "Done in %.1fs. ok=%d failed=%d finished=%d active_left=%d",
        elapsed,
        len(ok),
        len(failed),
        len(finished),
        len(active),
    )
    if amounts_sorted:
        logging.info(
            "Finished amounts stats: min=%s p50=%s p90=%s max=%s (n=%d)",
            amounts_sorted[0],
            _p(50),
            _p(90),
            amounts_sorted[-1],
            len(amounts_sorted),
        )
    else:
        logging.info("No finished amounts collected (maybe finish_ratio=0?)")

    logging.info(
        "Next: run Airflow DAG full_pipeline_manual to load into ClickHouse + rebuild marts"
    )


if __name__ == "__main__":
    main()


