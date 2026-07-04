package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/zippyra/platform/services/catalog-service/internal/model"
)

func TestProductRepo_GetByStoreAndBarcode(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewProductRepo(mock)
	ctx := context.Background()
	storeID := uuid.New()
	barcode := "123"
	fullCols := []string{"id", "store_id", "barcode", "name", "description", "brand", "category",
		"hsn_code", "mrp", "selling_price", "gst_rate", "unit", "image_url",
		"thumbnail_url", "is_active", "is_returnable", "stock_quantity",
		"reorder_point", "sync_seq", "created_at", "updated_at"}

	t.Run("Success", func(t *testing.T) {
		rows := pgxmock.NewRows(fullCols).AddRow(uuid.New(), storeID, barcode, "P", nil, nil, nil, nil, 0.0, 0.0, 0.0, nil, nil, nil, true, false, 0, 0, int64(1), nil, nil)
		mock.ExpectQuery("SELECT (.+) FROM products").WithArgs(storeID, barcode).WillReturnRows(rows)
		res, err := repo.GetByStoreAndBarcode(ctx, storeID, barcode)
		assert.NoError(t, err)
		assert.NotNil(t, res)
	})

	t.Run("Query Error", func(t *testing.T) {
		mock.ExpectQuery("SELECT (.+) FROM products").WithArgs(storeID, barcode).WillReturnError(assert.AnError)
		_, err := repo.GetByStoreAndBarcode(ctx, storeID, barcode)
		assert.Error(t, err)
	})

	t.Run("Scan Error", func(t *testing.T) {
		rows := pgxmock.NewRows(fullCols).AddRow("invalid", storeID, barcode, "P", nil, nil, nil, nil, 0.0, 0.0, 0.0, nil, nil, nil, true, false, 0, 0, int64(1), nil, nil)
		mock.ExpectQuery("SELECT (.+) FROM products").WithArgs(storeID, barcode).WillReturnRows(rows)
		_, err := repo.GetByStoreAndBarcode(ctx, storeID, barcode)
		assert.Error(t, err)
	})
}

func TestProductRepo_Search(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewProductRepo(mock)
	ctx := context.Background()
	storeID := uuid.New()
	query := "test"
	fullCols := []string{"id", "store_id", "barcode", "name", "description", "brand", "category",
		"hsn_code", "mrp", "selling_price", "gst_rate", "unit", "image_url",
		"thumbnail_url", "is_active", "is_returnable", "stock_quantity",
		"reorder_point", "sync_seq", "created_at", "updated_at"}

	t.Run("Success", func(t *testing.T) {
		mock.ExpectQuery("SELECT COUNT").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(1))
		mock.ExpectQuery("SELECT (.+) FROM products").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnRows(pgxmock.NewRows(fullCols).AddRow(uuid.New(), storeID, "1", "P", nil, nil, nil, nil, 0.0, 0.0, 0.0, nil, nil, nil, true, false, 0, 0, int64(1), nil, nil))
		_, _, err := repo.Search(ctx, storeID, query, nil, 1, 20)
		assert.NoError(t, err)
	})

	t.Run("Count Error", func(t *testing.T) {
		mock.ExpectQuery("SELECT COUNT").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnError(assert.AnError)
		_, _, err := repo.Search(ctx, storeID, query, nil, 1, 20)
		assert.Error(t, err)
	})

	t.Run("Query Error", func(t *testing.T) {
		mock.ExpectQuery("SELECT COUNT").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(1))
		mock.ExpectQuery("SELECT (.+) FROM products").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnError(assert.AnError)
		_, _, err := repo.Search(ctx, storeID, query, nil, 1, 20)
		assert.Error(t, err)
	})

	t.Run("Scan Error", func(t *testing.T) {
		mock.ExpectQuery("SELECT COUNT").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(1))
		mock.ExpectQuery("SELECT (.+) FROM products").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnRows(pgxmock.NewRows(fullCols).AddRow("invalid", storeID, "1", "P", nil, nil, nil, nil, 0.0, 0.0, 0.0, nil, nil, nil, true, false, 0, 0, int64(1), nil, nil))
		_, _, err := repo.Search(ctx, storeID, query, nil, 1, 20)
		assert.Error(t, err)
	})

	t.Run("Rows Err", func(t *testing.T) {
		mock.ExpectQuery("SELECT COUNT").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(1))
		mock.ExpectQuery("SELECT (.+) FROM products").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnRows(pgxmock.NewRows(fullCols).
			AddRow(uuid.New(), storeID, "1", "P", nil, nil, nil, nil, 0.0, 0.0, 0.0, nil, nil, nil, true, false, 0, 0, int64(1), nil, nil).
			RowError(1, assert.AnError))
		_, _, err := repo.Search(ctx, storeID, query, nil, 1, 20)
		assert.Error(t, err)
	})

	t.Run("With Category", func(t *testing.T) {
		category := strPtr("electronics")
		mock.ExpectQuery("SELECT COUNT").WithArgs(storeID, "%"+query+"%").WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(1))
		mock.ExpectQuery("SELECT (.+) FROM products").WithArgs(storeID, *category, "%"+query+"%", 20, 0).WillReturnRows(pgxmock.NewRows(fullCols).AddRow(uuid.New(), storeID, "1", "P", nil, nil, "electronics", nil, 0.0, 0.0, 0.0, nil, nil, nil, true, false, 0, 0, int64(1), nil, nil))
		products, total, err := repo.Search(ctx, storeID, query, category, 1, 20)
		assert.NoError(t, err)
		assert.Len(t, products, 1)
		assert.Equal(t, 1, total)
	})
}

