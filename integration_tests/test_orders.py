"""
Integration tests for Order creation, retrieval, and completion.

Tests cover:
- Creating orders from valid offers
- Order idempotency (client-provided order_id)
- Order status lifecycle
- Order finishing and payment handling
- Error handling for invalid operations
"""

import time
import uuid

import pytest


def unique_user():
    """Generate a unique user ID to avoid offer cache conflicts between tests."""
    return f"test-user-{uuid.uuid4()}"


class TestOrderCreation:
    """Tests for POST /orders endpoint."""
    
    def test_create_order_success(self, client_service):
        """Test successful order creation from a valid offer."""
        user_id = unique_user()
        
        # First create an offer
        offer_response = client_service.create_offer(user_id, "scooter-1")
        assert offer_response.status_code == 201
        offer = offer_response.json()
        
        # Create order from offer
        order_id = str(uuid.uuid4())
        order_response = client_service.create_order(order_id, offer["id"], user_id)
        
        assert order_response.status_code == 201, \
            f"Expected 201, got {order_response.status_code}: {order_response.text}"
        
        order = order_response.json()
        
        # Verify required fields from OpenAPI spec
        assert order["id"] == order_id
        assert order["user_id"] == user_id
        assert order["scooter_id"] == offer["scooter_id"]
        assert order["offer_id"] == offer["id"]
        assert order["status"] == "ACTIVE"
        assert "start_time" in order
    
    def test_create_order_inherits_offer_pricing(self, client_service):
        """Test that order inherits pricing from offer."""
        user_id = unique_user()
        
        offer_response = client_service.create_offer(user_id, "scooter-1")
        assert offer_response.status_code == 201
        offer = offer_response.json()
        
        order_id = str(uuid.uuid4())
        order_response = client_service.create_order(order_id, offer["id"], user_id)
        
        assert order_response.status_code == 201
        order = order_response.json()
        
        # Order should have same pricing as offer
        assert order["price_per_minute"] == offer["price_per_minute"]
        assert order["price_unlock"] == offer["price_unlock"]
        assert order["deposit"] == offer["deposit"]
    
    def test_create_order_idempotency_same_order_id(self, client_service):
        """
        Test idempotency: same order_id returns existing order.
        ADR: проверка по order_id - если заказ уже существует с тем же order_id,
        возвращается существующий заказ
        """
        user_id = unique_user()
        
        offer_response = client_service.create_offer(user_id, "scooter-1")
        assert offer_response.status_code == 201
        offer = offer_response.json()
        
        order_id = str(uuid.uuid4())
        
        # First request
        response1 = client_service.create_order(order_id, offer["id"], user_id)
        assert response1.status_code == 201
        order1 = response1.json()
        
        # Second request with same order_id (idempotent)
        response2 = client_service.create_order(order_id, offer["id"], user_id)
        assert response2.status_code == 201
        order2 = response2.json()
        
        # Should return same order
        assert order1["id"] == order2["id"]
        # Note: start_time may have different precision in responses,
        # so we compare the date portion only (first 19 chars: "2025-01-01T12:00:00")
        assert order1["start_time"][:19] == order2["start_time"][:19]
    
    def test_create_order_offer_not_found(self, client_service):
        """Test error when offer doesn't exist."""
        order_id = str(uuid.uuid4())
        response = client_service.create_order(order_id, "nonexistent-offer", unique_user())
        
        assert response.status_code == 400
        assert "not found" in response.text.lower()
    
    def test_create_order_offer_already_used(self, client_service):
        """Test error when offer was already used for another order."""
        user_id = unique_user()
        
        # Create offer
        offer_response = client_service.create_offer(user_id, "scooter-1")
        assert offer_response.status_code == 201
        offer = offer_response.json()
        
        # First order
        order_id1 = str(uuid.uuid4())
        response1 = client_service.create_order(order_id1, offer["id"], user_id)
        assert response1.status_code == 201
        
        # Second order with same offer but different order_id
        order_id2 = str(uuid.uuid4())
        response2 = client_service.create_order(order_id2, offer["id"], user_id)
        
        assert response2.status_code == 400
        assert "already used" in response2.text.lower()
    
    def test_create_order_user_mismatch(self, client_service):
        """Test error when user_id doesn't match offer's user."""
        user1 = unique_user()
        user2 = unique_user()
        
        # Create offer for user1
        offer_response = client_service.create_offer(user1, "scooter-1")
        assert offer_response.status_code == 201
        offer = offer_response.json()
        
        # Try to create order with different user
        order_id = str(uuid.uuid4())
        response = client_service.create_order(order_id, offer["id"], user2)
        
        assert response.status_code == 400
        assert "user" in response.text.lower() or "invalid" in response.text.lower()
    
    def test_create_order_missing_order_id(self, client_service):
        """Test error when order_id is missing."""
        user_id = unique_user()
        offer_response = client_service.create_offer(user_id, "scooter-1")
        offer = offer_response.json()
        
        response = client_service.post("/orders", json={
            "offer_id": offer["id"],
            "user_id": user_id
        })
        
        assert response.status_code == 400
        assert "order_id" in response.text.lower()
    
    def test_create_order_missing_offer_id(self, client_service):
        """Test error when offer_id is missing."""
        response = client_service.post("/orders", json={
            "order_id": str(uuid.uuid4()),
            "user_id": unique_user()
        })
        
        assert response.status_code == 400
        assert "offer_id" in response.text.lower()
    
    def test_create_order_missing_user_id(self, client_service):
        """Test error when user_id is missing."""
        user_id = unique_user()
        offer_response = client_service.create_offer(user_id, "scooter-1")
        offer = offer_response.json()
        
        response = client_service.post("/orders", json={
            "order_id": str(uuid.uuid4()),
            "offer_id": offer["id"]
        })
        
        assert response.status_code == 400
        assert "user_id" in response.text.lower()
    
    def test_create_order_initial_amount_includes_unlock(self, client_service, test_zones):
        """Test that initial current_amount equals unlock price."""
        # Use unique user for fresh offer
        user_id = unique_user()
        
        offer_response = client_service.create_offer(user_id, "scooter-3")
        assert offer_response.status_code == 201
        offer = offer_response.json()
        
        order_id = str(uuid.uuid4())
        order_response = client_service.create_order(order_id, offer["id"], user_id)
        
        assert order_response.status_code == 201
        order = order_response.json()
        
        # Initial amount should include unlock price
        # Unknown users get charged unlock price (not 0)
        assert order["current_amount"] == offer["price_unlock"]


