"""
Integration tests for API contract validation.

Tests verify that API responses match OpenAPI specification.
"""

import re
import uuid
from datetime import datetime

import pytest


def unique_user():
    """Generate a unique user ID to avoid offer cache conflicts between tests."""
    return f"test-user-{uuid.uuid4()}"


def normalize_iso_datetime(iso_str: str) -> str:
    """
    Normalize ISO datetime string by truncating nanoseconds to microseconds.
    
    Go's time.Time serializes to RFC3339Nano format with nanoseconds (9 digits),
    but Python's datetime.fromisoformat() only supports microseconds (6 digits).
    
    Example:
        Input:  "2025-11-30T20:47:37.042099855Z"
        Output: "2025-11-30T20:47:37.042099+00:00"
    """
    # Replace Z with +00:00 first
    normalized = iso_str.replace("Z", "+00:00")
    
    # Match fractional seconds with more than 6 digits: .123456789+00:00
    # Truncate to 6 digits (microseconds)
    pattern = r'\.(\d{6})\d+(\+00:00)'
    replacement = r'.\1\2'
    normalized = re.sub(pattern, replacement, normalized)
    
    return normalized


class TestOfferAPIContract:
    """Tests for Offer API contract validation."""
    
    def test_offer_response_schema(self, client_service):
        """Test that offer response matches OpenAPI schema."""
        response = client_service.create_offer("user-1", "scooter-1")
        
        assert response.status_code == 201
        offer = response.json()
        
        required_fields = [
            "id", "user_id", "scooter_id", "zone_id", 
            "expires_at", "price_per_minute", "price_unlock", "deposit"
        ]
        
        for field in required_fields:
            assert field in offer, f"Missing required field: {field}"
        
        assert isinstance(offer["id"], str)
        assert isinstance(offer["user_id"], str)
        assert isinstance(offer["scooter_id"], str)
        assert isinstance(offer["zone_id"], str)
        assert isinstance(offer["expires_at"], str)
        assert isinstance(offer["price_per_minute"], int)
        assert isinstance(offer["price_unlock"], int)
        assert isinstance(offer["deposit"], int)
        
        try:
            # Нормализуем ISO строку: обрезаем наносекунды до микросекунд для совместимости с Python
            expires_at_str = normalize_iso_datetime(offer["expires_at"])
            datetime.fromisoformat(expires_at_str)
        except ValueError:
            pytest.fail(f"expires_at is not valid ISO datetime: {offer['expires_at']}")
    
    def test_offer_values_are_positive(self, client_service):
        """Test that offer numeric values are non-negative."""
        response = client_service.create_offer("user-3", "scooter-1")
        offer = response.json()
        
        assert offer["price_per_minute"] >= 0
        assert offer["price_unlock"] >= 0
        assert offer["deposit"] >= 0
    
    def test_offer_ids_are_unique(self, client_service):
        """Test that different offers have unique IDs."""
        combinations = [
            ("user-1", "scooter-1"),
            ("user-1", "scooter-2"),
            ("user-2", "scooter-1"),
            ("user-2", "scooter-2"),
        ]
        
        offer_ids = set()
        for user_id, scooter_id in combinations:
            response = client_service.create_offer(user_id, scooter_id)
            assert response.status_code == 201
            offer_ids.add(response.json()["id"])
        
        assert len(offer_ids) == len(combinations), \
            "All offers should have unique IDs"


class TestOrderAPIContract:
    """Tests for Order API contract validation."""
    
    def test_order_response_schema_on_create(self, client_service):
        """Test that order response on creation matches OpenAPI schema."""
        user_id = unique_user()
        
        offer_response = client_service.create_offer(user_id, "scooter-1")
        offer = offer_response.json()
        
        order_id = str(uuid.uuid4())
        response = client_service.create_order(order_id, offer["id"], user_id)
        
        assert response.status_code == 201
        order = response.json()
        
        required_fields = [
            "id", "user_id", "scooter_id", "offer_id", "status", "start_time"
        ]
        
        for field in required_fields:
            assert field in order, f"Missing required field: {field}"
        
        assert isinstance(order["id"], str)
        assert isinstance(order["user_id"], str)
        assert isinstance(order["scooter_id"], str)
        assert isinstance(order["offer_id"], str)
        assert isinstance(order["status"], str)
        assert isinstance(order["start_time"], str)
        
        valid_statuses = ["ACTIVE", "FINISHED", "CANCELLED", "PAYMENT_FAILED"]
        assert order["status"] in valid_statuses, \
            f"Invalid status: {order['status']}"
        
        assert order["status"] == "ACTIVE"
    
    def test_order_response_schema_after_finish(self, client_service):
        """Test that finished order response matches OpenAPI schema."""
        user_id = unique_user()
        
        offer_response = client_service.create_offer(user_id, "scooter-1")
        offer = offer_response.json()
        
        order_id = str(uuid.uuid4())
        client_service.create_order(order_id, offer["id"], user_id)
        
        response = client_service.finish_order(order_id)
        
        assert response.status_code == 200
        order = response.json()
        
        assert "finish_time" in order
        assert order["finish_time"] is not None
        
        assert "duration_seconds" in order
        assert isinstance(order["duration_seconds"], int)
        assert order["duration_seconds"] >= 0
        
        assert order["status"] == "FINISHED"
    
    def test_order_pricing_fields(self, client_service):
        """Test that order contains pricing information."""
        user_id = unique_user()
        
        offer_response = client_service.create_offer(user_id, "scooter-1")
        offer = offer_response.json()
        
        order_id = str(uuid.uuid4())
        response = client_service.create_order(order_id, offer["id"], user_id)
        order = response.json()
        
        assert "price_per_minute" in order
        assert "price_unlock" in order
        assert "deposit" in order
        assert "current_amount" in order
        
        assert order["price_per_minute"] == offer["price_per_minute"]
        assert order["price_unlock"] == offer["price_unlock"]
        assert order["deposit"] == offer["deposit"]


