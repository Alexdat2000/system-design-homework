package pricing

import "math"

// Inputs represents all parameters required to compute pricing according to ADR.
type Inputs struct {
	// Base tariff values from zone service
	ZonePricePerMinute int
	ZonePriceUnlock    int
	ZoneDefaultDeposit int

	// Dynamic configs
	Surge                     float64
	LowChargeDiscount         float64
	LowChargeThresholdPercent int

	// Runtime context
	ScooterChargePercent int
	HasSubscription      bool
	Trusted              bool
}

// Output contains computed pricing values to be placed into an offer.
type Output struct {
	PricePerMinute int
	PriceUnlock    int
	Deposit        int
}

// Calculate computes price per minute, unlock price and deposit based on:
// - zone tariff (base),
// - surge multiplier,
// - low charge discount below threshold,
// - subscription (free unlock),
// - trust (no deposit).
func Calculate(in Inputs) Output {
	// price per minute: base * surge, then optional low-battery discount
	ppm := float64(in.ZonePricePerMinute) * safeFloat(in.Surge, 1.0)
	if in.ScooterChargePercent >= 0 &&
		in.LowChargeThresholdPercent > 0 &&
		in.ScooterChargePercent < in.LowChargeThresholdPercent {
		ppm *= safeFloat(in.LowChargeDiscount, 1.0)
	}
	pricePerMinute := int(math.Round(ppm))
	if pricePerMinute < 0 {
		pricePerMinute = 0
	}

	// unlock price: 0 if subscription
	priceUnlock := in.ZonePriceUnlock
	if in.HasSubscription {
		priceUnlock = 0
	}
	if priceUnlock < 0 {
		priceUnlock = 0
	}

	// deposit: 0 if trusted
	deposit := in.ZoneDefaultDeposit
	if in.Trusted {
		deposit = 0
	}
	if deposit < 0 {
		deposit = 0
	}

	return Output{
		PricePerMinute: pricePerMinute,
		PriceUnlock:    priceUnlock,
		Deposit:        deposit,
	}
}

func safeFloat(v float64, def float64) float64 {
	if v <= 0 {
		return def
	}
	return v
}
