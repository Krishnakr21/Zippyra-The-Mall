package service

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
)

type mockRow struct {
	rate float64
	err  error
}

func (m *mockRow) Scan(dest ...any) error {
	if m.err != nil {
		return m.err
	}
	*(dest[0].(*float64)) = m.rate
	return nil
}

type mockDB struct {
	rate float64
	err  error
}

func (m *mockDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return &mockRow{rate: m.rate, err: m.err}
}

func TestCalculateGST(t *testing.T) {
	tests := []struct {
		name          string
		price         float64
		gstRate       float64
		storeGSTIN    string
		customerGSTIN string
		wantSupply    string
		wantTotal     float64
	}{
		{
			name:          "Intrastate B2C",
			price:         100.0,
			gstRate:       18.0,
			storeGSTIN:    "27AAAAA",
			customerGSTIN: "",
			wantSupply:    "intrastate",
			wantTotal:     18.0,
		},
		{
			name:          "Interstate B2B",
			price:         100.0,
			gstRate:       18.0,
			storeGSTIN:    "27AAAAA",
			customerGSTIN: "29BBBBB",
			wantSupply:    "interstate",
			wantTotal:     18.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateGST(tt.price, tt.gstRate, tt.storeGSTIN, tt.customerGSTIN)
			if got.SupplyType != tt.wantSupply {
				t.Errorf("got %v, want %v", got.SupplyType, tt.wantSupply)
			}
			if got.TotalGST != tt.wantTotal {
				t.Errorf("got %v, want %v", got.TotalGST, tt.wantTotal)
			}
		})
	}
}

func TestGetRateForHSN(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		db := &mockDB{rate: 18.0}
		rate, err := GetRateForHSN(context.Background(), db, "1234")
		if err != nil {
			t.Fatal(err)
		}
		if rate != 18.0 {
			t.Errorf("got %v, want 18.0", rate)
		}
	})

	t.Run("Error", func(t *testing.T) {
		db := &mockDB{err: errors.New("db error")}
		_, err := GetRateForHSN(context.Background(), db, "1234")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}
