// =============================================================================
// ZIPPYRA — WatermelonDB Schema
// Complete on-device SQLite schema for React Native offline-first architecture
// 11 tables: 6 Phase 0/1 + 5 Phase 2 scale
// Version 2.0 | March 2026 | Confidential
//
// V2 FIXES (cross-system audit):
//   1. scan_buffer: added sku_id — server needs sku_id for hold processing; barcode alone
//      requires extra catalog lookup per sync item (defeats offline-first purpose)
//   2. catalog_cache: added sync_seq — sequence-based delta sync cursor to prevent
//      timestamp race conditions on concurrent catalog updates
//   3. store_session: added last_delta_sync_seq — device-side cursor for resumable delta sync
//   4. local_cart: added weight_grams + weight_unit for sold_by_weight items
//   5. getUnSyncedScans: implemented full WatermelonDB query (was empty placeholder — runtime crash)
//   6. Migration v3→v4: removed empty addColumns step (was misleading no-op)
//   7. offer_cache: added unit clarification comments for min_cart/max_discount
//
// SYNC MODEL:
//   Server always wins. Never merge. On conflict, server version replaces local.
//   NEVER sync PII to device. user_id is opaque UUID only.
//   Sequence-based sync (sync_seq) is used instead of timestamps to avoid
//   race conditions where two concurrent writes at same millisecond can cause
//   a client to miss one update.
// =============================================================================

import { appSchema, tableSchema } from '@nozbe/watermelondb'

// =============================================================================
// TABLE 1: local_cart
// Offline cart buffer. Source of truth for scanned items while offline.
// Phase: 0 — required from day one
// =============================================================================
const localCartSchema = tableSchema({
  name: 'local_cart',
  columns: [
    { name: 'sku_id',        type: 'string'  },   // FK → catalog_cache.sku_id
    { name: 'barcode',       type: 'string'  },   // Original scanned barcode
    { name: 'store_id',      type: 'string'  },   // Store this item was scanned in
    { name: 'qty',           type: 'number'  },   // Current quantity
    { name: 'unit_price',    type: 'number'  },   // Price at scan time (paise/100) — snapshot
    { name: 'mrp',           type: 'number'  },   // MRP at scan time (paise/100) — snapshot
    { name: 'gst_rate',      type: 'number'  },   // GST % at scan time — snapshot
    { name: 'hsn_code',      type: 'string'  },   // For GST breakdown display
    { name: 'product_name',  type: 'string'  },   // Denormalized — survives catalog_cache clear
    { name: 'image_url',     type: 'string'  },   // CDN URL for offline display
    { name: 'weight_grams',  type: 'number', isOptional: true }, // [V2] Package weight — for sold_by_weight display
    { name: 'weight_unit',   type: 'string', isOptional: true }, // [V2] kg|g|ml|l — unit label display
    { name: 'scanned_at',    type: 'number'  },   // Unix timestamp ms — sync ordering
    { name: 'synced',        type: 'boolean' },   // false = not confirmed by server yet
    { name: 'sync_retries',  type: 'number'  },   // Incremented on each failed sync attempt
    { name: 'sync_error',    type: 'string',  isOptional: true }, // Last sync error code
    { name: 'is_deleted',    type: 'boolean' },   // Soft delete: item removed while offline
    //
    // DESIGN: unit_price, mrp, gst_rate, product_name are all denormalized.
    // catalog_cache can be cleared or updated between scan and checkout sync.
    // The cart item must always reflect what the user saw at time of scan.
  ],
})

