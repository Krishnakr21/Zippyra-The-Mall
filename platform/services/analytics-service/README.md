# analytics-service

Dashboard, ML models (fraud scoring, demand forecasting, dynamic pricing, RFM segmentation), recommendations, session stitching. ClickHouse tables: user_purchase_patterns, sales_summary, peak_hour_heatmap, shrinkage_signals, co_purchase_matrix, user_behavior_sessions, demand_forecasting_features, dynamic_pricing_signals, ml_fraud_features.

## Run Locally
```bash
go run ./cmd/server  # Port 8013
```
