"""
Integration tests for the Offer creation functionality.

Tests cover:
- Creating offers for valid user/scooter combinations
- Pricing calculations based on user status and scooter charge
- Idempotency of offer creation
- Offer expiration handling
- Error handling for invalid inputs
"""

import math
import uuid

import pytest


class TestOfferCreation:
    """Tests for POST /offers endpoint."""
    
    def test_create_offer_success(self, client_service, test_users, test_scooters):
        """Test successful offer creation with valid user and scooter."""
        user = test_users[0]  # user-1: has_subscription=True, trusted=True
        scooter = test_scooters[0]  # scooter-1: zone-1, charge=85
        
        response = client_service.create_offer(user["id"], scooter["id"])
        
        assert response.status_code == 201, f"Expected 201, got {response.status_code}: {response.text}"
        
        offer = response.json()
        
        assert "id" in offer
        assert offer["user_id"] == user["id"]
        assert offer["scooter_id"] == scooter["id"]
        assert "zone_id" in offer
        assert "expires_at" in offer
        assert "price_per_minute" in offer
        assert "price_unlock" in offer
        assert "deposit" in offer
    
    def test_create_offer_subscription_user_free_unlock(self, client_service, test_zones, test_configs):
        """
        Test that users with subscription get free unlock (price_unlock = 0).
        ADR: стоимость разблокировки самоката (0 если есть подписка)
        """
        # user-1 has subscription
        response = client_service.create_offer("user-1", "scooter-1")
        
        assert response.status_code == 201
        offer = response.json()
        
        # Subscription user should have free unlock
        assert offer["price_unlock"] == 0, "Subscription user should have free unlock"
    
    def test_create_offer_non_subscription_user_pays_unlock(self, client_service, test_zones, test_configs):
        """
        Test that users without subscription pay for unlock.
        """
        # user-2 has no subscription
        response = client_service.create_offer("user-2", "scooter-1")
        
        assert response.status_code == 201
        offer = response.json()
        
        # Non-subscription user should pay for unlock
        # Zone-1 has price_unlock=50
        zone_price_unlock = 50
        assert offer["price_unlock"] == zone_price_unlock, \
            f"Non-subscription user should pay {zone_price_unlock} for unlock"
    
    def test_create_offer_trusted_user_no_deposit(self, client_service):
        """
        Test that trusted users don't need to pay deposit.
        ADR: депозит (в зависимости от доверенности пользователя)
        """
        # user-1 is trusted
        response = client_service.create_offer("user-1", "scooter-1")
        
        assert response.status_code == 201
        offer = response.json()
        
        # Trusted user should have no deposit
        assert offer["deposit"] == 0, "Trusted user should have no deposit"
    
    def test_create_offer_untrusted_user_pays_deposit(self, client_service, test_zones):
        """
        Test that untrusted users must pay deposit.
        """
        # user-3 is not trusted
        response = client_service.create_offer("user-3", "scooter-1")
        
        assert response.status_code == 201
        offer = response.json()
        
        # Zone-1 has default_deposit=200
        expected_deposit = 200
        assert offer["deposit"] == expected_deposit, \
            f"Untrusted user should pay {expected_deposit} deposit"
    
    def test_create_offer_surge_pricing(self, client_service, test_zones, test_configs):
        """
        Test that surge multiplier is applied to price per minute.
        """
        # user-3: no subscription, not trusted - gets full pricing
        response = client_service.create_offer("user-3", "scooter-1")
        
        assert response.status_code == 201
        offer = response.json()
        
        # Zone-1 has price_per_minute=7, surge=1.2
        # scooter-1 has charge=85 (above low charge threshold of 28)
        base_price = 7
        surge = 1.2
        expected_price = round(base_price * surge)
        
        assert offer["price_per_minute"] == expected_price, \
            f"Expected price_per_minute={expected_price} with surge, got {offer['price_per_minute']}"
    
    def test_create_offer_low_charge_discount(self, client_service, test_zones, test_configs):
        """
        Test that low charge discount is applied for low battery scooters.
        ADR: low_charge_discount при заряде ниже порога
        """
        # scooter-4 has charge=25, below threshold of 28
        response = client_service.create_offer("user-3", "scooter-4")
        
        assert response.status_code == 201
        offer = response.json()
        
        # Zone-2 has price_per_minute=10, surge=1.2, low_charge_discount=0.7
        base_price = 10
        surge = 1.2
        low_charge_discount = 0.7
        expected_price = round(base_price * surge * low_charge_discount)
        
        assert offer["price_per_minute"] == expected_price, \
            f"Expected price_per_minute={expected_price} with low charge discount, got {offer['price_per_minute']}"
    
    def test_create_offer_zone_assignment(self, client_service, test_scooters):
        """Test that offer gets correct zone from scooter."""
        # scooter-1 is in zone-1
        response = client_service.create_offer("user-1", "scooter-1")
        
        assert response.status_code == 201
        offer = response.json()
        assert offer["zone_id"] == "zone-1"
        
        # scooter-3 is in zone-2
        response = client_service.create_offer("user-2", "scooter-3")
        
        assert response.status_code == 201
        offer = response.json()
        assert offer["zone_id"] == "zone-2"
    
    def test_create_offer_idempotency_same_user_scooter(self, client_service):
        """
        Test idempotency: same user/scooter combination returns same offer.
        ADR: если оффер с такими параметрами (user_id, scooter_id) уже существует и валиден,
        возвращаем существующий из кэша
        """
        user_id = "user-1"
        scooter_id = "scooter-1"
        
        # First request
        response1 = client_service.create_offer(user_id, scooter_id)
        assert response1.status_code == 201
        offer1 = response1.json()
        
        # Second request with same params
        response2 = client_service.create_offer(user_id, scooter_id)
        assert response2.status_code == 201
        offer2 = response2.json()
        
        # Should return same offer
        assert offer1["id"] == offer2["id"], \
            "Idempotent requests should return the same offer"
    
    def test_create_offer_different_scooter_new_offer(self, client_service):
        """Test that different scooter for same user creates new offer."""
        user_id = "user-1"
        
        response1 = client_service.create_offer(user_id, "scooter-1")
        assert response1.status_code == 201
        offer1 = response1.json()
        
        response2 = client_service.create_offer(user_id, "scooter-2")
        assert response2.status_code == 201
        offer2 = response2.json()
        
        # Different scooters should create different offers
        assert offer1["id"] != offer2["id"], \
            "Different scooters should create different offers"
    
    def test_create_offer_missing_user_id(self, client_service):
        """Test error when user_id is missing."""
        response = client_service.post("/offers", json={
            "scooter_id": "scooter-1"
        })
        
        assert response.status_code == 400
    
    def test_create_offer_missing_scooter_id(self, client_service):
        """Test error when scooter_id is missing."""
        response = client_service.post("/offers", json={
            "user_id": "user-1"
        })
        
        assert response.status_code == 400
    
    def test_create_offer_empty_request_body(self, client_service):
        """Test error when request body is empty."""
        response = client_service.post("/offers", json={})
        
        assert response.status_code == 400
    
    def test_create_offer_invalid_scooter(self, client_service):
        """Test error when scooter doesn't exist."""
        response = client_service.create_offer("user-1", "nonexistent-scooter")
        
        # Should return 400 (scooter not found) or 503 (scooters service error)
        assert response.status_code in [400, 503]
    
    def test_create_offer_unknown_user_default_pricing(self, client_service, test_zones):
        """
        Test that unknown user gets default pricing (no subscription, not trusted).
        ADR: Недоступность users: формируем оффер как для юзера без привилегий
        """
        # Use a user that doesn't exist in external service
        response = client_service.create_offer("unknown-user", "scooter-1")
        
        # Should still create offer but with default pricing
        assert response.status_code == 201
        offer = response.json()
        
        # Default: no subscription (pays unlock), not trusted (pays deposit)
        zone_price_unlock = 50  # zone-1
        zone_default_deposit = 200  # zone-1
        
        assert offer["price_unlock"] == zone_price_unlock, \
            "Unknown user should pay for unlock"
        assert offer["deposit"] == zone_default_deposit, \
            "Unknown user should pay deposit"