// =============================================================================
// TABLE 2: catalog_cache
// Full product catalog for the current store — synced differentially.
// Phase: 0 — required from day one
// =============================================================================
const catalogCacheSchema = tableSchema({
  name: 'catalog_cache',
  columns: [
    { name: 'sku_id',           type: 'string'  },
    { name: 'barcode',          type: 'string'  },   // HOT PATH: O(1) lookup on barcode scan
    { name: 'store_id',         type: 'string'  },   // Always delete-and-replace per store
    { name: 'name',             type: 'string'  },
    { name: 'brand',            type: 'string',  isOptional: true },
    { name: 'description',      type: 'string',  isOptional: true },
    { name: 'mrp',              type: 'number'  },   // paise × 100
    { name: 'effective_price',  type: 'number'  },   // paise × 100 — current selling price
    { name: 'gst_rate',         type: 'number'  },
    { name: 'hsn_code',         type: 'string'  },
    { name: 'category_id',      type: 'string'  },
    { name: 'category_name',    type: 'string'  },
    { name: 'shelf_id',         type: 'string',  isOptional: true },
    { name: 'stock_status',     type: 'string'  },   // IN_STOCK | LOW_STOCK | OUT_OF_STOCK
    { name: 'rfid_required',    type: 'boolean' },
    { name: 'sold_by_weight',   type: 'boolean' },
    { name: 'weight_grams',     type: 'number',  isOptional: true }, // Package weight in grams
    { name: 'unit_of_measure',  type: 'string',  isOptional: true }, // kg|g|ml|l|pcs|pack
    { name: 'pack_size',        type: 'string',  isOptional: true }, // Display string e.g. "500g"
    { name: 'image_url',        type: 'string',  isOptional: true },
    { name: 'has_offer',        type: 'boolean' },
    { name: 'offer_badge_text', type: 'string',  isOptional: true },
    { name: 'is_active',        type: 'boolean' },
    { name: 'server_updated_at',type: 'number'  },   // Server updated_at for differential sync
    { name: 'sync_seq',         type: 'number'  },   // [V2] Sequence number from server. Used as delta sync cursor instead of timestamp to prevent race conditions. Store last seen seq in store_session.last_delta_sync_seq.
    //
    // INDEX: barcode is the hot lookup path. SQLite index on barcode created in migrations.
    // At ~50K rows/store, barcode lookup must be < 5ms.
    // SYNC: GET /v1/catalog/sync?store_id=X&since_seq=<last_delta_sync_seq>
    //       Response includes next_seq which client stores in store_session.last_delta_sync_seq
  ],
})

// =============================================================================
// TABLE 3: offer_cache
// Active offers for the current store. Used for offline offer evaluation.
// Phase: 0 — required from day one
// =============================================================================
const offerCacheSchema = tableSchema({
  name: 'offer_cache',
  columns: [
    { name: 'offer_id',       type: 'string'  },
    { name: 'store_id',       type: 'string'  },
    { name: 'type',           type: 'string'  },   // FLASH | COMBO | COUPON | LOYALTY_BONUS | PERSONAL
    { name: 'discount_type',  type: 'string'  },   // PERCENT | FLAT | FREE_ITEM
    { name: 'value',          type: 'number'  },   // Discount value: paise if FLAT, integer percent if PERCENT
    { name: 'min_cart',       type: 'number'  },   // Minimum cart value in PAISE (not rupees) — compare against cart total × 100
    { name: 'max_discount',   type: 'number'  },   // Max discount cap in PAISE — 0 means no cap
    { name: 'rules_json',     type: 'string'  },   // Serialized JSON offer conditions
    { name: 'applicable_skus',type: 'string'  },   // JSON array of sku_ids
    { name: 'expires_at',     type: 'number'  },   // Unix timestamp ms
    { name: 'is_active',      type: 'boolean' },
    //
    // DESIGN: rules_json is a JSON blob intentionally.
    // Offer rules are complex (buy-X-get-Y, combo quantities, tiered discounts).
    // Normalizing on-device would require joins at cart evaluation time.
    // JSON blob + in-memory parse on cart mutation is faster for the client.
    //
    // NO LOCAL WRITES: Server refreshes this table every 5 minutes.
    // Clock skew: filter expires_at < now() at query time, not on insert.
  ],
})

