package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/opensearch-project/opensearch-go/v2"
	"github.com/opensearch-project/opensearch-go/v2/opensearchapi"
	"github.com/rs/zerolog/log"

	"github.com/zippyra/platform/services/catalog-service/internal/model"
)

type SearchClient interface {
	Search(ctx context.Context, storeID, query, category string, page, limit int) ([]model.Product, int, error)
	IndexProduct(ctx context.Context, p model.Product) error
}

type OpenSearchClient struct {
	client *opensearch.Client
	index  string
}

func NewOpenSearchClient(addr, username, password string) (SearchClient, error) {
	if addr == "" {
		return nil, fmt.Errorf("address is required")
	}

	cfg := opensearch.Config{
		Addresses: []string{addr},
		Username:  username,
		Password:  password,
	}

	client, err := opensearch.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create opensearch client: %w", err)
	}

	return &OpenSearchClient{
		client: client,
		index:  "products",
	}, nil
}

func (c *OpenSearchClient) Search(ctx context.Context, storeID, query, category string, page, limit int) ([]model.Product, int, error) {
	ctx, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
	defer cancel()

	offset := (page - 1) * limit

	searchQuery := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{
						"match": map[string]interface{}{
							"name": query,
						},
					},
				},
				"filter": []map[string]interface{}{
					{
						"term": map[string]interface{}{
							"store_id": storeID,
						},
					},
					{
						"term": map[string]interface{}{
							"is_active": true,
						},
					},
				},
			},
		},
		"from": offset,
		"size": limit,
	}

	if category != "" {
		searchQuery["query"].(map[string]interface{})["bool"].(map[string]interface{})["filter"] = append(
			searchQuery["query"].(map[string]interface{})["bool"].(map[string]interface{})["filter"].([]map[string]interface{}),
			map[string]interface{}{
				"term": map[string]interface{}{
					"category": category,
				},
			},
		)
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(searchQuery); err != nil {
		return nil, 0, fmt.Errorf("failed to encode search query: %w", err)
	}

	req := opensearchapi.SearchRequest{
		Index: []string{c.index},
		Body:  &buf,
	}

	res, err := req.Do(ctx, c.client)
	if err != nil {
		log.Warn().Err(err).Str("storeID", storeID).Str("query", query).Msg("opensearch search failed, will fall back to database")
		return nil, 0, fmt.Errorf("opensearch search failed: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		log.Warn().Str("status", res.Status()).Str("storeID", storeID).Str("query", query).Msg("opensearch search returned error, will fall back to database")
		return nil, 0, fmt.Errorf("opensearch search error: %s", res.Status())
	}

	var searchResp map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&searchResp); err != nil {
		return nil, 0, fmt.Errorf("failed to decode search response: %w", err)
	}

	hits, ok := searchResp["hits"].(map[string]interface{})
	if !ok {
		return nil, 0, fmt.Errorf("invalid search response format")
	}

	total, ok := hits["total"].(map[string]interface{})["value"].(float64)
	if !ok {
		total = 0
	}

	hitsList, ok := hits["hits"].([]interface{})
	if !ok {
		return []model.Product{}, int(total), nil
	}

	var products []model.Product
	for _, hit := range hitsList {
		hitMap, ok := hit.(map[string]interface{})
		if !ok {
			continue
		}

		source, ok := hitMap["_source"].(map[string]interface{})
		if !ok {
			continue
		}

		productJSON, err := json.Marshal(source)
		if err != nil {
			continue
		}

		var product model.Product
		if err := json.Unmarshal(productJSON, &product); err != nil {
			continue
		}

		products = append(products, product)
	}

	return products, int(total), nil
}

func (c *OpenSearchClient) IndexProduct(ctx context.Context, p model.Product) error {
	doc := map[string]interface{}{
		"id":             p.ID.String(),
		"store_id":       p.StoreID.String(),
		"barcode":        p.Barcode,
		"name":           p.Name,
		"description":    p.Description,
		"brand":          p.Brand,
		"category":       p.Category,
		"hsn_code":       p.HSNCode,
		"mrp":            p.MRP,
		"selling_price":  p.SellingPrice,
		"gst_rate":       p.GSTRate,
		"unit":           p.Unit,
		"image_url":      p.ImageURL,
		"thumbnail_url":  p.ThumbnailURL,
		"is_active":      p.IsActive,
		"is_returnable":  p.IsReturnable,
		"stock_quantity": p.StockQuantity,
		"reorder_point":  p.ReorderPoint,
		"sync_seq":       p.SyncSeq,
		"created_at":     p.CreatedAt,
		"updated_at":     p.UpdatedAt,
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(doc); err != nil {
		return fmt.Errorf("failed to encode product document: %w", err)
	}

	req := opensearchapi.IndexRequest{
		Index:      c.index,
		DocumentID: p.ID.String(),
		Body:       &buf,
		Refresh:    "false",
	}

	res, err := req.Do(ctx, c.client)
	if err != nil {
		return fmt.Errorf("failed to index product: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("failed to index product: %s", res.Status())
	}

	return nil
}

func (c *OpenSearchClient) DeleteProduct(ctx context.Context, storeID, productID string) error {
	req := opensearchapi.DeleteRequest{
		Index:      c.index,
		DocumentID: productID,
		Refresh:    "false",
	}

	res, err := req.Do(ctx, c.client)
	if err != nil {
		return fmt.Errorf("failed to delete product from index: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() && res.StatusCode != 404 {
		return fmt.Errorf("failed to delete product from index: %s", res.Status())
	}

	return nil
}

func SanitizeQuery(query string) string {
	sanitized := strings.ReplaceAll(query, "\n", " ")
	sanitized = strings.ReplaceAll(sanitized, "\t", " ")
	sanitized = strings.ReplaceAll(sanitized, "%", "\\%")
	sanitized = strings.ReplaceAll(sanitized, "_", "\\_")
	sanitized = strings.ReplaceAll(sanitized, "'", "''")
	sanitized = strings.ReplaceAll(sanitized, ";", "")
	return strings.TrimSpace(sanitized)
}
