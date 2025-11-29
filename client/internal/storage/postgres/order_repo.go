package postgres

import (
	"client/api"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type OrderRepository struct {
	db *DB
}

func NewOrderRepository(db *DB) *OrderRepository {
	return &OrderRepository{db: db}
}

func (r *OrderRepository) CreateOrder(ctx context.Context, order *api.Order, transactionID string) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	totalAmount := 0
	if order.CurrentAmount != nil {
		totalAmount = *order.CurrentAmount
	} else if order.PriceUnlock != nil {
		totalAmount = *order.PriceUnlock
	}

	orderQuery := `
		INSERT INTO orders (
			id, user_id, scooter_id, offer_id, 
			price_per_minute, price_unlock, deposit, total_amount,
			status, start_time, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`

	now := time.Now()
	_, err = tx.Exec(ctx, orderQuery,
		order.Id,
		order.UserId,
		order.ScooterId,
		order.OfferId,
		getIntValue(order.PricePerMinute),
		getIntValue(order.PriceUnlock),
		getIntValue(order.Deposit),
		totalAmount,
		string(order.Status),
		order.StartTime,
		now,
		now,
	)
	if err != nil {
		return fmt.Errorf("failed to insert order: %w", err)
	}

	depositAmount := 0
	if order.Deposit != nil {
		depositAmount = *order.Deposit
	}

	transactionQuery := `
		INSERT INTO payment_transactions (
			id, order_id, user_id, transaction_type, 
			amount, status, external_transaction_id, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	transactionUUID := fmt.Sprintf("txn-%s", order.Id)
	_, err = tx.Exec(ctx, transactionQuery,
		transactionUUID,
		order.Id,
		order.UserId,
		"HOLD",
		depositAmount,
		"SUCCESS",
		transactionID,
		now,
	)
	if err != nil {
		return fmt.Errorf("failed to insert payment transaction: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (r *OrderRepository) GetOrderByID(ctx context.Context, orderID string) (*api.Order, error) {
	query := `
		SELECT 
			id, user_id, scooter_id, offer_id,
			price_per_minute, price_unlock, deposit, total_amount,
			status, start_time, finish_time, duration_seconds
		FROM orders
		WHERE id = $1
	`

	var order api.Order
	var status string
	var finishTime *time.Time
	var durationSeconds *int

	err := r.db.Pool.QueryRow(ctx, query, orderID).Scan(
		&order.Id,
		&order.UserId,
		&order.ScooterId,
		&order.OfferId,
		&order.PricePerMinute,
		&order.PriceUnlock,
		&order.Deposit,
		&order.CurrentAmount, // Using CurrentAmount for total_amount
		&status,
		&order.StartTime,
		&finishTime,
		&durationSeconds,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query order: %w", err)
	}

	order.Status = api.OrderStatus(status)
	order.FinishTime = finishTime
	order.DurationSeconds = durationSeconds

	return &order, nil
}

func (r *OrderRepository) GetOrderByOfferID(ctx context.Context, offerID string) (*api.Order, error) {
	query := `
		SELECT
			id, user_id, scooter_id, offer_id,
			price_per_minute, price_unlock, deposit, total_amount,
			status, start_time, finish_time, duration_seconds
		FROM orders
		WHERE offer_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`

	var order api.Order
	var status string
	var finishTime *time.Time
	var durationSeconds *int

	err := r.db.Pool.QueryRow(ctx, query, offerID).Scan(
		&order.Id,
		&order.UserId,
		&order.ScooterId,
		&order.OfferId,
		&order.PricePerMinute,
		&order.PriceUnlock,
		&order.Deposit,
		&order.CurrentAmount,
		&status,
		&order.StartTime,
		&finishTime,
		&durationSeconds,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query order by offer_id: %w", err)
	}

	order.Status = api.OrderStatus(status)
	order.FinishTime = finishTime
	order.DurationSeconds = durationSeconds

	return &order, nil
}

func (r *OrderRepository) FinishOrder(ctx context.Context, orderID string, finishTime time.Time, durationSeconds int, totalAmount int, finalStatus api.OrderStatus, chargeSuccess bool, unholdSuccess bool, chargeTxID string) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	now := time.Now()

	updateOrder := `
		UPDATE orders
		SET finish_time = $1,
			duration_seconds = $2,
			total_amount = $3,
			status = $4,
			updated_at = $5
		WHERE id = $6
	`
	if _, err := tx.Exec(ctx, updateOrder, finishTime, durationSeconds, totalAmount, string(finalStatus), now, orderID); err != nil {
		return fmt.Errorf("failed to update order on finish: %w", err)
	}

	var userID string
	var deposit int
	row := tx.QueryRow(ctx, `SELECT user_id, deposit FROM orders WHERE id = $1`, orderID)
	if err := row.Scan(&userID, &deposit); err != nil {
		return fmt.Errorf("failed to fetch order for transactions: %w", err)
	}

	clearStatus := "FAILED"
	if chargeSuccess {
		clearStatus = "SUCCESS"
	}
	clearID := fmt.Sprintf("txn-clear-%s", orderID)
	if _, err := tx.Exec(ctx, `
		INSERT INTO payment_transactions (
			id, order_id, user_id, transaction_type, amount, status, external_transaction_id, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`,
		clearID, orderID, userID, "CLEAR", totalAmount, clearStatus, chargeTxID, now,
	); err != nil {
		return fmt.Errorf("failed to insert CLEAR transaction: %w", err)
	}

	refundStatus := "FAILED"
	if unholdSuccess {
		refundStatus = "SUCCESS"
	}
	refundID := fmt.Sprintf("txn-refund-%s", orderID)
	if _, err := tx.Exec(ctx, `
		INSERT INTO payment_transactions (
			id, order_id, user_id, transaction_type, amount, status, external_transaction_id, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`,
		refundID, orderID, userID, "REFUND", deposit, refundStatus, "", now,
	); err != nil {
		return fmt.Errorf("failed to insert REFUND transaction: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit finish transaction: %w", err)
	}
	return nil
}

func getIntValue(ptr *int) int {
	if ptr == nil {
		return 0
	}
	return *ptr
}
