package jobs

import (
	"client/api"
	"client/internal/domain/orders"
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/rs/zerolog"
)

var cleanupLogger zerolog.Logger

func init() {
	cleanupLogger = zerolog.New(os.Stdout).
		With().
		Timestamp().
		Logger().
		Level(zerolog.InfoLevel)
}

type OrderCleanupJob struct {
	repo      orders.Repository
	olderThan time.Duration
	interval  time.Duration
	stopChan  chan struct{}
	doneChan  chan struct{}
}

func NewOrderCleanupJob(repo orders.Repository, olderThan time.Duration, interval time.Duration) *OrderCleanupJob {
	return &OrderCleanupJob{
		repo:      repo,
		olderThan: olderThan,
		interval:  interval,
		stopChan:  make(chan struct{}),
		doneChan:  make(chan struct{}),
	}
}

func (j *OrderCleanupJob) Start() {
	go j.run()
}

func (j *OrderCleanupJob) Stop() {
	close(j.stopChan)
	<-j.doneChan
}

func (j *OrderCleanupJob) run() {
	defer close(j.doneChan)

	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()

	j.cleanup()

	for {
		select {
		case <-ticker.C:
			j.cleanup()
		case <-j.stopChan:
			cleanupLogger.Info().Msg("Order cleanup job stopped")
			return
		}
	}
}

func (j *OrderCleanupJob) cleanup() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	startTime := time.Now()

	cleanupLogger.Info().
		Str("event", "cleanup_started").
		Dur("older_than", j.olderThan).
		Msg("Starting order cleanup")

	oldOrders, err := j.repo.GetOldOrders(ctx, j.olderThan)
	if err != nil {
		cleanupLogger.Error().
			Str("event", "cleanup_error").
			Err(err).
			Msg("Failed to get old orders")
		return
	}

	if len(oldOrders) == 0 {
		cleanupLogger.Info().
			Str("event", "cleanup_completed").
			Int("orders_deleted", 0).
			Dur("duration_ms", time.Since(startTime)).
			Msg("No old orders to delete")
		return
	}

	j.logOrders(oldOrders)

	orderIDs := make([]string, 0, len(oldOrders))
	for _, order := range oldOrders {
		orderIDs = append(orderIDs, order.Id)
	}

	if err := j.repo.DeleteOrders(ctx, orderIDs); err != nil {
		cleanupLogger.Error().
			Str("event", "cleanup_error").
			Err(err).
			Int("orders_count", len(orderIDs)).
			Msg("Failed to delete orders")
		return
	}

	duration := time.Since(startTime)
	cleanupLogger.Info().
		Str("event", "cleanup_completed").
		Int("orders_deleted", len(orderIDs)).
		Dur("duration_ms", duration).
		Msg("Order cleanup completed successfully")
}

func (j *OrderCleanupJob) logOrders(orders []*api.Order) {
	for _, order := range orders {
		orderJSON, err := json.Marshal(order)
		if err != nil {
			cleanupLogger.Warn().
				Str("event", "cleanup_log_error").
				Str("order_id", order.Id).
				Err(err).
				Msg("Failed to marshal order for logging")
			continue
		}

		cleanupLogger.Info().
			Str("event", "order_deleted").
			Str("order_id", order.Id).
			Str("user_id", order.UserId).
			Str("status", string(order.Status)).
			RawJSON("order_data", orderJSON).
			Msg("Deleting old order")
	}
}
