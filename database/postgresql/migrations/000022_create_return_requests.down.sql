-- 000022_create_return_requests.down.sql
ALTER TABLE orders DROP CONSTRAINT IF EXISTS unique_orders_payment_id;
DROP TABLE IF EXISTS return_requests;