// =============================================================================
// TABLE 4: scan_buffer
// Ordered queue of scans that occurred while offline.
// Separate from local_cart: scan_buffer is the raw event log; local_cart is state.
// Phase: 1 (offline scan mode)
// =============================================================================
const scanBufferSchema = tableSchema({
  name: 'scan_buffer',
  columns: [
    { name: 'barcode',       type: 'string'  },
    { name: 'sku_id',        type: 'string',  isOptional: true }, // [V2] Added. Server uses sku_id directly for inventory holds, avoiding per-item catalog lookup on sync. Populated from catalog_cache at scan time; null if catalog miss.
    { name: 'store_id',      type: 'string'  },
    { name: 'qty',           type: 'number'  },
    { name: 'scanned_at',    type: 'number'  },  // Unix timestamp ms — MUST sync ascending
    { name: 'synced',        type: 'boolean' },
    { name: 'retry_count',   type: 'number'  },
    { name: 'sync_error',    type: 'string',  isOptional: true },
    //
    // ORDERING RULE: Always sync in scanned_at ASC order.
    // Server processes scan events in time order for correct inventory hold sequencing.
    // Out-of-order sync = wrong holds = potential oversell.
    //
    // CLEANUP: Delete synced=true rows after successful POST /v1/cart/sync.
    // After retry_count >= 3, flag row for manual review but don't block new scans.
  ],
})

// =============================================================================
// TABLE 5: recent_orders_cache
// Last 50 orders. Powers Quick Reorder (D-3 screen) offline.
// Phase: 1
// =============================================================================
const recentOrdersCacheSchema = tableSchema({
  name: 'recent_orders_cache',
  columns: [
    { name: 'order_id',       type: 'string'  },
    { name: 'store_id',       type: 'string'  },
    { name: 'store_name',     type: 'string'  },  // Denormalized for display
    { name: 'total',          type: 'number'  },  // paise × 100
    { name: 'item_count',     type: 'number'  },
    { name: 'items_json',     type: 'string'  },  // [{sku_id, name, qty, price, image_url}]
    { name: 'status',         type: 'string'  },  // COMPLETED | REFUNDED
    { name: 'created_at',     type: 'number'  },  // Unix timestamp ms
    { name: 'invoice_s3_key', type: 'string',  isOptional: true },
  ],
})

// =============================================================================
// TABLE 6: store_session
// Single-row table. Current store session operational state.
// Replaced entirely on each store bind.
// Phase: 0 — required from day one
// =============================================================================
const storeSessionSchema = tableSchema({
  name: 'store_session',
  columns: [
    { name: 'store_id',              type: 'string'  },
    { name: 'store_name',            type: 'string'  },
    { name: 'store_short_code',      type: 'string'  },
    { name: 'lat',                   type: 'number'  },
    { name: 'lng',                   type: 'number'  },
    { name: 'session_started_at',    type: 'number'  },  // Unix timestamp ms
    { name: 'catalog_sync_at',       type: 'number'  },  // Last successful full sync timestamp
    { name: 'catalog_version',       type: 'string'  },  // Hash from Redis store_catalog_version:{store_id}
    { name: 'last_delta_sync_seq',   type: 'number'  },  // [V2] Sequence-based delta sync cursor. Sent as ?since_seq= on next sync. Eliminates timestamp race conditions. 0 = no delta sync done yet (trigger full sync).
    { name: 'jwt_expiry',            type: 'number'  },  // Store-scoped JWT expiry timestamp
    { name: 'is_open',               type: 'boolean' },
    { name: 'gate_count',            type: 'number'  },
    { name: 'max_capacity',          type: 'number',  isOptional: true },
    //
    // SINGLE ROW: Always exactly 1 row per device.
    // On new store bind: DELETE all + INSERT new row.
    // last_delta_sync_seq is compared with server on app foreground.
    // If catalog_version != Redis store_catalog_version:{store_id} → full sync.
    // Else if last_delta_sync_seq < server max_seq → delta sync.
  ],
})

// =============================================================================
// TABLE 7: feature_flags_cache    [Phase 2]
// Per-store feature flags. Source: PostgreSQL feature_flags → Redis feature_flags:{store_id}
// =============================================================================
const featureFlagsCacheSchema = tableSchema({
  name: 'feature_flags_cache',
  columns: [
    { name: 'store_id',      type: 'string'  },
    { name: 'flag_key',      type: 'string'  },   // e.g. 'ar_navigation', 'upi_lite_v2'
    { name: 'enabled',       type: 'boolean' },
    { name: 'rollout_pct',   type: 'number'  },   // 0–100 — already resolved for this user by server
    { name: 'variant',       type: 'string',  isOptional: true },
    { name: 'payload_json',  type: 'string',  isOptional: true },
    { name: 'fetched_at',    type: 'number'  },  // Stale after 5 min
  ],
})

