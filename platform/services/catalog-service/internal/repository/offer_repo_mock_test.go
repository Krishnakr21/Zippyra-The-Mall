package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
)

func TestOfferRepo_GetActiveByStore(t *testing.T) {
	mock, err := pgxmock.NewPool()
	assert.NoError(t, err)
	defer mock.Close()

	repo := NewOfferRepo(mock)
	ctx := context.Background()
	storeID := uuid.New()

	fullCols := []string{"id", "store_id", "name", "description", "type", "value", "min_amount", "max_discount",
		"category", "product_ids", "priority", "is_active", "valid_from", "valid_until",
		"created_at", "updated_at"}

	t.Run("Success", func(t *testing.T) {
		productIDs := []string{uuid.New().String(), "invalid-uuid"}
		rows := pgxmock.NewRows(fullCols).
			AddRow(uuid.New(), storeID, "Offer", strPtr("Desc"), "discount", 10.0, floatPtr(0.0), floatPtr(0.0),
				strPtr("Cat"), productIDs, 0, true, time.Now(), timePtr(time.Now().Add(time.Hour)), time.Now(), time.Now())
		mock.ExpectQuery("SELECT (.+) FROM offer_rules").WithArgs(storeID).WillReturnRows(rows)
		res, err := repo.GetActiveByStore(ctx, storeID)
		assert.NoError(t, err)
		assert.Len(t, res, 1)
		assert.Len(t, res[0].ProductIDs, 1) // Only 1 valid UUID
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("Errors", func(t *testing.T) {
		// Query Error
		mock.ExpectQuery("SELECT (.+) FROM offer_rules").WithArgs(storeID).WillReturnError(assert.AnError)
		repo.GetActiveByStore(ctx, storeID)

		// Scan Error
		mock.ExpectQuery("SELECT (.+) FROM offer_rules").WithArgs(storeID).WillReturnRows(pgxmock.NewRows(fullCols).AddRow("invalid", nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil))
		_, err := repo.GetActiveByStore(ctx, storeID)
		assert.Error(t, err)

		// Rows Err
		mock.ExpectQuery("SELECT (.+) FROM offer_rules").WithArgs(storeID).WillReturnRows(pgxmock.NewRows(fullCols).
			AddRow(uuid.New(), storeID, "Offer", strPtr("Desc"), "discount", 10.0, floatPtr(0.0), floatPtr(0.0),
				strPtr("Cat"), []string{}, 0, true, time.Now(), timePtr(time.Now().Add(time.Hour)), time.Now(), time.Now()).
			RowError(1, assert.AnError))
		_, err = repo.GetActiveByStore(ctx, storeID)
		assert.Error(t, err)

		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
