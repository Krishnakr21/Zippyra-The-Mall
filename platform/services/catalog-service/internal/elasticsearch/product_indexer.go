package elasticsearch

// ProductIndexer indexes products into ElasticSearch products index.
// Uses store_id routing for physical data isolation.
// 64 shards, 1 replica.
type ProductIndexer struct{}

// SearchService handles autocomplete, full-text, voice, and OOS alternative search.
type SearchService struct{}
