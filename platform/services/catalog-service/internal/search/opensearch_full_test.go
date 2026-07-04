package search

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/opensearch-project/opensearch-go/v2"
	"github.com/stretchr/testify/assert"
	"github.com/zippyra/platform/services/catalog-service/internal/model"
)

type mockTransport struct {
	roundTripFunc func(req *http.Request) (*http.Response, error)
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.roundTripFunc(req)
}

func TestOpenSearchClient_Search_Full(t *testing.T) {
	ctx := context.Background()
	storeID := uuid.New().String()
	query := "test"

	t.Run("Success", func(t *testing.T) {
		resp := map[string]interface{}{
			"hits": map[string]interface{}{
				"total": map[string]interface{}{"value": 1.0},
				"hits": []interface{}{
					map[string]interface{}{
						"_source": map[string]interface{}{
							"id":   uuid.New().String(),
							"name": "Product 1",
						},
					},
				},
			},
		}
		respJSON, _ := json.Marshal(resp)

		client, _ := opensearch.NewClient(opensearch.Config{
			Transport: &mockTransport{
				roundTripFunc: func(req *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: 200,
						Body:       io.NopCloser(bytes.NewBuffer(respJSON)),
						Header:     make(http.Header),
					}, nil
				},
			},
		})
		c := &OpenSearchClient{client: client, index: "test"}
		products, total, err := c.Search(ctx, storeID, query, "cat", 1, 10)
		assert.NoError(t, err)
		assert.Equal(t, 1, total)
		assert.Len(t, products, 1)
	})

	t.Run("Do Error", func(t *testing.T) {
		client, _ := opensearch.NewClient(opensearch.Config{
			Transport: &mockTransport{
				roundTripFunc: func(req *http.Request) (*http.Response, error) {
					return nil, assert.AnError
				},
			},
		})
		c := &OpenSearchClient{client: client, index: "test"}
		_, _, err := c.Search(ctx, storeID, query, "", 1, 10)
		assert.Error(t, err)
	})

	t.Run("Status Error", func(t *testing.T) {
		client, _ := opensearch.NewClient(opensearch.Config{
			Transport: &mockTransport{
				roundTripFunc: func(req *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: 500,
						Body:       io.NopCloser(bytes.NewBufferString(`{"error":"internal"}`)),
						Header:     make(http.Header),
					}, nil
				},
			},
		})
		c := &OpenSearchClient{client: client, index: "test"}
		_, _, err := c.Search(ctx, storeID, query, "", 1, 10)
		assert.Error(t, err)
	})

	t.Run("Decode Error", func(t *testing.T) {
		client, _ := opensearch.NewClient(opensearch.Config{
			Transport: &mockTransport{
				roundTripFunc: func(req *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: 200,
						Body:       io.NopCloser(bytes.NewBufferString(`invalid json`)),
						Header:     make(http.Header),
					}, nil
				},
			},
		})
		c := &OpenSearchClient{client: client, index: "test"}
		_, _, err := c.Search(ctx, storeID, query, "", 1, 10)
		assert.Error(t, err)
	})

	t.Run("Invalid Format Hits Missing", func(t *testing.T) {
		respJSON, _ := json.Marshal(map[string]interface{}{"foo": "bar"})
		client, _ := opensearch.NewClient(opensearch.Config{
			Transport: &mockTransport{
				roundTripFunc: func(req *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: 200,
						Body:       io.NopCloser(bytes.NewBuffer(respJSON)),
						Header:     make(http.Header),
					}, nil
				},
			},
		})
		c := &OpenSearchClient{client: client, index: "test"}
		_, _, err := c.Search(ctx, storeID, query, "", 1, 10)
		assert.Error(t, err)
	})

	t.Run("Invalid Total Format", func(t *testing.T) {
		resp := map[string]interface{}{
			"hits": map[string]interface{}{
				"total": "invalid",
				"hits":  []interface{}{},
			},
		}
		respJSON, _ := json.Marshal(resp)
		client, _ := opensearch.NewClient(opensearch.Config{
			Transport: &mockTransport{
				roundTripFunc: func(req *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: 200,
						Body:       io.NopCloser(bytes.NewBuffer(respJSON)),
						Header:     make(http.Header),
					}, nil
				},
			},
		})
		c := &OpenSearchClient{client: client, index: "test"}
		products, total, err := c.Search(ctx, storeID, query, "", 1, 10)
		assert.NoError(t, err)
		assert.Equal(t, 0, total)
		assert.Len(t, products, 0)
	})

	t.Run("Hits List Missing", func(t *testing.T) {
		resp := map[string]interface{}{
			"hits": map[string]interface{}{
				"total": map[string]interface{}{"value": 0.0},
			},
		}
		respJSON, _ := json.Marshal(resp)
		client, _ := opensearch.NewClient(opensearch.Config{
			Transport: &mockTransport{
				roundTripFunc: func(req *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: 200,
						Body:       io.NopCloser(bytes.NewBuffer(respJSON)),
						Header:     make(http.Header),
					}, nil
				},
			},
		})
		c := &OpenSearchClient{client: client, index: "test"}
		products, _, err := c.Search(ctx, storeID, query, "", 1, 10)
		assert.NoError(t, err)
		assert.Len(t, products, 0)
	})

	t.Run("Invalid Hit Format", func(t *testing.T) {
		resp := map[string]interface{}{
			"hits": map[string]interface{}{
				"hits": []interface{}{"not-a-map"},
			},
		}
		respJSON, _ := json.Marshal(resp)
		client, _ := opensearch.NewClient(opensearch.Config{
			Transport: &mockTransport{
				roundTripFunc: func(req *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: 200,
						Body:       io.NopCloser(bytes.NewBuffer(respJSON)),
						Header:     make(http.Header),
					}, nil
				},
			},
		})
		c := &OpenSearchClient{client: client, index: "test"}
		products, _, err := c.Search(ctx, storeID, query, "", 1, 10)
		assert.NoError(t, err)
		assert.Len(t, products, 0)
	})

	t.Run("Source Missing", func(t *testing.T) {
		resp := map[string]interface{}{
			"hits": map[string]interface{}{
				"hits": []interface{}{map[string]interface{}{"foo": "bar"}},
			},
		}
		respJSON, _ := json.Marshal(resp)
		client, _ := opensearch.NewClient(opensearch.Config{
			Transport: &mockTransport{
				roundTripFunc: func(req *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: 200,
						Body:       io.NopCloser(bytes.NewBuffer(respJSON)),
						Header:     make(http.Header),
					}, nil
				},
			},
		})
		c := &OpenSearchClient{client: client, index: "test"}
		products, _, err := c.Search(ctx, storeID, query, "", 1, 10)
		assert.NoError(t, err)
		assert.Len(t, products, 0)
	})

	t.Run("Unmarshal Error", func(t *testing.T) {
		resp := map[string]interface{}{
			"hits": map[string]interface{}{
				"hits": []interface{}{
					map[string]interface{}{
						"_source": map[string]interface{}{
							"id": 123, // Should be string (UUID)
						},
					},
				},
			},
		}
		respJSON, _ := json.Marshal(resp)
		client, _ := opensearch.NewClient(opensearch.Config{
			Transport: &mockTransport{
				roundTripFunc: func(req *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: 200,
						Body:       io.NopCloser(bytes.NewBuffer(respJSON)),
						Header:     make(http.Header),
					}, nil
				},
			},
		})
		c := &OpenSearchClient{client: client, index: "test"}
		_, _, err := c.Search(ctx, storeID, query, "", 1, 10)
		assert.Error(t, err)
	})
}