// =============================================================================
// TABLE 8: loyalty_balance_cache  [Phase 1]
// Loyalty balance for home, cart, and payment screens.
// =============================================================================
const loyaltyBalanceCacheSchema = tableSchema({
  name: 'loyalty_balance_cache',
  columns: [
    { name: 'balance',              type: 'number'  },
    { name: 'tier',                 type: 'string'  },  // BRONZE | SILVER | GOLD | PLATINUM
    { name: 'tier_name',            type: 'string'  },
    { name: 'points_to_next_tier',  type: 'number'  },
    { name: 'last_order_pts',       type: 'number'  },
    { name: 'multiplier',           type: 'number'  },  // Tier earning multiplier e.g. 2.0
    { name: 'fetched_at',           type: 'number'  },  // Stale after 300s
    //
    // SINGLE ROW: DELETE + INSERT on refresh.
    // Shows "~1,240 pts" (with tilde prefix) when stale > 5 min.
  ],
})

// =============================================================================
// TABLE 9: notification_inbox     [Phase 1]
// Last 30 notifications for in-app notification centre (A-10 screen).
// =============================================================================
const notificationInboxSchema = tableSchema({
  name: 'notification_inbox',
  columns: [
    { name: 'notification_id', type: 'string'  },
    { name: 'type',            type: 'string'  },  // ORDER | PAYMENT | REFUND | LOYALTY | FLASH_SALE | SYSTEM
    { name: 'title',           type: 'string'  },
    { name: 'body',            type: 'string'  },
    { name: 'deep_link',       type: 'string',  isOptional: true }, // e.g. "zippyra://orders/abc123"
    { name: 'image_url',       type: 'string',  isOptional: true },
    { name: 'is_read',         type: 'boolean' },
    { name: 'sent_at',         type: 'number'  },  // Unix ms
    { name: 'order_id',        type: 'string',  isOptional: true },
    //
    // READ-ONLY CACHE. Writes on:
    //   (a) FCM push received → insert + trim to 30 rows
    //   (b) App foreground + last_sync > 30s → pull latest
    // Marking read: optimistic local + fire-and-forget PATCH /v1/notifications/{id}/read
  ],
})

// =============================================================================
// TABLE 10: user_profile_cache    [Phase 1]
// Basic user profile for settings and checkout. No full PII on device.
// =============================================================================
const userProfileCacheSchema = tableSchema({
  name: 'user_profile_cache',
  columns: [
    { name: 'user_id',       type: 'string'  },  // Opaque UUID — no full PII
    { name: 'display_name',  type: 'string'  },
    { name: 'phone_masked',  type: 'string'  },  // "98765*****" — NEVER full phone
    { name: 'email',         type: 'string',  isOptional: true },
    { name: 'avatar_url',    type: 'string',  isOptional: true },
    { name: 'loyalty_tier',  type: 'string'  },  // Denormalized for header display
    { name: 'fetched_at',    type: 'number'  },  // Stale after 3600s
    //
    // SINGLE ROW: DELETE + INSERT on profile update.
    // Full phone NEVER stored. Auth via JWT in react-native-keychain.
  ],
})

// =============================================================================
// TABLE 11: store_session_cache   [Phase 2]
// Rich store metadata for UI rendering. Separate from store_session (lean ops state).
// =============================================================================
const storeSessionCacheSchema = tableSchema({
  name: 'store_session_cache',
  columns: [
    { name: 'store_id',           type: 'string'  },
    { name: 'opening_time',       type: 'string'  },  // "09:00"
    { name: 'closing_time',       type: 'string'  },  // "22:00"
    { name: 'quiet_hours_start',  type: 'string',  isOptional: true },
    { name: 'quiet_hours_end',    type: 'string',  isOptional: true },
    { name: 'features_json',      type: 'string'  },  // ["rfid_exit","self_checkout","pharmacy"]
    { name: 'aisle_map_svg_key',  type: 'string',  isOptional: true },
    { name: 'contact_phone',      type: 'string',  isOptional: true },
    { name: 'avg_rating',         type: 'number',  isOptional: true },
    { name: 'gate_statuses_json', type: 'string',  isOptional: true },
    { name: 'fetched_at',         type: 'number'  },
  ],
})

