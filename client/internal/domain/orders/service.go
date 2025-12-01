package orders

import (
	"client/api"
	"client/internal/domain/offers"
	"client/internal/external"
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"golang.org/x/sync/singleflight"
)

var (
	ErrOfferNotFound = errors.New("offer not found")
	ErrOfferExpired = errors.New("offer expired")
	ErrOfferAlreadyUsed = errors.New("offer already used")
	ErrInvalidUser = errors.New("user_id doesn't match offer")
	ErrOrderNotActive = errors.New("order not active")
	ErrNoSuchOrder = errors.New("order not found")
)

type OrderCache interface {
	GetOrder(ctx context.Context, orderID string) (*api.Order, error)
	SetOrder(ctx context.Context, order *api.Order, ttl time.Duration) error
	Invalidate(ctx context.Context, orderID string) error
}

// ConfigsProvider provides access to dynamic configuration
type ConfigsProvider interface {
	GetConfigs(ctx context.Context) (*external.DynamicConfigs, error)
}

type ServiceInterface interface {
	CreateOrder(ctx context.Context, req *CreateOrderRequest) (*api.Order, error)
	GetOrder(ctx context.Context, orderID string) (*api.Order, error)
	FinishOrder(ctx context.Context, orderID string) (*api.Order, error)
}

type Service struct {
	orderRepo      Repository
	offerRepo      offers.Repository
	paymentsClient external.PaymentsClientInterface
	cache          OrderCache
	singleFlight   *singleflight.Group
	configsProvider ConfigsProvider
}

func NewService(orderRepo Repository, offerRepo offers.Repository, paymentsClient external.PaymentsClientInterface) *Service {
	return NewServiceWithCache(orderRepo, offerRepo, paymentsClient, nil, nil)
}

func NewServiceWithCache(orderRepo Repository, offerRepo offers.Repository, paymentsClient external.PaymentsClientInterface, cache OrderCache, configsProvider ConfigsProvider) *Service {
	return &Service{
		orderRepo:       orderRepo,
		offerRepo:       offerRepo,
		paymentsClient:  paymentsClient,
		cache:           cache,
		singleFlight:    &singleflight.Group{},
		configsProvider: configsProvider,
	}
}

type CreateOrderRequest struct {
	OrderID string
	OfferID string
	UserID  string
}

func (s *Service) CreateOrder(ctx context.Context, req *CreateOrderRequest) (*api.Order, error) {
	if req.OrderID == "" {
		return nil, fmt.Errorf("order_id is required")
	}
	existingOrder, err := s.orderRepo.GetOrderByID(ctx, req.OrderID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing order: %w", err)
	}
	if existingOrder != nil {
		return existingOrder, nil
	}

	offer, err := s.offerRepo.GetOffer(ctx, req.OfferID)
	if err != nil {
		return nil, fmt.Errorf("failed to get offer: %w", err)
	}
	if offer == nil {
		return nil, ErrOfferNotFound
	}

	if time.Now().After(offer.ExpiresAt) {
		return nil, ErrOfferExpired
	}

	if offer.UserId != req.UserID {
		return nil, ErrInvalidUser
	}

	marked, err := s.offerRepo.MarkOfferAsUsed(ctx, req.OfferID)
	if err != nil {
		return nil, fmt.Errorf("failed to mark offer as used: %w", err)
	}
	if !marked {
		return nil, ErrOfferAlreadyUsed
	}

	orderID := req.OrderID

	holdReq := &external.PaymentHoldRequest{
		UserID:  req.UserID,
		OrderID: orderID,
		Amount:  offer.Deposit,
	}
	holdResp, err := s.paymentsClient.HoldMoneyForOrder(ctx, holdReq)
	if err != nil {
		return nil, fmt.Errorf("failed to hold deposit: %w", err)
	}
	if !holdResp.Success {
		return nil, fmt.Errorf("payment hold failed")
	}

	now := time.Now()
	order := &api.Order{
		Id:             orderID,
		OfferId:        req.OfferID,
		UserId:         req.UserID,
		ScooterId:      offer.ScooterId,
		Status:         api.ACTIVE,
		StartTime:      now,
		PricePerMinute: &offer.PricePerMinute,
		PriceUnlock:    &offer.PriceUnlock,
		Deposit:        &offer.Deposit,
		CurrentAmount:  &offer.PriceUnlock,
	}

	if err := s.orderRepo.CreateOrder(ctx, order, holdResp.TransactionID); err != nil {
		_ = s.paymentsClient.UnholdMoneyForOrder(ctx, orderID)
		return nil, fmt.Errorf("failed to create order: %w", err)
	}

	if s.cache != nil {
		_ = s.cache.SetOrder(ctx, order, 30*time.Minute)
	}

	return order, nil
}