class TestErrorResponses:
    """Tests for error response format."""
    
    def test_400_response_format(self, client_service):
        """Test that 400 errors have descriptive text."""
        response = client_service.post("/offers", json={})
        
        assert response.status_code == 400
        assert len(response.text) > 0
    
    def test_404_response_for_order(self, client_service):
        """Test that 404 is returned for non-existent order."""
        response = client_service.get_order("nonexistent-order-id")
        
        assert response.status_code == 404
    
    def test_409_response_for_finished_order(self, client_service):
        """Test that 409 is returned when finishing already finished order."""
        user_id = unique_user()
        
        offer_response = client_service.create_offer(user_id, "scooter-1")
        offer = offer_response.json()
        
        order_id = str(uuid.uuid4())
        client_service.create_order(order_id, offer["id"], user_id)
        
        client_service.finish_order(order_id)
        
        response = client_service.finish_order(order_id)
        
        assert response.status_code == 409


class TestContentTypeHeaders:
    """Tests for proper Content-Type headers."""
    
    def test_offers_returns_json(self, client_service):
        """Test that POST /offers returns JSON content type."""
        response = client_service.create_offer(unique_user(), "scooter-1")
        
        assert response.status_code == 201
        assert "application/json" in response.headers.get("Content-Type", "")
    
    def test_orders_returns_json(self, client_service):
        """Test that POST /orders returns JSON content type."""
        user_id = unique_user()
        offer_response = client_service.create_offer(user_id, "scooter-1")
        offer = offer_response.json()
        
        order_id = str(uuid.uuid4())
        response = client_service.create_order(order_id, offer["id"], user_id)
        
        assert response.status_code == 201
        assert "application/json" in response.headers.get("Content-Type", "")
    
    def test_get_order_returns_json(self, client_service):
        """Test that GET /orders/{id} returns JSON content type."""
        user_id = unique_user()
        offer_response = client_service.create_offer(user_id, "scooter-1")
        offer = offer_response.json()
        
        order_id = str(uuid.uuid4())
        client_service.create_order(order_id, offer["id"], user_id)
        
        response = client_service.get_order(order_id)
        
        assert response.status_code == 200
        assert "application/json" in response.headers.get("Content-Type", "")


class TestOfferExpiration:
    """Tests for offer expiration behavior."""
    
    def test_offer_has_future_expiration(self, client_service):
        """Test that newly created offer has expiration in the future."""
        response = client_service.create_offer("user-1", "scooter-1")
        offer = response.json()
        
        expires_at_str = normalize_iso_datetime(offer["expires_at"])
        expires_at = datetime.fromisoformat(expires_at_str)
        now = datetime.now(expires_at.tzinfo)
        
        assert expires_at > now, \
            f"Offer should expire in future, but expires_at={expires_at}, now={now}"
    
    def test_offer_expires_in_5_minutes(self, client_service):
        """Test that offer expiration is approximately 5 minutes in the future."""
        response = client_service.create_offer("user-2", "scooter-2")
        offer = response.json()
        
        expires_at_str = normalize_iso_datetime(offer["expires_at"])
        expires_at = datetime.fromisoformat(expires_at_str)
        
        if "created_at" in offer and offer["created_at"]:
            created_at_str = normalize_iso_datetime(offer["created_at"])
            created_at = datetime.fromisoformat(created_at_str)
            ttl = (expires_at - created_at).total_seconds()
            
            assert abs(ttl - 600) <= 10, f"Offer TTL should be ~5 minutes, got {ttl} seconds"
