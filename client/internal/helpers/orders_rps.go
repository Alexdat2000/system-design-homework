package helpers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
	"time"

	"client/internal/storage/postgres"

	"github.com/go-chi/chi/v5"
)

// OrdersRPSCollector tracks request counts for /orders endpoints.
//
// We intentionally count by route pattern (chi) so the metric is stable:
// - POST /orders
// - GET /orders/{order_id} (reported as "GET /orders" in aggregated dashboards)
type OrdersRPSCollector struct {
	getOrders  atomic.Uint64
	postOrders atomic.Uint64
}

func (c *OrdersRPSCollector) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)

		routePattern := chi.RouteContext(r.Context()).RoutePattern()

		switch r.Method {
		case http.MethodPost:
			// Count only create order.
			if routePattern == "/orders" {
				c.postOrders.Add(1)
			}
		case http.MethodGet:
			// Count getting an order as GET /orders (since OpenAPI is /orders/{order_id}).
			if routePattern == "/orders/{order_id}" || routePattern == "/orders" {
				c.getOrders.Add(1)
			}
		}
	})
}

func (c *OrdersRPSCollector) snapshotAndReset() (getOrders uint64, postOrders uint64) {
	return c.getOrders.Swap(0), c.postOrders.Swap(0)
}

func EnsureOrdersRPSMinuteTable(ctx context.Context, db *postgres.DB) error {
	ddl := `
CREATE TABLE IF NOT EXISTS orders_rps_minute (
    minute_ts TIMESTAMP PRIMARY KEY,
    get_orders_count INTEGER NOT NULL DEFAULT 0,
    post_orders_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);`
	_, err := db.Pool.Exec(ctx, ddl)
	if err != nil {
		return fmt.Errorf("ensure orders_rps_minute table: %w", err)
	}

	// If the table existed before we added updated_at, add it (idempotent).
	_, _ = db.Pool.Exec(ctx, `ALTER TABLE orders_rps_minute ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP`)

	return nil
}

// StartOrdersRPSWriter flushes counters to Postgres once per minute.
// It aligns flushes to the minute boundary and writes counts for the *previous* minute.
func StartOrdersRPSWriter(ctx context.Context, db *postgres.DB, c *OrdersRPSCollector) {
	// Align to the next minute boundary.
	next := time.Now().UTC().Truncate(time.Minute).Add(time.Minute)
	timer := time.NewTimer(time.Until(next))
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case t := <-timer.C:
			// We flushed at the boundary; attribute counts to the previous minute window.
			minuteTS := t.UTC().Truncate(time.Minute).Add(-time.Minute)
			getCnt, postCnt := c.snapshotAndReset()

			writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			_, err := db.Pool.Exec(
				writeCtx,
				`INSERT INTO orders_rps_minute (minute_ts, get_orders_count, post_orders_count, updated_at)
				 VALUES ($1, $2, $3, CURRENT_TIMESTAMP)
				 ON CONFLICT (minute_ts) DO UPDATE
				 SET get_orders_count = orders_rps_minute.get_orders_count + EXCLUDED.get_orders_count,
				     post_orders_count = orders_rps_minute.post_orders_count + EXCLUDED.post_orders_count,
				     updated_at = CURRENT_TIMESTAMP`,
				minuteTS,
				int64(getCnt),
				int64(postCnt),
			)
			cancel()
			if err != nil {
				// Best-effort metric. We log and continue.
				log.Printf("orders_rps_minute flush failed: %v", err)
			}

			// Schedule next tick precisely.
			next = t.UTC().Truncate(time.Minute).Add(time.Minute)
			timer.Reset(time.Until(next))
		}
	}
}


