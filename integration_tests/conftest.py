"""
Pytest fixtures for integration tests.
Handles Docker Compose setup/teardown and provides HTTP clients for testing.

Environment variables:
- SKIP_DOCKER_COMPOSE: Set to "1" to skip starting docker-compose (use already running services)
- CLIENT_SERVICE_URL: Override default client service URL (default: http://localhost:8080)
- EXTERNAL_SERVICE_URL: Override default external service URL (default: http://localhost:8081)
"""

import os
import subprocess
import time
from typing import Generator

import pytest
import requests
from tenacity import retry, stop_after_delay, wait_fixed


# Configuration
PROJECT_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
DOCKER_COMPOSE_FILE = os.path.join(PROJECT_ROOT, "docker-compose.yml")

CLIENT_SERVICE_URL = os.environ.get("CLIENT_SERVICE_URL", "http://localhost:8080")
EXTERNAL_SERVICE_URL = os.environ.get("EXTERNAL_SERVICE_URL", "http://localhost:8081")

# Skip docker-compose if services are already running
SKIP_DOCKER_COMPOSE = os.environ.get("SKIP_DOCKER_COMPOSE", "0") == "1"

# Timeouts
STARTUP_TIMEOUT = 120  # seconds
HEALTH_CHECK_INTERVAL = 2  # seconds


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


class ExternalServiceClient(ServiceClient):
    """External service specific methods (for verification)."""
    
    def get_scooter_data(self, scooter_id: str) -> requests.Response:
        """GET /scooter-data - Get scooter information."""
        return self.get("/scooter-data", params={"id": scooter_id})
    
    def get_zone_data(self, zone_id: str) -> requests.Response:
        """GET /tariff-zone-data - Get zone information."""
        return self.get("/tariff-zone-data", params={"id": zone_id})
    
    def get_user_profile(self, user_id: str) -> requests.Response:
        """GET /user-profile - Get user profile."""
        return self.get("/user-profile", params={"id": user_id})
    
    def get_configs(self) -> requests.Response:
        """GET /configs - Get dynamic configs."""
        return self.get("/configs")


def run_docker_compose(command: list, check: bool = True) -> subprocess.CompletedProcess:
    """Run docker-compose command."""
    full_command = ["docker-compose", "-f", DOCKER_COMPOSE_FILE] + command
    return subprocess.run(
        full_command,
        cwd=PROJECT_ROOT,
        capture_output=True,
        text=True,
        check=check
    )


@retry(stop=stop_after_delay(STARTUP_TIMEOUT), wait=wait_fixed(HEALTH_CHECK_INTERVAL))
def wait_for_service(client: ServiceClient, service_name: str):
    """Wait for a service to become healthy."""
    if not client.health_check():
        raise Exception(f"{service_name} is not healthy yet")
    print(f"{service_name} is healthy!")


@pytest.fixture(scope="session")
def docker_services() -> Generator[None, None, None]:
    """
    Session-scoped fixture that starts all Docker services before tests
    and stops them after all tests complete.
    
    Set SKIP_DOCKER_COMPOSE=1 to skip docker-compose commands (for already running services).
    """
    if SKIP_DOCKER_COMPOSE:
        print("\n⏭️ Skipping Docker Compose (SKIP_DOCKER_COMPOSE=1)")
        print("Using existing services...")
        
        # Just verify services are running
        client_client = ClientServiceClient(CLIENT_SERVICE_URL)
        external_client = ExternalServiceClient(EXTERNAL_SERVICE_URL)
        
        try:
            wait_for_service(external_client, "External Service")
            wait_for_service(client_client, "Client Service")
        finally:
            client_client.close()
            external_client.close()
        
        print("✅ All services are healthy!")
        yield
        return
    
    print("\n🚀 Starting Docker services...")
    
    # Stop any existing containers
    run_docker_compose(["down", "-v", "--remove-orphans"], check=False)
    
    # Build and start services
    try:
        result = run_docker_compose(["build"], check=False)
        if result.returncode != 0:
            print(f"Build output: {result.stdout}")
            print(f"Build errors: {result.stderr}")
            raise Exception(f"Failed to build Docker images: {result.stderr}")
        
        result = run_docker_compose(["up", "-d"], check=False)
        if result.returncode != 0:
            print(f"Start output: {result.stdout}")
            print(f"Start errors: {result.stderr}")
            raise Exception(f"Failed to start Docker services: {result.stderr}")
        
        print("Docker services started, waiting for health checks...")
        
        # Wait for services to be healthy
        client_client = ClientServiceClient(CLIENT_SERVICE_URL)
        external_client = ExternalServiceClient(EXTERNAL_SERVICE_URL)
        
        try:
            wait_for_service(external_client, "External Service")
            wait_for_service(client_client, "Client Service")
        finally:
            client_client.close()
            external_client.close()
        
        print("✅ All services are healthy!")
        
        yield
        
    finally:
        print("\n🛑 Stopping Docker services...")
        run_docker_compose(["down", "-v"], check=False)
        print("Docker services stopped.")


@pytest.fixture(scope="function")
def client_service(docker_services) -> Generator[ClientServiceClient, None, None]:
    """Function-scoped fixture providing a client service HTTP client."""
    client = ClientServiceClient(CLIENT_SERVICE_URL)
    yield client
    client.close()


@pytest.fixture(scope="function")
def external_service(docker_services) -> Generator[ExternalServiceClient, None, None]:
    """Function-scoped fixture providing an external service HTTP client."""
    client = ExternalServiceClient(EXTERNAL_SERVICE_URL)
    yield client
    client.close()


@pytest.fixture
def unique_user_id() -> str:
    """Generate a unique user ID for testing to avoid offer cache conflicts."""
    import uuid
    return f"test-user-{uuid.uuid4()}"


@pytest.fixture
def unique_scooter_id() -> str:
    """
    Return a scooter ID. Since we have limited scooters in external service,
    we'll use existing ones but tests should be designed to handle offer reuse.
    """
    return "scooter-1"


# Test data fixtures based on external/data/*.json
@pytest.fixture
def test_users() -> list[dict]:
    """Available test users from external service."""
    return [
        {"id": "user-1", "has_subscription": True, "trusted": True},
        {"id": "user-2", "has_subscription": False, "trusted": True},
        {"id": "user-3", "has_subscription": False, "trusted": False},
        {"id": "user-4", "has_subscription": True, "trusted": False},
    ]


@pytest.fixture
def test_scooters() -> list[dict]:
    """Available test scooters from external service."""
    return [
        {"id": "scooter-1", "zone_id": "zone-1", "charge": 85},
        {"id": "scooter-2", "zone_id": "zone-1", "charge": 45},
        {"id": "scooter-3", "zone_id": "zone-2", "charge": 90},
        {"id": "scooter-4", "zone_id": "zone-2", "charge": 25},  # Low charge
    ]


@pytest.fixture
def test_zones() -> list[dict]:
    """Available test zones from external service."""
    return [
        {"id": "zone-1", "price_per_minute": 7, "price_unlock": 50, "default_deposit": 200},
        {"id": "zone-2", "price_per_minute": 10, "price_unlock": 75, "default_deposit": 300},
    ]


@pytest.fixture
def test_configs() -> dict:
    """Dynamic configs from external service."""
    return {
        "surge": 1.2,
        "low_charge_discount": 0.7,
        "low_charge_threshold_percent": 28,
        "incomplete_ride_threshold_seconds": 5,
    }
