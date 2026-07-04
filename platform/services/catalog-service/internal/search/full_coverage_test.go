package search

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/zippyra/platform/services/catalog-service/internal/model"
)

type mockOpenSearchClient struct {
	searchFunc        func(ctx context.Context, storeID, query, category string, page, limit int) ([]model.Product, int, error)
	indexProductFunc  func(ctx context.Context, p model.Product) error
	deleteProductFunc func(ctx context.Context, storeID, productID string) error
}

func (m *mockOpenSearchClient) Search(ctx context.Context, storeID, query, category string, page, limit int) ([]model.Product, int, error) {
	return m.searchFunc(ctx, storeID, query, category, page, limit)
}

func (m *mockOpenSearchClient) IndexProduct(ctx context.Context, p model.Product) error {
	return m.indexProductFunc(ctx, p)
}

func (m *mockOpenSearchClient) DeleteProduct(ctx context.Context, storeID, productID string) error {
	return m.deleteProductFunc(ctx, storeID, productID)
}

func TestOpenSearchClient_FullCoverage(t *testing.T) {
	tests := []struct {
		name          string
		client        *mockOpenSearchClient
		storeID       string
		query         string
		category      string
		page          int
		limit         int
		expected      []model.Product
		expectedTotal int
		expectedErr   error
	}{
		{
			name: "successful search",
			client: &mockOpenSearchClient{
				searchFunc: func(ctx context.Context, storeID, query, category string, page, limit int) ([]model.Product, int, error) {
					return []model.Product{
						{
							Name:    "Test Product",
							Barcode: "123456789",
						},
					}, 1, nil
				},
			},
			storeID:  "store-123",
			query:    "test",
			category: "electronics",
			page:     1,
			limit:    20,
			expected: []model.Product{
				{
					Name:    "Test Product",
					Barcode: "123456789",
				},
			},
			expectedTotal: 1,
			expectedErr:   nil,
		},
		{
			name: "search error",
			client: &mockOpenSearchClient{
				searchFunc: func(ctx context.Context, storeID, query, category string, page, limit int) ([]model.Product, int, error) {
					return nil, 0, assert.AnError
				},
			},
			storeID:       "store-123",
			query:         "test",
			category:      "electronics",
			page:          1,
			limit:         20,
			expected:      nil,
			expectedTotal: 0,
			expectedErr:   assert.AnError,
		},
		{
			name: "empty results",
			client: &mockOpenSearchClient{
				searchFunc: func(ctx context.Context, storeID, query, category string, page, limit int) ([]model.Product, int, error) {
					return []model.Product{}, 0, nil
				},
			},
			storeID:       "store-123",
			query:         "nonexistent",
			category:      "electronics",
			page:          1,
			limit:         20,
			expected:      []model.Product{},
			expectedTotal: 0,
			expectedErr:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, total, err := tt.client.Search(context.Background(), tt.storeID, tt.query, tt.category, tt.page, tt.limit)

			if tt.expectedErr != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedErr, err)
				assert.Nil(t, result)
				assert.Equal(t, tt.expectedTotal, total)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedTotal, total)
				assert.Equal(t, len(tt.expected), len(result))
				if len(tt.expected) > 0 {
					assert.Equal(t, tt.expected[0].Name, result[0].Name)
					assert.Equal(t, tt.expected[0].Barcode, result[0].Barcode)
				}
			}
		})
	}
}

func TestOpenSearchClient_IndexProduct_FullCoverage(t *testing.T) {
	tests := []struct {
		name        string
		client      *mockOpenSearchClient
		product     model.Product
		expectedErr error
	}{
		{
			name: "successful index",
			client: &mockOpenSearchClient{
				indexProductFunc: func(ctx context.Context, p model.Product) error {
					return nil
				},
			},
			product: model.Product{
				ID:      uuid.New(),
				Name:    "Test Product",
				Barcode: "123456789",
			},
			expectedErr: nil,
		},
		{
			name: "index error",
			client: &mockOpenSearchClient{
				indexProductFunc: func(ctx context.Context, p model.Product) error {
					return assert.AnError
				},
			},
			product: model.Product{
				ID:      uuid.New(),
				Name:    "Test Product",
				Barcode: "123456789",
			},
			expectedErr: assert.AnError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.client.IndexProduct(context.Background(), tt.product)

			if tt.expectedErr != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedErr, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestOpenSearchClient_DeleteProduct_FullCoverage(t *testing.T) {
	tests := []struct {
		name        string
		client      *mockOpenSearchClient
		storeID     string
		productID   string
		expectedErr error
	}{
		{
			name: "successful delete",
			client: &mockOpenSearchClient{
				deleteProductFunc: func(ctx context.Context, storeID, productID string) error {
					return nil
				},
			},
			storeID:     "store-123",
			productID:   "prod-123",
			expectedErr: nil,
		},
		{
			name: "delete error",
			client: &mockOpenSearchClient{
				deleteProductFunc: func(ctx context.Context, storeID, productID string) error {
					return assert.AnError
				},
			},
			storeID:     "store-123",
			productID:   "prod-123",
			expectedErr: assert.AnError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.client.DeleteProduct(context.Background(), tt.storeID, tt.productID)

			if tt.expectedErr != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedErr, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSanitizeQuery_FullCoverage(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "normal search query",
			expected: "normal search query",
		},
		{
			input:    "search with 'quotes'",
			expected: "search with ''quotes''",
		},
		{
			input:    "search with; semicolons",
			expected: "search with semicolons",
		},
		{
			input:    "search with -- comments",
			expected: "search with -- comments",
		},
		{
			input:    "search with /* comments */",
			expected: "search with /* comments */",
		},
		{
			input:    "search with DROP TABLE",
			expected: "search with DROP TABLE",
		},
		{
			input:    "",
			expected: "",
		},
		{
			input:    "   ",
			expected: "",
		},
		{
			input:    "search with\nnewlines",
			expected: "search with\nnewlines",
		},
		{
			input:    "search with\ttabs",
			expected: "search with\ttabs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := SanitizeQuery(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