func TestProductRepo_GetByCategory(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewProductRepo(mock)
	ctx := context.Background()
	storeID := uuid.New()
	category := "c"
	fullCols := []string{"id", "store_id", "barcode", "name", "description", "brand", "category",
		"hsn_code", "mrp", "selling_price", "gst_rate", "unit", "image_url",
		"thumbnail_url", "is_active", "is_returnable", "stock_quantity",
		"reorder_point", "sync_seq", "created_at", "updated_at"}

	t.Run("Success", func(t *testing.T) {
		mock.ExpectQuery("SELECT COUNT").WithArgs(storeID, category).WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(1))
		mock.ExpectQuery("SELECT (.+) FROM products").WithArgs(storeID, category, 20, 0).WillReturnRows(pgxmock.NewRows(fullCols).AddRow(uuid.New(), storeID, "1", "P", nil, nil, category, nil, 0.0, 0.0, 0.0, nil, nil, nil, true, false, 0, 0, int64(1), nil, nil))
		products, total, err := repo.GetByCategory(ctx, storeID, category, 1, 20)
		assert.NoError(t, err)
		assert.Len(t, products, 1)
		assert.Equal(t, 1, total)
	})

	t.Run("Count Error", func(t *testing.T) {
		mock.ExpectQuery("SELECT COUNT").WithArgs(storeID, category).WillReturnError(assert.AnError)
		_, _, err := repo.GetByCategory(ctx, storeID, category, 1, 20)
		assert.Error(t, err)
	})

	t.Run("Query Error", func(t *testing.T) {
		mock.ExpectQuery("SELECT COUNT").WithArgs(storeID, category).WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(1))
		mock.ExpectQuery("SELECT (.+) FROM products").WithArgs(storeID, category, 20, 0).WillReturnError(assert.AnError)
		_, _, err := repo.GetByCategory(ctx, storeID, category, 1, 20)
		assert.Error(t, err)
	})

	t.Run("Scan Error", func(t *testing.T) {
		mock.ExpectQuery("SELECT COUNT").WithArgs(storeID, category).WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(1))
		mock.ExpectQuery("SELECT (.+) FROM products").WithArgs(storeID, category, 20, 0).WillReturnRows(pgxmock.NewRows(fullCols).AddRow("invalid", storeID, "1", "P", nil, nil, category, nil, 0.0, 0.0, 0.0, nil, nil, nil, true, false, 0, 0, int64(1), nil, nil))
		_, _, err := repo.GetByCategory(ctx, storeID, category, 1, 20)
		assert.Error(t, err)
	})

	t.Run("Rows Err", func(t *testing.T) {
		mock.ExpectQuery("SELECT COUNT").WithArgs(storeID, category).WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(1))
		mock.ExpectQuery("SELECT (.+) FROM products").WithArgs(storeID, category, 20, 0).WillReturnRows(pgxmock.NewRows(fullCols).
			AddRow(uuid.New(), storeID, "1", "P", nil, nil, category, nil, 0.0, 0.0, 0.0, nil, nil, nil, true, false, 0, 0, int64(1), nil, nil).
			RowError(1, assert.AnError))
		_, _, err := repo.GetByCategory(ctx, storeID, category, 1, 20)
		assert.Error(t, err)
	})
}