class TestOrderRetrieval:
    """Tests for GET /orders/{order_id} endpoint."""
    
    def test_get_order_success(self, client_service):
        """Test successful order retrieval."""
        user_id = unique_user()
        
        # Create offer and order
        offer_response = client_service.create_offer(user_id, "scooter-1")
        offer = offer_response.json()
        
        order_id = str(uuid.uuid4())
        client_service.create_order(order_id, offer["id"], user_id)
        
        # Get order
        response = client_service.get_order(order_id)
        
        assert response.status_code == 200
        order = response.json()
        
        assert order["id"] == order_id
        assert order["status"] == "ACTIVE"
    
    def test_get_order_not_found(self, client_service):
        """Test 404 when order doesn't exist."""
        response = client_service.get_order("nonexistent-order")
        
        assert response.status_code == 404
    
    def test_get_order_contains_pricing_info(self, client_service):
        """Test that retrieved order contains pricing information."""
        user_id = unique_user()
        
        offer_response = client_service.create_offer(user_id, "scooter-1")
        offer = offer_response.json()
        
        order_id = str(uuid.uuid4())
        client_service.create_order(order_id, offer["id"], user_id)
        
        response = client_service.get_order(order_id)
        order = response.json()
        
        # Verify pricing fields are present
        assert "price_per_minute" in order
        assert "price_unlock" in order
        assert "deposit" in order
        assert "start_time" in order
    
    def test_get_order_caching(self, client_service):
        """Test that order retrieval is consistent (cached or not)."""
        user_id = unique_user()
        
        offer_response = client_service.create_offer(user_id, "scooter-2")
        offer = offer_response.json()
        
        order_id = str(uuid.uuid4())
        client_service.create_order(order_id, offer["id"], user_id)
        
        # Multiple GETs should return consistent data
        response1 = client_service.get_order(order_id)
        response2 = client_service.get_order(order_id)
        
        assert response1.json() == response2.json()


