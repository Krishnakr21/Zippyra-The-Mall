# catalog-service

Product catalog, pricing rules, offers, combos, recipes, bulk import, ElasticSearch indexing. Tables: categories, products, pricing_rules, offers, offer_rules, combo_rules, combo_items, recipes, recipe_ingredients. ES: products index (64 shards).

## Run Locally
```bash
go run ./cmd/server  # Port 8003
```
