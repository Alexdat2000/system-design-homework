package pricing

import "math"

type Inputs struct {
	ZonePricePerMinute int
	ZonePriceUnlock    int
	ZoneDefaultDeposit int

	Surge                     float64
	LowChargeDiscount         float64
	LowChargeThresholdPercent int

	ScooterChargePercent int
	HasSubscription      bool
	Trusted              bool
}

type Output struct {
	PricePerMinute int
	PriceUnlock    int
	Deposit        int
}

func Calculate(in Inputs) Output {
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

	priceUnlock := in.ZonePriceUnlock
	if in.HasSubscription {
		priceUnlock = 0
	}
	if priceUnlock < 0 {
		priceUnlock = 0
	}

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