class TestOrderFinishing:
    """Tests for POST /orders/{order_id}/finish endpoint."""
    
    def test_finish_order_success(self, client_service):
        """Test successful order finishing."""
        user_id = unique_user()
        
        # Create offer and order
        offer_response = client_service.create_offer(user_id, "scooter-1")
        offer = offer_response.json()
        
        order_id = str(uuid.uuid4())
        client_service.create_order(order_id, offer["id"], user_id)
        
        # Wait a bit to accumulate some ride time
        time.sleep(1)
        
        # Finish order
        response = client_service.finish_order(order_id)
        
        assert response.status_code == 200
        order = response.json()
        
        assert order["id"] == order_id
        assert order["status"] == "FINISHED"
        assert "finish_time" in order
        assert "duration_seconds" in order
        assert order["duration_seconds"] >= 1
    
    def test_finish_order_calculates_total_amount(self, client_service):
        """
        Test that total amount is calculated correctly.
        Formula: unlock + ceil(seconds/60) * price_per_minute
        """
        user_id = unique_user()
        
        offer_response = client_service.create_offer(user_id, "scooter-1")
        offer = offer_response.json()
        
        order_id = str(uuid.uuid4())
        client_service.create_order(order_id, offer["id"], user_id)
        
        # Wait for some time to ensure ride duration exceeds incomplete_ride_threshold (5 seconds)
        time.sleep(6)  # 6 seconds > 5 seconds threshold, so payment will be charged
        
        response = client_service.finish_order(order_id)
        order = response.json()
        
        # Calculate expected total
        duration_seconds = order["duration_seconds"]
        minutes = -(-duration_seconds // 60)  # ceiling division
        expected_total = offer["price_unlock"] + (minutes * offer["price_per_minute"])
        
        assert order["current_amount"] == expected_total, \
            f"Expected {expected_total}, got {order['current_amount']}"
    
    def test_finish_order_idempotency(self, client_service):
        """
        Test that finishing already finished order returns 409.
        ADR: idempotent finish
        """
        user_id = unique_user()
        
        offer_response = client_service.create_offer(user_id, "scooter-1")
        offer = offer_response.json()
        
        order_id = str(uuid.uuid4())
        client_service.create_order(order_id, offer["id"], user_id)
        
        # First finish
        response1 = client_service.finish_order(order_id)
        assert response1.status_code == 200
        
        # Second finish (idempotent)
        response2 = client_service.finish_order(order_id)
        assert response2.status_code == 409
        assert "already finished" in response2.text.lower()
    
    def test_finish_order_not_found(self, client_service):
        """Test error when order doesn't exist."""
        response = client_service.finish_order("nonexistent-order")
        
        assert response.status_code == 400
    
    def test_finish_order_updates_cache(self, client_service):
        """Test that finishing order updates cached data."""
        user_id = unique_user()
        
        offer_response = client_service.create_offer(user_id, "scooter-1")
        offer = offer_response.json()
        
        order_id = str(uuid.uuid4())
        client_service.create_order(order_id, offer["id"], user_id)
        
        # Verify initial status
        get_response1 = client_service.get_order(order_id)
        assert get_response1.json()["status"] == "ACTIVE"
        
        # Finish order
        client_service.finish_order(order_id)
        
        # Verify status is updated in cache/DB
        get_response2 = client_service.get_order(order_id)
        assert get_response2.json()["status"] == "FINISHED"


class TestOrderLifecycle:
    """End-to-end tests for complete order lifecycle."""
    
    def test_full_order_lifecycle(self, client_service):
        """
        Test complete order lifecycle:
        1. Create offer
        2. Create order from offer
        3. Get order status during ride
        4. Finish order
        5. Verify final state
        """
        user_id = unique_user()
        
        # Step 1: Create offer
        offer_response = client_service.create_offer(user_id, "scooter-1")
        assert offer_response.status_code == 201
        offer = offer_response.json()
        
        # Step 2: Create order
        order_id = str(uuid.uuid4())
        order_response = client_service.create_order(order_id, offer["id"], user_id)
        assert order_response.status_code == 201
        order = order_response.json()
        assert order["status"] == "ACTIVE"
        
        # Step 3: Get order during ride
        time.sleep(1)
        get_response = client_service.get_order(order_id)
        assert get_response.status_code == 200
        active_order = get_response.json()
        assert active_order["status"] == "ACTIVE"
        
        # Wait to ensure ride duration exceeds incomplete_ride_threshold (5 seconds)
        time.sleep(6)  # Total duration will be >= 7 seconds
        
        # Step 4: Finish order
        finish_response = client_service.finish_order(order_id)
        assert finish_response.status_code == 200
        finished_order = finish_response.json()
        assert finished_order["status"] == "FINISHED"
        
        # Step 5: Verify final state
        final_response = client_service.get_order(order_id)
        assert final_response.status_code == 200
        final_order = final_response.json()
        
        assert final_order["status"] == "FINISHED"
        assert final_order["finish_time"] is not None
        assert final_order["duration_seconds"] >= 1
        assert final_order["current_amount"] > 0
    
    def test_multiple_concurrent_orders_different_users(self, client_service):
        """Test that multiple users can have concurrent active orders."""
        orders = []
        
        # Create orders for different unique users
        scooters = ["scooter-1", "scooter-2", "scooter-3"]
        for scooter_id in scooters:
            user_id = unique_user()
            
            offer_response = client_service.create_offer(user_id, scooter_id)
            assert offer_response.status_code == 201
            offer = offer_response.json()
            
            order_id = str(uuid.uuid4())
            order_response = client_service.create_order(order_id, offer["id"], user_id)
            assert order_response.status_code == 201
            orders.append(order_response.json())
        
        # Verify all orders are active
        for order in orders:
            get_response = client_service.get_order(order["id"])
            assert get_response.json()["status"] == "ACTIVE"
        
        # Finish all orders
        for order in orders:
            finish_response = client_service.finish_order(order["id"])
            assert finish_response.status_code == 200
            assert finish_response.json()["status"] == "FINISHED"
    
    def test_order_cannot_be_created_from_used_offer(self, client_service):
        """Test that same offer cannot be used for multiple orders."""
        user_id = unique_user()
        
        offer_response = client_service.create_offer(user_id, "scooter-1")
        offer = offer_response.json()
        
        # First order succeeds
        order1_id = str(uuid.uuid4())
        response1 = client_service.create_order(order1_id, offer["id"], user_id)
        assert response1.status_code == 201
        
        # Finish first order
        client_service.finish_order(order1_id)
        
        # Try to create second order with same offer - should fail
        order2_id = str(uuid.uuid4())
        response2 = client_service.create_order(order2_id, offer["id"], user_id)
        assert response2.status_code == 400
    
    def test_user_can_create_new_offer_after_finishing_order(self, client_service):
        """Test that user can create new offer and order after finishing previous."""
        user_id = unique_user()
        scooter_id = "scooter-1"
        
        # First ride
        offer1_response = client_service.create_offer(user_id, scooter_id)
        offer1 = offer1_response.json()
        
        order1_id = str(uuid.uuid4())
        client_service.create_order(order1_id, offer1["id"], user_id)
        client_service.finish_order(order1_id)
        
        # Second ride - new offer should be created (old one was used)
        # Wait a bit to ensure the system processes the finish
        time.sleep(1)
        
        # Create offer with a different scooter to avoid cache
        offer2_response = client_service.create_offer(user_id, "scooter-2")
        assert offer2_response.status_code == 201
        offer2 = offer2_response.json()
        
        # Can create new order
        order2_id = str(uuid.uuid4())
        order2_response = client_service.create_order(order2_id, offer2["id"], user_id)
        assert order2_response.status_code == 201