func TestOpenSearchClient_IndexProduct_EncodeError(t *testing.T) {
	client, _ := opensearch.NewClient(opensearch.Config{})
	c := &OpenSearchClient{client: client, index: "test"}
	// Trigger JSON encoding error by using math.NaN()
	p := model.Product{
		MRP: math.NaN(),
	}
	err := c.IndexProduct(context.Background(), p)
	assert.Error(t, err)
}

func TestOpenSearchClient_IndexProduct_DoError(t *testing.T) {
	client, _ := opensearch.NewClient(opensearch.Config{
		Transport: &mockTransport{
			roundTripFunc: func(req *http.Request) (*http.Response, error) {
				return nil, assert.AnError
			},
		},
	})
	c := &OpenSearchClient{client: client, index: "test"}
	err := c.IndexProduct(context.Background(), model.Product{})
	assert.Error(t, err)
}

func TestOpenSearchClient_IndexProduct_StatusError(t *testing.T) {
	client, _ := opensearch.NewClient(opensearch.Config{
		Transport: &mockTransport{
			roundTripFunc: func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: 500,
					Body:       io.NopCloser(bytes.NewBufferString(`{}`)),
					Header:     make(http.Header),
				}, nil
			},
		},
	})
	c := &OpenSearchClient{client: client, index: "test"}
	err := c.IndexProduct(context.Background(), model.Product{})
	assert.Error(t, err)
}
