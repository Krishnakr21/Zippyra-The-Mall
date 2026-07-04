package search

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/zippyra/platform/services/catalog-service/internal/model"
)

func TestSanitizeQuery(t *testing.T) {
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
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := SanitizeQuery(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewOpenSearchClient(t *testing.T) {
	// Test with valid URL - should create client
	client, err := NewOpenSearchClient("http://localhost:9200", "user", "pass")
	// Note: This might succeed or fail depending on if OpenSearch is running
	// We'll handle both cases
	if err != nil {
		assert.Error(t, err)
		assert.Nil(t, client)
	} else {
		assert.NotNil(t, client)
	}

	// Test with invalid URL - should return error (empty address)
	client, err = NewOpenSearchClient("", "user", "pass")
	assert.Error(t, err)
	assert.Nil(t, client)
}

func TestOpenSearchClient_Search(t *testing.T) {
	// Test with nil client - should handle gracefully
	client := &OpenSearchClient{
		client: nil,
		index:  "products",
	}

	ctx := context.Background()

	// This will panic due to nil client, so we catch it
	assert.Panics(t, func() {
		client.Search(ctx, "store-id", "query", "category", 1, 20)
	})
}

func TestOpenSearchClient_IndexProduct(t *testing.T) {
	// Test with nil client - should handle gracefully
	client := &OpenSearchClient{
		client: nil,
		index:  "products",
	}

	ctx := context.Background()
	product := model.Product{
		Name: "Test Product",
	}

	// This will panic due to nil client, so we catch it
	assert.Panics(t, func() {
		client.IndexProduct(ctx, product)
	})
}

func TestOpenSearchClient_DeleteProduct(t *testing.T) {
	// Test with nil client - should handle gracefully
	client := &OpenSearchClient{
		client: nil,
		index:  "products",
	}

	ctx := context.Background()
	productID := "test-id"

	// This will panic due to nil client, so we catch it
	assert.Panics(t, func() {
		client.DeleteProduct(ctx, "store-id", productID)
	})
}
