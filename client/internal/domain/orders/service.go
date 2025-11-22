package orders

import (
	"client/api"
	"client/internal/domain/offers"
	"client/internal/external"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var (
	// ErrOfferNotFound is returned when offer doesn't exist
	ErrOfferNotFound = errors.New("offer not found")
	// ErrOfferExpired is returned when offer has expired
	ErrOfferExpired = errors.New("offer expired")
	// ErrOfferAlreadyUsed is returned when offer was already used
	ErrOfferAlreadyUsed = errors.New("offer already used")
	// ErrInvalidUser is returned when user_id doesn't match offer's user_id
	ErrInvalidUser = errors.New("user_id doesn't match offer")
	// ErrOrderAlreadyExists is returned when order with same offer_id already exists (idempotency)
	ErrOrderAlreadyExists = errors.New("order already exists for this offer")
)

// ServiceInterface defines the interface for order service operations
type ServiceInterface interface {
	CreateOrder(ctx context.Context, req *CreateOrderRequest) (*api.Order, error)
}

// Service handles business logic for orders
type Service struct {
	orderRepo      Repository
	offerRepo      offers.Repository
	paymentsClient external.PaymentsClientInterface
}

// NewService creates a new orders service
func NewService(orderRepo Repository, offerRepo offers.Repository, paymentsClient external.PaymentsClientInterface) *Service {
	return &Service{
		orderRepo:      orderRepo,
		offerRepo:      offerRepo,
		paymentsClient: paymentsClient,
	}
}

// CreateOrderRequest represents the input for creating an order
type CreateOrderRequest struct {
	OfferID string
	UserID  string
}

// CreateOrder creates a new order from an offer
// According to ADR:
// 1. Validate offer (not expired, not used)
// 2. If valid, hold deposit on user's card
// 3. Create order
// 4. Ensure idempotency (if order already exists for this offer, return existing)
func (s *Service) CreateOrder(ctx context.Context, req *CreateOrderRequest) (*api.Order, error) {
	// Step 1: Check idempotency - if order already exists for this offer, return it
	existingOrder, err := s.orderRepo.GetOrderByOfferID(ctx, req.OfferID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing order: %w", err)
	}
	if existingOrder != nil {
		// Order already exists for this offer - return it (idempotency)
		return existingOrder, nil
	}

	// Step 2: Get and validate offer
	offer, err := s.offerRepo.GetOffer(ctx, req.OfferID)
	if err != nil {
		return nil, fmt.Errorf("failed to get offer: %w", err)
	}
	if offer == nil {
		return nil, ErrOfferNotFound
	}

	// Step 3: Validate offer is not expired
	if time.Now().After(offer.ExpiresAt) {
		return nil, ErrOfferExpired
	}

	// Step 4: Validate user_id matches
	if offer.UserId != req.UserID {
		return nil, ErrInvalidUser
	}

	// Step 5: Try to mark offer as used (atomic operation)
	marked, err := s.offerRepo.MarkOfferAsUsed(ctx, req.OfferID)
	if err != nil {
		return nil, fmt.Errorf("failed to mark offer as used: %w", err)
	}
	if !marked {
		// Offer was already used by another request
		return nil, ErrOfferAlreadyUsed
	}

	// Step 6: Generate order ID (UUID)
	orderID := uuid.New().String()

	// Step 7: Hold deposit on user's card
	holdReq := &external.PaymentHoldRequest{
		UserID:  req.UserID,
		OrderID: orderID,
		Amount:  offer.Deposit,
	}
	holdResp, err := s.paymentsClient.HoldMoneyForOrder(ctx, holdReq)
	if err != nil {
		// If payment hold fails, we should not create the order
		// Note: In production, we might want to retry or handle this differently
		return nil, fmt.Errorf("failed to hold deposit: %w", err)
	}
	if !holdResp.Success {
		return nil, fmt.Errorf("payment hold failed")
	}

	// Step 8: Create order
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
		CurrentAmount:  &offer.PriceUnlock, // Initial amount includes unlock price
	}

	// Step 9: Save order to database with payment transaction
	// According to ADR, this should be in a transaction with payment transaction record
	if err := s.orderRepo.CreateOrder(ctx, order, holdResp.TransactionID); err != nil {
		// If order creation fails, we should unhold the deposit
		// In production, this should be handled in a transaction
		_ = s.paymentsClient.UnholdMoneyForOrder(ctx, orderID)
		return nil, fmt.Errorf("failed to create order: %w", err)
	}

	return order, nil
}
