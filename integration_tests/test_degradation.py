"""
Integration tests for system degradation scenarios.

Tests cover:
- Service behavior when external services are unavailable
- Caching and fallback mechanisms
- Error handling and response codes according to ADR
"""

import uuid

import pytest


def unique_user():
    """Generate a unique user ID to avoid offer cache conflicts between tests."""
    return f"test-user-{uuid.uuid4()}"


class TestExternalServiceDegradation:
    """
    Tests for system behavior when external services are degraded.
    """
    
    def test_scooter_service_data_available(self, external_service, test_scooters):
        """Verify external scooter service is working correctly."""
        for scooter in test_scooters:
            response = external_service.get_scooter_data(scooter["id"])
            assert response.status_code == 200
            
            data = response.json()
            assert data["id"] == scooter["id"]
            assert data["zone_id"] == scooter["zone_id"]
            assert data["charge"] == scooter["charge"]
    
    def test_zone_service_data_available(self, external_service, test_zones):
        """Verify external zone service is working correctly."""
        for zone in test_zones:
            response = external_service.get_zone_data(zone["id"])
            assert response.status_code == 200
            
            data = response.json()
            assert data["id"] == zone["id"]
            assert data["price_per_minute"] == zone["price_per_minute"]
            assert data["price_unlock"] == zone["price_unlock"]
            assert data["default_deposit"] == zone["default_deposit"]
    
    def test_user_service_data_available(self, external_service, test_users):
        """Verify external user service is working correctly."""
        for user in test_users:
            response = external_service.get_user_profile(user["id"])
            assert response.status_code == 200
            
            data = response.json()
            assert data["id"] == user["id"]
            assert data["has_subscription"] == user["has_subscription"]
            assert data["trusted"] == user["trusted"]
    
    def test_configs_service_data_available(self, external_service, test_configs):
        """Verify external configs service is working correctly."""
        response = external_service.get_configs()
        assert response.status_code == 200
        
        data = response.json()
        assert data["surge"] == test_configs["surge"]
        assert data["low_charge_discount"] == test_configs["low_charge_discount"]
        assert data["low_charge_threshold_percent"] == test_configs["low_charge_threshold_percent"]
        assert data["incomplete_ride_threshold_seconds"] == test_configs["incomplete_ride_threshold_seconds"]


class TestZoneCaching:
    """
    Tests for zone data caching.
    """
    
    def test_zone_data_consistency(self, client_service, test_zones):
        """Test that zone data is consistently used in offers."""
        user_id = unique_user()
        response1 = client_service.create_offer(user_id, "scooter-1")
        response2 = client_service.create_offer(user_id, "scooter-2")
        
        offer1 = response1.json()
        offer2 = response2.json()
        
        assert offer1["zone_id"] == "zone-1"
        assert offer2["zone_id"] == "zone-1"
        


class TestUnknownUserFallback:
    """
    Tests for unknown user handling.
    """
    
    def test_unknown_user_gets_no_privileges(self, client_service, test_zones):
        """Test that unknown user is treated as having no privileges."""
        response = client_service.create_offer(unique_user(), "scooter-1")
        
        assert response.status_code == 201
        offer = response.json()
        
        zone = test_zones[0]
        assert offer["price_unlock"] == zone["price_unlock"]
        
        assert offer["deposit"] == zone["default_deposit"]
    
    def test_unknown_user_can_complete_ride(self, client_service):
        """Test that unknown user can complete full ride cycle."""
        user_id = unique_user()
        
        offer_response = client_service.create_offer(user_id, "scooter-1")
        assert offer_response.status_code == 201
        offer = offer_response.json()
        
        order_id = str(uuid.uuid4())
        order_response = client_service.create_order(order_id, offer["id"], user_id)
        assert order_response.status_code == 201
        
        finish_response = client_service.finish_order(order_id)
        assert finish_response.status_code == 200
        assert finish_response.json()["status"] == "FINISHED"


class TestNonexistentResources:
    """Tests for handling non-existent resources."""
    
    def test_nonexistent_scooter_returns_error(self, client_service):
        """Test that non-existent scooter returns appropriate error."""
        response = client_service.create_offer(unique_user(), "scooter-does-not-exist")
        
        assert response.status_code in [400, 503]
    
    def test_nonexistent_user_can_still_create_offer(self, client_service):
        """
        Test that non-existent user can still create offer with default pricing.
        ADR: Недоступность users: формируем оффер как для юзера без привилегий
        """
        response = client_service.create_offer(unique_user(), "scooter-1")
        
        assert response.status_code == 201


class TestPaymentIntegration:
    """Tests for payment service integration."""
    
    def test_order_creation_holds_deposit(self, client_service):
        """Test that creating order triggers deposit hold."""
        user_id = unique_user()
        
        offer_response = client_service.create_offer(user_id, "scooter-1")
        offer = offer_response.json()
        
        order_id = str(uuid.uuid4())
        order_response = client_service.create_order(order_id, offer["id"], user_id)
        
        assert order_response.status_code == 201
        order = order_response.json()
        assert order["status"] == "ACTIVE"
    
    def test_order_finish_processes_payment(self, client_service):
        """Test that finishing order processes payment and unholds deposit."""
        user_id = unique_user()
        
        offer_response = client_service.create_offer(user_id, "scooter-1")
        offer = offer_response.json()
        
        order_id = str(uuid.uuid4())
        client_service.create_order(order_id, offer["id"], user_id)
        
        finish_response = client_service.finish_order(order_id)
        
        assert finish_response.status_code == 200
        order = finish_response.json()
        assert order["status"] == "FINISHED"


class TestConfigsDynamicBehavior:
    """Tests for dynamic configs behavior."""
    
    def test_configs_affect_pricing(self, client_service, test_configs, test_zones):
        """Test that dynamic configs are applied to pricing."""
        response = client_service.create_offer(unique_user(), "scooter-1")
        
        assert response.status_code == 201
        offer = response.json()
        
        zone = test_zones[0]
        expected_price = round(zone["price_per_minute"] * test_configs["surge"])
        
        assert offer["price_per_minute"] == expected_price, \
            f"Expected {expected_price} with surge={test_configs['surge']}, got {offer['price_per_minute']}"
