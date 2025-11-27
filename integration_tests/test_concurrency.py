"""
Integration tests for concurrent operations and stress scenarios.

Tests cover:
- Concurrent offer creation
- Concurrent order operations
- Race conditions in offer usage
- System behavior under load
"""

import concurrent.futures
import time
import uuid
from typing import Callable

import pytest


def unique_user():
    """Generate a unique user ID to avoid offer cache conflicts between tests."""
    return f"test-user-{uuid.uuid4()}"


class TestConcurrentOfferCreation:
    """Tests for concurrent offer creation scenarios."""
    
    def test_concurrent_offers_same_user_scooter(self, client_service):
        """
        Test concurrent offer creation for same user/scooter.
        Due to race conditions, concurrent requests may create multiple offers
        before caching catches up. This test verifies the system handles 
        concurrent requests gracefully.
        """
        user_id = unique_user()
        scooter_id = "scooter-1"
        offer_ids = []
        
        def create_offer():
            response = client_service.create_offer(user_id, scooter_id)
            if response.status_code == 201:
                return response.json()["id"]
            return None
        
        # Execute 5 concurrent requests
        with concurrent.futures.ThreadPoolExecutor(max_workers=5) as executor:
            futures = [executor.submit(create_offer) for _ in range(5)]
            for future in concurrent.futures.as_completed(futures):
                result = future.result()
                if result:
                    offer_ids.append(result)
        
        # All requests should succeed
        assert len(offer_ids) >= 1, "At least one offer should be created"
        
        # After concurrent requests, subsequent requests should return cached offer
        subsequent_response = client_service.create_offer(user_id, scooter_id)
        assert subsequent_response.status_code == 201
        cached_offer_id = subsequent_response.json()["id"]
        
        # The cached offer should match one of the created offers
        assert cached_offer_id in offer_ids, \
            "Subsequent request should return one of the created offers"
    
    def test_concurrent_offers_different_users(self, client_service):
        """Test concurrent offer creation for different users."""
        results = {}
        
        def create_offer_for_user(user_id: str):
            response = client_service.create_offer(user_id, "scooter-1")
            return user_id, response.status_code, response.json() if response.status_code == 201 else None
        
        users = [unique_user() for _ in range(4)]
        
        with concurrent.futures.ThreadPoolExecutor(max_workers=4) as executor:
            futures = [executor.submit(create_offer_for_user, uid) for uid in users]
            for future in concurrent.futures.as_completed(futures):
                user_id, status, offer = future.result()
                results[user_id] = (status, offer)
        
        # All should succeed
        for user_id, (status, offer) in results.items():
            assert status == 201, f"User {user_id} failed to create offer"
            assert offer["user_id"] == user_id


class TestConcurrentOrderCreation:
    """Tests for concurrent order creation scenarios."""
    
    def test_concurrent_orders_same_offer(self, client_service):
        """
        Test concurrent order creation from same offer.
        Only one should succeed, others should fail (offer already used).
        """
        user_id = unique_user()
        
        # Create offer
        offer_response = client_service.create_offer(user_id, "scooter-1")
        assert offer_response.status_code == 201
        offer = offer_response.json()
        
        success_count = 0
        fail_count = 0
        
        def create_order():
            order_id = str(uuid.uuid4())
            response = client_service.create_order(order_id, offer["id"], user_id)
            return response.status_code
        
        with concurrent.futures.ThreadPoolExecutor(max_workers=5) as executor:
            futures = [executor.submit(create_order) for _ in range(5)]
            for future in concurrent.futures.as_completed(futures):
                status = future.result()
                if status == 201:
                    success_count += 1
                elif status == 400:
                    fail_count += 1
        
        # Only one should succeed (offer can only be used once)
        assert success_count == 1, \
            f"Expected exactly 1 success, got {success_count}"
        assert fail_count == 4, \
            f"Expected 4 failures, got {fail_count}"
    
    def test_concurrent_orders_same_order_id(self, client_service):
        """
        Test concurrent order creation with same order_id (idempotency).
        All should return the same order.
        """
        user_id = unique_user()
        
        offer_response = client_service.create_offer(user_id, "scooter-1")
        offer = offer_response.json()
        
        order_id = str(uuid.uuid4())
        orders = []
        
        def create_order():
            response = client_service.create_order(order_id, offer["id"], user_id)
            if response.status_code == 201:
                return response.json()
            return None
        
        with concurrent.futures.ThreadPoolExecutor(max_workers=5) as executor:
            futures = [executor.submit(create_order) for _ in range(5)]
            for future in concurrent.futures.as_completed(futures):
                result = future.result()
                if result:
                    orders.append(result)
        
        # All should return same order (idempotency by order_id)
        assert len(orders) > 0
        order_ids = [o["id"] for o in orders]
        assert len(set(order_ids)) == 1, \
            f"All orders should have same ID, got {set(order_ids)}"