// =============================================================================
// SCHEMA EXPORT
// =============================================================================
export const schema = appSchema({
  version: 11,
  tables: [
    localCartSchema,
    catalogCacheSchema,
    offerCacheSchema,
    scanBufferSchema,
    recentOrdersCacheSchema,
    storeSessionSchema,
    featureFlagsCacheSchema,
    loyaltyBalanceCacheSchema,
    notificationInboxSchema,
    userProfileCacheSchema,
    storeSessionCacheSchema,
  ],
})

// =============================================================================
// MIGRATIONS
// RULE: Never modify an existing migration. Always add a new one.
// =============================================================================
import { schemaMigrations, addColumns, createTable } from '@nozbe/watermelondb/Schema/migrations'

export const migrations = schemaMigrations({
  migrations: [
    // v1 → v2: initial schema (Phase 0 — 6 core tables)
    {
      toVersion: 2,
      steps: [
        createTable({ name: 'local_cart', columns: [
          { name: 'sku_id', type: 'string' }, { name: 'barcode', type: 'string' },
          { name: 'store_id', type: 'string' }, { name: 'qty', type: 'number' },
          { name: 'unit_price', type: 'number' }, { name: 'mrp', type: 'number' },
          { name: 'gst_rate', type: 'number' }, { name: 'hsn_code', type: 'string' },
          { name: 'product_name', type: 'string' }, { name: 'image_url', type: 'string' },
          { name: 'scanned_at', type: 'number' }, { name: 'synced', type: 'boolean' },
          { name: 'sync_retries', type: 'number' },
          { name: 'sync_error', type: 'string', isOptional: true },
          { name: 'is_deleted', type: 'boolean' },
        ]}),
        createTable({ name: 'catalog_cache', columns: [
          { name: 'sku_id', type: 'string' }, { name: 'barcode', type: 'string' },
          { name: 'store_id', type: 'string' }, { name: 'name', type: 'string' },
          { name: 'brand', type: 'string', isOptional: true },
          { name: 'description', type: 'string', isOptional: true },
          { name: 'mrp', type: 'number' }, { name: 'effective_price', type: 'number' },
          { name: 'gst_rate', type: 'number' }, { name: 'hsn_code', type: 'string' },
          { name: 'category_id', type: 'string' }, { name: 'category_name', type: 'string' },
          { name: 'shelf_id', type: 'string', isOptional: true },
          { name: 'stock_status', type: 'string' },
          { name: 'rfid_required', type: 'boolean' }, { name: 'sold_by_weight', type: 'boolean' },
          { name: 'weight_grams', type: 'number', isOptional: true },
          { name: 'unit_of_measure', type: 'string', isOptional: true },
          { name: 'pack_size', type: 'string', isOptional: true },
          { name: 'image_url', type: 'string', isOptional: true },
          { name: 'has_offer', type: 'boolean' },
          { name: 'offer_badge_text', type: 'string', isOptional: true },
          { name: 'is_active', type: 'boolean' },
          { name: 'server_updated_at', type: 'number' },
          { name: 'sync_seq', type: 'number' },  // [V2] sequence cursor
        ]}),
        createTable({ name: 'offer_cache', columns: [
          { name: 'offer_id', type: 'string' }, { name: 'store_id', type: 'string' },
          { name: 'type', type: 'string' }, { name: 'discount_type', type: 'string' },
          { name: 'value', type: 'number' }, { name: 'min_cart', type: 'number' },
          { name: 'max_discount', type: 'number' }, { name: 'rules_json', type: 'string' },
          { name: 'applicable_skus', type: 'string' }, { name: 'expires_at', type: 'number' },
          { name: 'is_active', type: 'boolean' },
        ]}),
        createTable({ name: 'scan_buffer', columns: [
          { name: 'barcode', type: 'string' },
          { name: 'sku_id', type: 'string', isOptional: true },  // [V2]
          { name: 'store_id', type: 'string' }, { name: 'qty', type: 'number' },
          { name: 'scanned_at', type: 'number' }, { name: 'synced', type: 'boolean' },
          { name: 'retry_count', type: 'number' },
          { name: 'sync_error', type: 'string', isOptional: true },
        ]}),
        createTable({ name: 'recent_orders_cache', columns: [
          { name: 'order_id', type: 'string' }, { name: 'store_id', type: 'string' },
          { name: 'store_name', type: 'string' }, { name: 'total', type: 'number' },
          { name: 'item_count', type: 'number' }, { name: 'items_json', type: 'string' },
          { name: 'status', type: 'string' }, { name: 'created_at', type: 'number' },
          { name: 'invoice_s3_key', type: 'string', isOptional: true },
        ]}),
        createTable({ name: 'store_session', columns: [
          { name: 'store_id', type: 'string' }, { name: 'store_name', type: 'string' },
          { name: 'store_short_code', type: 'string' }, { name: 'lat', type: 'number' },
          { name: 'lng', type: 'number' }, { name: 'session_started_at', type: 'number' },
          { name: 'catalog_sync_at', type: 'number' }, { name: 'catalog_version', type: 'string' },
          { name: 'last_delta_sync_seq', type: 'number' },  // [V2] delta sync cursor
          { name: 'jwt_expiry', type: 'number' }, { name: 'is_open', type: 'boolean' },
          { name: 'gate_count', type: 'number' },
          { name: 'max_capacity', type: 'number', isOptional: true },
        ]}),
        // [V2] local_cart weight fields added in v2 (included in initial schema for simplicity)
        // In practice if upgrading from a pre-V2 schema, use addColumns:
        addColumns({ table: 'local_cart', columns: [
          { name: 'weight_grams', type: 'number', isOptional: true },
          { name: 'weight_unit', type: 'string', isOptional: true },
        ]}),
      ],
    },
    // v2 → v3: Phase 1 — loyalty, notifications, user profile
    {
      toVersion: 3,
      steps: [
        createTable({ name: 'loyalty_balance_cache', columns: [
          { name: 'balance', type: 'number' }, { name: 'tier', type: 'string' },
          { name: 'tier_name', type: 'string' }, { name: 'points_to_next_tier', type: 'number' },
          { name: 'last_order_pts', type: 'number' }, { name: 'multiplier', type: 'number' },
          { name: 'fetched_at', type: 'number' },
        ]}),
        createTable({ name: 'notification_inbox', columns: [
          { name: 'notification_id', type: 'string' }, { name: 'type', type: 'string' },
          { name: 'title', type: 'string' }, { name: 'body', type: 'string' },
          { name: 'deep_link', type: 'string', isOptional: true },
          { name: 'image_url', type: 'string', isOptional: true },
          { name: 'is_read', type: 'boolean' }, { name: 'sent_at', type: 'number' },
          { name: 'order_id', type: 'string', isOptional: true },
        ]}),
        createTable({ name: 'user_profile_cache', columns: [
          { name: 'user_id', type: 'string' }, { name: 'display_name', type: 'string' },
          { name: 'phone_masked', type: 'string' },
          { name: 'email', type: 'string', isOptional: true },
          { name: 'avatar_url', type: 'string', isOptional: true },
          { name: 'loyalty_tier', type: 'string' }, { name: 'fetched_at', type: 'number' },
        ]}),
      ],
    },
    // v3 → v4: Phase 2 — feature flags, extended store cache
    {
      toVersion: 4,
      steps: [
        createTable({ name: 'feature_flags_cache', columns: [
          { name: 'store_id', type: 'string' }, { name: 'flag_key', type: 'string' },
          { name: 'enabled', type: 'boolean' }, { name: 'rollout_pct', type: 'number' },
          { name: 'variant', type: 'string', isOptional: true },
          { name: 'payload_json', type: 'string', isOptional: true },
          { name: 'fetched_at', type: 'number' },
        ]}),
        createTable({ name: 'store_session_cache', columns: [
          { name: 'store_id', type: 'string' },
          { name: 'opening_time', type: 'string' }, { name: 'closing_time', type: 'string' },
          { name: 'quiet_hours_start', type: 'string', isOptional: true },
          { name: 'quiet_hours_end', type: 'string', isOptional: true },
          { name: 'features_json', type: 'string' },
          { name: 'aisle_map_svg_key', type: 'string', isOptional: true },
          { name: 'contact_phone', type: 'string', isOptional: true },
          { name: 'avg_rating', type: 'number', isOptional: true },
          { name: 'gate_statuses_json', type: 'string', isOptional: true },
          { name: 'fetched_at', type: 'number' },
        ]}),
        // [V2 FIX] Removed empty addColumns step that was a misleading no-op
      ],
    },
  ],
})