func TestProductRepo_Sync(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewProductRepo(mock)
	ctx := context.Background()
	storeID := uuid.New()
	fullCols := []string{"id", "store_id", "barcode", "name", "description", "brand", "category",
		"hsn_code", "mrp", "selling_price", "gst_rate", "unit", "image_url",
		"thumbnail_url", "is_active", "is_returnable", "stock_quantity",
		"reorder_point", "sync_seq", "created_at", "updated_at"}

	t.Run("Success", func(t *testing.T) {
		mock.ExpectQuery("SELECT (.+) FROM products").WithArgs(storeID, int64(0), 100).WillReturnRows(pgxmock.NewRows(fullCols).AddRow(uuid.New(), storeID, "1", "P", nil, nil, nil, nil, 0.0, 0.0, 0.0, nil, nil, nil, true, false, 0, 0, int64(1), nil, nil))
		products, err := repo.Sync(ctx, storeID, 0, 100)
		assert.NoError(t, err)
		assert.Len(t, products, 1)
	})

	t.Run("Query Error", func(t *testing.T) {
		mock.ExpectQuery("SELECT (.+) FROM products").WithArgs(storeID, int64(0), 100).WillReturnError(assert.AnError)
		_, err := repo.Sync(ctx, storeID, 0, 100)
		assert.Error(t, err)
	})

	t.Run("Scan Error", func(t *testing.T) {
		mock.ExpectQuery("SELECT (.+) FROM products").WithArgs(storeID, int64(0), 100).WillReturnRows(pgxmock.NewRows(fullCols).AddRow("invalid", storeID, "1", "P", nil, nil, nil, nil, 0.0, 0.0, 0.0, nil, nil, nil, true, false, 0, 0, int64(1), nil, nil))
		_, err := repo.Sync(ctx, storeID, 0, 100)
		assert.Error(t, err)
	})

	t.Run("Rows Err", func(t *testing.T) {
		mock.ExpectQuery("SELECT (.+) FROM products").WithArgs(storeID, int64(0), 100).WillReturnRows(pgxmock.NewRows(fullCols).
			AddRow(uuid.New(), storeID, "1", "P", nil, nil, nil, nil, 0.0, 0.0, 0.0, nil, nil, nil, true, false, 0, 0, int64(1), nil, nil).
			RowError(1, assert.AnError))
		_, err := repo.Sync(ctx, storeID, 0, 100)
		assert.Error(t, err)
	})
}

func TestProductRepo_Upsert(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewProductRepo(mock)
	ctx := context.Background()
	p := &model.Product{StoreID: uuid.New(), Barcode: "123"}

	t.Run("Success", func(t *testing.T) {
		mock.ExpectQuery("INSERT INTO products").
			WithArgs(
				pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
				pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
				pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
				pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			).
			WillReturnRows(pgxmock.NewRows([]string{"id", "sync_seq", "created_at", "updated_at"}).
				AddRow(uuid.New(), int64(1), nil, nil))
		err := repo.Upsert(ctx, p)
		assert.NoError(t, err)
	})

	t.Run("Error", func(t *testing.T) {
		mock.ExpectQuery("INSERT INTO products").
			WithArgs(
				pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
				pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
				pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
				pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(),
			).
			WillReturnError(assert.AnError)
		err := repo.Upsert(ctx, p)
		assert.Error(t, err)
	})
}

func TestProductRepo_GetByID(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewProductRepo(mock)
	ctx := context.Background()
	id := uuid.New()
	fullCols := []string{"id", "store_id", "barcode", "name", "description", "brand", "category",
		"hsn_code", "mrp", "selling_price", "gst_rate", "unit", "image_url",
		"thumbnail_url", "is_active", "is_returnable", "stock_quantity",
		"reorder_point", "sync_seq", "created_at", "updated_at"}

	t.Run("Success", func(t *testing.T) {
		rows := pgxmock.NewRows(fullCols).AddRow(id, uuid.New(), "1", "P", nil, nil, nil, nil, 0.0, 0.0, 0.0, nil, nil, nil, true, false, 0, 0, int64(1), nil, nil)
		mock.ExpectQuery("SELECT (.+) FROM products WHERE id =").WithArgs(id).WillReturnRows(rows)
		res, err := repo.GetByID(ctx, id)
		assert.NoError(t, err)
		assert.NotNil(t, res)
	})

	t.Run("Scan Error", func(t *testing.T) {
		rows := pgxmock.NewRows(fullCols).AddRow("invalid", uuid.New(), "1", "P", nil, nil, nil, nil, 0.0, 0.0, 0.0, nil, nil, nil, true, false, 0, 0, int64(1), nil, nil)
		mock.ExpectQuery("SELECT (.+) FROM products WHERE id =").WithArgs(id).WillReturnRows(rows)
		_, err := repo.GetByID(ctx, id)
		assert.Error(t, err)
	})
}
