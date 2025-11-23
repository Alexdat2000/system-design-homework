package redis

import (
	"client/api"
	"context"
	"fmt"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
)

func TestOrderCache_SetGetInvalidate(t *testing.T) {
	// Start in-memory Redis
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer s.Close()

	// Create client for miniredis
	url := fmt.Sprintf("redis://%s/0", s.Addr())
	rcli, err := NewClient(url)
	if err != nil {
		t.Fatalf("failed to create redis client: %v", err)
	}
	defer rcli.Close()

	cache := NewOrderCache(rcli)
	ctx := context.Background()

	// Data
	now := time.Now()
	order := &api.Order{
		Id:        "order-cache-1",
		UserId:    "user-1",
		ScooterId: "scooter-1",
		StartTime: now,
		Status:    api.ACTIVE,
	}

	// Set
	if err := cache.SetOrder(ctx, order, time.Minute); err != nil {
		t.Fatalf("SetOrder error: %v", err)
	}

	// Get
	got, err := cache.GetOrder(ctx, order.Id)
	if err != nil {
		t.Fatalf("GetOrder error: %v", err)
	}
	if got == nil || got.Id != order.Id || got.UserId != order.UserId {
		t.Fatalf("unexpected order from cache: %+v", got)
	}

	// Invalidate
	if err := cache.Invalidate(ctx, order.Id); err != nil {
		t.Fatalf("Invalidate error: %v", err)
	}

	// Ensure removed
	got2, err := cache.GetOrder(ctx, order.Id)
	if err != nil {
		t.Fatalf("GetOrder after invalidate error: %v", err)
	}
	if got2 != nil {
		t.Fatalf("expected nil after invalidate, got: %+v", got2)
	}
}

func TestOrderCache_TTL_Expiry(t *testing.T) {
	// Start in-memory Redis
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer s.Close()

	// Create client for miniredis
	url := fmt.Sprintf("redis://%s/0", s.Addr())
	rcli, err := NewClient(url)
	if err != nil {
		t.Fatalf("failed to create redis client: %v", err)
	}
	defer rcli.Close()

	cache := NewOrderCache(rcli)
	ctx := context.Background()

	order := &api.Order{
		Id:        "order-cache-ttl",
		UserId:    "user-ttl",
		ScooterId: "scooter-ttl",
		StartTime: time.Now(),
		Status:    api.ACTIVE,
	}

	// Set with short TTL
	if err := cache.SetOrder(ctx, order, 2*time.Second); err != nil {
		t.Fatalf("SetOrder error: %v", err)
	}

	// Should exist immediately
	got, err := cache.GetOrder(ctx, order.Id)
	if err != nil {
		t.Fatalf("GetOrder error: %v", err)
	}
	if got == nil {
		t.Fatalf("expected order present before expiry")
	}

	// Fast forward miniredis time past TTL
	s.FastForward(3 * time.Second)

	// Should be expired
	got2, err := cache.GetOrder(ctx, order.Id)
	if err != nil {
		t.Fatalf("GetOrder after expiry error: %v", err)
	}
	if got2 != nil {
		t.Fatalf("expected nil after expiry, got: %+v", got2)
	}
}