class TestOfferPricingMatrix:
    """
    Comprehensive pricing tests covering all user/scooter combinations.
    """
    
    @pytest.mark.parametrize("user_id,scooter_id,expected_free_unlock,expected_no_deposit", [
        # user-1: subscription=True, trusted=True
        ("user-1", "scooter-1", True, True),
        # user-2: subscription=False, trusted=True
        ("user-2", "scooter-1", False, True),
        # user-3: subscription=False, trusted=False
        ("user-3", "scooter-1", False, False),
        # user-4: subscription=True, trusted=False
        ("user-4", "scooter-1", True, False),
    ])
    def test_pricing_by_user_status(
        self, client_service, user_id, scooter_id, expected_free_unlock, expected_no_deposit
    ):
        """Test pricing varies correctly based on user subscription and trust status."""
        response = client_service.create_offer(user_id, scooter_id)
        
        assert response.status_code == 201
        offer = response.json()
        
        if expected_free_unlock:
            assert offer["price_unlock"] == 0, \
                f"User {user_id} should have free unlock"
        else:
            assert offer["price_unlock"] > 0, \
                f"User {user_id} should pay for unlock"
        
        if expected_no_deposit:
            assert offer["deposit"] == 0, \
                f"User {user_id} should have no deposit"
        else:
            assert offer["deposit"] > 0, \
                f"User {user_id} should pay deposit"
    
    @pytest.mark.parametrize("scooter_id,has_low_charge", [
        # scooter-1: charge=85, threshold=28 -> no discount
        ("scooter-1", False),
        # scooter-2: charge=45, threshold=28 -> no discount
        ("scooter-2", False),
        # scooter-3: charge=90, threshold=28 -> no discount
        ("scooter-3", False),
        # scooter-4: charge=25, threshold=28 -> discount applied
        ("scooter-4", True),
    ])
    def test_low_charge_discount_application(
        self, client_service, scooter_id, has_low_charge, test_zones, test_scooters, test_configs
    ):
        """Test that low charge discount is applied correctly based on battery level."""
        response = client_service.create_offer("user-3", scooter_id)
        
        assert response.status_code == 201
        offer = response.json()
        
        # Find scooter and zone data
        scooter = next(s for s in test_scooters if s["id"] == scooter_id)
        zone = next(z for z in test_zones if z["id"] == scooter["zone_id"])
        
        base_price = zone["price_per_minute"]
        surge = test_configs["surge"]
        discount = test_configs["low_charge_discount"] if has_low_charge else 1.0
        
        expected_price = round(base_price * surge * discount)
        
        assert offer["price_per_minute"] == expected_price, \
            f"Expected {expected_price} for {scooter_id} (low_charge={has_low_charge}), got {offer['price_per_minute']}"