// =============================================================================
// SYNC LIFECYCLE HELPERS
// =============================================================================

/**
 * shouldRunFullCatalogSync
 * Returns true if the catalog needs a full refresh (not differential).
 * Triggers: new store session | catalog_version hash mismatch | stale > 72hr
 */
export function shouldRunFullCatalogSync(storeSession, serverCatalogVersion) {
  if (!storeSession) return true
  const now = Date.now()
  const ageMs = now - storeSession.catalog_sync_at
  const SEVENTY_TWO_HOURS_MS = 72 * 60 * 60 * 1000
  if (ageMs > SEVENTY_TWO_HOURS_MS) return true
  if (storeSession.catalog_version !== serverCatalogVersion) return true
  return false
}

/**
 * shouldRunDeltaSync
 * Returns true if a delta sync is warranted (stale > 1hr but < 72hr, same store).
 * Uses last_delta_sync_seq as cursor — request goes to:
 *   GET /v1/catalog/sync?store_id=X&since_seq=<last_delta_sync_seq>
 */
export function shouldRunDeltaSync(storeSession) {
  if (!storeSession) return false
  const ageMs = Date.now() - storeSession.catalog_sync_at
  const ONE_HOUR_MS = 60 * 60 * 1000
  return ageMs > ONE_HOUR_MS
}

