"""
Health check tests for all services.
"""

import pytest


class TestHealthEndpoints:
    """Tests for service health endpoints."""
    
    def test_client_service_health(self, client_service):
        """Test that client service health endpoint returns 200."""
        response = client_service.session.get(f"{client_service.base_url}/health")
        
        assert response.status_code == 200
        assert response.text == "OK"
    
    def test_external_service_health(self, external_service):
        """Test that external service health endpoint returns 200."""
        response = external_service.session.get(f"{external_service.base_url}/health")
        
        assert response.status_code == 200
        assert response.text == "OK"
