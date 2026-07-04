package service

import (
	"context"

	"github.com/jackc/pgx/v5"
)

type DBQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type GSTBreakdown struct {
	GSTRate    float64
	CGSTRate   float64
	SGSTRate   float64
	IGSTRate   float64
	CGSTAmount float64
	SGSTAmount float64
	IGSTAmount float64
	TotalGST   float64
	SupplyType string // "intrastate" or "interstate"
}

// Calculate determines correct GST breakdown.
// storeGSTIN: store's GSTIN (first 2 chars = state code)
// customerGSTIN: customer's GSTIN (empty for B2C)
// For B2C: always intrastate (CGST + SGST)
// For B2B: compare state codes, if same = intrastate, if different = interstate (IGST)
func CalculateGST(price float64, gstRate float64, storeGSTIN, customerGSTIN string) GSTBreakdown {
	var breakdown GSTBreakdown
	breakdown.GSTRate = gstRate
	totalGST := (price * gstRate) / 100
	breakdown.TotalGST = totalGST

	isB2B := customerGSTIN != ""
	isIntrastate := true

	if isB2B {
		if len(storeGSTIN) >= 2 && len(customerGSTIN) >= 2 {
			if storeGSTIN[:2] != customerGSTIN[:2] {
				isIntrastate = false
			}
		}
	}
	// For B2C, it's always intrastate per requirements.

	if isIntrastate {
		breakdown.SupplyType = "intrastate"
		breakdown.CGSTRate = gstRate / 2
		breakdown.SGSTRate = gstRate / 2
		breakdown.CGSTAmount = totalGST / 2
		breakdown.SGSTAmount = totalGST / 2
	} else {
		breakdown.SupplyType = "interstate"
		breakdown.IGSTRate = gstRate
		breakdown.IGSTAmount = totalGST
	}

	return breakdown
}

// GetRateForHSN fetches GST rate from hsn_gst_rates table
func GetRateForHSN(ctx context.Context, db DBQuerier, hsnCode string) (float64, error) {
	var rate float64
	err := db.QueryRow(ctx, "SELECT gst_rate FROM hsn_gst_rates WHERE hsn_code = $1", hsnCode).Scan(&rate)
	if err != nil {
		return 0, err
	}
	return rate, nil
}