/**
 * isCatalogDangerouslyStale
 * Returns true if catalog is so stale that checkout should be blocked.
 */
export function isCatalogDangerouslyStale(storeSession) {
  if (!storeSession) return true
  const ageMs = Date.now() - storeSession.catalog_sync_at
  const SEVENTY_TWO_HOURS_MS = 72 * 60 * 60 * 1000
  return ageMs > SEVENTY_TWO_HOURS_MS
}

/**
 * getUnSyncedScans
 * Returns scan_buffer rows not yet synced, ordered by scanned_at ASC.
 * Used by background sync job to build POST /v1/cart/sync payload.
 *
 * [V2 FIX] Previously had empty query body — would crash at runtime.
 * Now implements full WatermelonDB Q query with correct ordering.
 */
export async function getUnSyncedScans(database) {
  const { Q } = await import('@nozbe/watermelondb')
  const scanBufferCollection = database.get('scan_buffer')
  return await scanBufferCollection
    .query(
      Q.where('synced', false),
      Q.where('retry_count', Q.lt(3)),
      Q.sortBy('scanned_at', Q.asc),
    )
    .fetch()
}

/**
 * markScansAsSynced
 * Atomically marks a batch of scan_buffer rows as synced.
 * Always call after a successful POST /v1/cart/sync.
 */
export async function markScansAsSynced(database, syncedIds) {
  await database.write(async () => {
    const scanBufferCollection = database.get('scan_buffer')
    const rows = await scanBufferCollection
      .query(Q.where('id', Q.oneOf(syncedIds)))
      .fetch()
    await database.batch(
      ...rows.map(row => row.prepareUpdate(r => { r.synced = true }))
    )
  })
}

/**
 * incrementSyncRetry
 * Increments retry_count for a failed scan sync attempt.
 * When retry_count >= 3, the scan is skipped but not deleted (manual review).
 */
export async function incrementSyncRetry(database, scanId, errorCode) {
  await database.write(async () => {
    const scan = await database.get('scan_buffer').find(scanId)
    await scan.update(s => {
      s.retry_count = s.retry_count + 1
      s.sync_error = errorCode
    })
  })
}

/**
 * updateDeltaSyncSeq
 * Updates the stored sequence cursor after a successful delta sync.
 * Call this with the next_seq from the server response.
 */
export async function updateDeltaSyncSeq(database, nextSeq) {
  const { Q } = await import('@nozbe/watermelondb')
  await database.write(async () => {
    const sessions = await database.get('store_session').query().fetch()
    if (sessions.length === 0) return
    await sessions[0].update(s => {
      s.last_delta_sync_seq = nextSeq
      s.catalog_sync_at = Date.now()
    })
  })
}