func (s *Service) GetOrder(ctx context.Context, orderID string) (*api.Order, error) {
	if orderID == "" {
		return nil, fmt.Errorf("order_id is required")
	}

	if s.cache != nil {
		if cached, err := s.cache.GetOrder(ctx, orderID); err == nil && cached != nil {
			return cached, nil
		}
	}

	result, err, _ := s.singleFlight.Do(orderID, func() (interface{}, error) {
		order, err := s.orderRepo.GetOrderByID(ctx, orderID)
		if err != nil {
			return nil, fmt.Errorf("failed to get order: %w", err)
		}
		if order == nil {
			return nil, nil
		}

		if s.cache != nil {
			_ = s.cache.SetOrder(ctx, order, 30*time.Minute)
		}

		return order, nil
	})

	if err != nil {
		return nil, err
	}

	if result == nil {
		return nil, nil
	}

	return result.(*api.Order), nil
}

func (s *Service) FinishOrder(ctx context.Context, orderID string) (*api.Order, error) {
	order, err := s.orderRepo.GetOrderByID(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("failed to get order: %w", err)
	}
	if order == nil {
		return nil, ErrNoSuchOrder
	}
	if order.Status != api.ACTIVE {
		return order, ErrOrderNotActive
	}

	now := time.Now()
	durationSeconds := int(now.Sub(order.StartTime).Seconds())
	if durationSeconds < 0 {
		durationSeconds = 0
	}

	// Get incomplete ride threshold from configs
	incompleteRideThreshold := 5 // default value
	if s.configsProvider != nil {
		if cfg, err := s.configsProvider.GetConfigs(ctx); err == nil && cfg != nil {
			incompleteRideThreshold = cfg.IncompleteRideThresholdSeconds
		}
	}

	ppm := 0
	if order.PricePerMinute != nil {
		ppm = *order.PricePerMinute
	}
	unlock := 0
	if order.PriceUnlock != nil {
		unlock = *order.PriceUnlock
	}

	// Calculate total: if ride duration is less than threshold, charge is 0
	total := 0
	if durationSeconds >= incompleteRideThreshold {
		minutes := int(math.Ceil(float64(durationSeconds) / 60.0))
		usage := minutes * ppm
		total = unlock + usage
	}

	chargeErr := s.paymentsClient.ChargeMoneyForOrder(ctx, orderID, total)
	chargeSuccess := chargeErr == nil

	unholdSuccess := false
	if chargeSuccess {
		if err := s.paymentsClient.UnholdMoneyForOrder(ctx, orderID); err == nil {
			unholdSuccess = true
		}
	}

	finalStatus := api.FINISHED
	if !chargeSuccess {
		finalStatus = api.PAYMENTFAILED
	}

	if err := s.orderRepo.FinishOrder(ctx, orderID, now, durationSeconds, total, finalStatus, chargeSuccess, unholdSuccess, ""); err != nil {
		return nil, fmt.Errorf("failed to finish order: %w", err)
	}

	if s.cache != nil {
		_ = s.cache.Invalidate(ctx, orderID)
	}

	updated, err := s.orderRepo.GetOrderByID(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("failed to reload order: %w", err)
	}
	if s.cache != nil && updated != nil {
		_ = s.cache.SetOrder(ctx, updated, 30*time.Minute)
	}
	return updated, nil
}