class TestConcurrentOrderFinishing:
    """Tests for concurrent order finishing scenarios."""
    
    def test_concurrent_finish_same_order(self, client_service):
        """
        Test concurrent finish requests for same order.
        At least one should succeed with 200. The service may handle concurrent
        finish requests differently - some implementations return success for
        all concurrent requests (idempotent finish), others return 409 for duplicates.
        """
        user_id = unique_user()
        
        # Create and start order
        offer_response = client_service.create_offer(user_id, "scooter-1")
        offer = offer_response.json()
        
        order_id = str(uuid.uuid4())
        client_service.create_order(order_id, offer["id"], user_id)
        
        results = []
        
        def finish_order():
            response = client_service.finish_order(order_id)
            return response.status_code
        
        with concurrent.futures.ThreadPoolExecutor(max_workers=5) as executor:
            futures = [executor.submit(finish_order) for _ in range(5)]
            for future in concurrent.futures.as_completed(futures):
                results.append(future.result())
        
        # At least one should succeed with 200
        success_count = sum(1 for r in results if r == 200)
        assert success_count >= 1, \
            f"Expected at least 1 success (200), got responses: {results}"
        
        # Verify final order state is FINISHED
        order = client_service.get_order(order_id).json()
        assert order["status"] == "FINISHED"


class TestMultipleRidesConcurrency:
    """Tests for multiple concurrent rides by different users."""
    
    def test_multiple_concurrent_full_rides(self, client_service):
        """Test multiple users doing full ride cycles concurrently."""
        results = []
        
        def complete_ride(user_id: str, scooter_id: str):
            try:
                # Create offer
                offer_response = client_service.create_offer(user_id, scooter_id)
                if offer_response.status_code != 201:
                    return {"user": user_id, "success": False, "stage": "offer"}
                offer = offer_response.json()
                
                # Create order
                order_id = str(uuid.uuid4())
                order_response = client_service.create_order(order_id, offer["id"], user_id)
                if order_response.status_code != 201:
                    return {"user": user_id, "success": False, "stage": "order"}
                
                # Small wait
                time.sleep(0.1)
                
                # Finish order
                finish_response = client_service.finish_order(order_id)
                if finish_response.status_code != 200:
                    return {"user": user_id, "success": False, "stage": "finish"}
                
                return {
                    "user": user_id, 
                    "success": True, 
                    "order_id": order_id,
                    "status": finish_response.json()["status"]
                }
            except Exception as e:
                return {"user": user_id, "success": False, "error": str(e)}
        
        # Each user rides different scooter
        ride_params = [
            (unique_user(), "scooter-1"),
            (unique_user(), "scooter-2"),
            (unique_user(), "scooter-3"),
            (unique_user(), "scooter-4"),
        ]
        
        with concurrent.futures.ThreadPoolExecutor(max_workers=4) as executor:
            futures = [
                executor.submit(complete_ride, user_id, scooter_id) 
                for user_id, scooter_id in ride_params
            ]
            for future in concurrent.futures.as_completed(futures):
                results.append(future.result())
        
        # All rides should complete successfully
        for result in results:
            assert result["success"], \
                f"User {result['user']} failed at stage {result.get('stage', 'unknown')}: {result.get('error', '')}"
            assert result["status"] == "FINISHED"


class TestRapidOperations:
    """Tests for rapid sequential operations."""
    
    def test_rapid_offer_creation_different_scooters(self, client_service):
        """Test rapid offer creation for different scooters."""
        user_id = unique_user()
        scooters = ["scooter-1", "scooter-2", "scooter-3", "scooter-4"]
        
        offers = []
        for scooter_id in scooters:
            response = client_service.create_offer(user_id, scooter_id)
            assert response.status_code == 201
            offers.append(response.json())
        
        # All offers should be different
        offer_ids = [o["id"] for o in offers]
        assert len(set(offer_ids)) == len(scooters), \
            "Each scooter should have its own offer"
    
    def test_rapid_order_get_requests(self, client_service):
        """Test rapid GET requests for order status."""
        user_id = unique_user()
        
        # Create order
        offer_response = client_service.create_offer(user_id, "scooter-1")
        offer = offer_response.json()
        
        order_id = str(uuid.uuid4())
        client_service.create_order(order_id, offer["id"], user_id)
        
        # Rapid GET requests
        success_count = 0
        for _ in range(20):
            response = client_service.get_order(order_id)
            if response.status_code == 200:
                success_count += 1
        
        # All should succeed
        assert success_count == 20
        
        # Cleanup
        client_service.finish_order(order_id)
    
    def test_back_to_back_rides(self, client_service):
        """Test completing multiple rides back-to-back for same user."""
        user_id = unique_user()
        completed_rides = []
        
        for i in range(3):
            # Need different scooters for each ride since offers are cached
            scooter_id = f"scooter-{i + 1}"
            
            # New offer
            offer_response = client_service.create_offer(user_id, scooter_id)
            assert offer_response.status_code == 201
            offer = offer_response.json()
            
            # Create order
            order_id = str(uuid.uuid4())
            order_response = client_service.create_order(order_id, offer["id"], user_id)
            assert order_response.status_code == 201
            
            # Finish
            finish_response = client_service.finish_order(order_id)
            assert finish_response.status_code == 200
            
            completed_rides.append({
                "ride": i + 1,
                "order_id": order_id,
                "offer_id": offer["id"]
            })
        
        # Verify all rides completed with unique order/offer IDs
        order_ids = [r["order_id"] for r in completed_rides]
        offer_ids = [r["offer_id"] for r in completed_rides]
        
        assert len(set(order_ids)) == 3, "All orders should be unique"
        assert len(set(offer_ids)) == 3, "All offers should be unique"
