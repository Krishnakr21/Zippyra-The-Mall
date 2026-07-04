// =============================================================================
// ZIPPYRA — WatermelonDB Schema
// Complete on-device SQLite schema for React Native offline-first architecture
// 11 tables: 6 Phase 0/1 + 5 new at 100M scale
// Version 1.0 | March 2026 | Confidential
//
// USAGE:
//   import { schema, migrations } from './watermelondb_schema'
//   const database = new Database({ adapter, modelClasses, schema })
//
// CONFLICT RULE: Server always wins. Never merge. On sync conflict, server
//               version replaces local version entirely.
// SYNC RULE:     Never sync PII to device. user_id stored as opaque UUID only.
// =============================================================================

import { appSchema, tableSchema } from '@nozbe/watermelondb'

// =============================================================================
// TABLE 1: local_cart
// Primary offline cart buffer for scan queue.
// The source of truth for what the user has scanned while offline.
//
// Phase: 0 — required from day one
// Sync endpoint: POST /v1/cart/sync
// Conflict: Server wins — server merges and deduplicates by barcode+scanned_at
// =============================================================================
const localCartSchema = tableSchema({
  name: 'local_cart',
  columns: [
    { name: 'sku_id',        type: 'string'  },   // FK ref to catalog_cache.sku_id
    { name: 'barcode',       type: 'string'  },   // Original scanned barcode
    { name: 'store_id',      type: 'string'  },   // Store this item was scanned in
    { name: 'qty',           type: 'number'  },   // Current quantity (may be updated by duplicate scan)
    { name: 'unit_price',    type: 'number'  },   // Price at time of scan (paise / 100)
    { name: 'mrp',           type: 'number'  },   // MRP at time of scan (paise / 100)
    { name: 'gst_rate',      type: 'number'  },   // GST % at time of scan
    { name: 'hsn_code',      type: 'string'  },   // For GST breakdown display
    { name: 'product_name',  type: 'string'  },   // Denormalized: show in cart even if catalog_cache clears
    { name: 'image_url',     type: 'string'  },   // CDN URL for offline display
    { name: 'scanned_at',    type: 'number'  },   // Unix timestamp ms — ordering for sync
    { name: 'synced',        type: 'boolean' },   // false = not yet confirmed by server
    { name: 'sync_retries',  type: 'number'  },   // Incremented on each failed sync attempt
    { name: 'sync_error',    type: 'string', isOptional: true }, // Last sync error code
    { name: 'is_deleted',    type: 'boolean' },   // Soft delete: user removed item while offline
    //
    // DESIGN NOTE: unit_price, mrp, gst_rate, product_name are all denormalized here.
    // This is intentional: catalog_cache can be cleared or updated between scan and sync.
    // The cart item must always show what the user saw at time of scan.
  ],
})

// =============================================================================
// TABLE 2: catalog_cache
// Full product catalog for the current store, synced differentially.
// Powers: barcode scan lookup (when Redis/network unavailable), offline search.
//
// Phase: 0 — required from day one
// Sync endpoint: GET /v1/catalog/sync?store_id=X&since=<last_sync_ts>
// Staleness limit: 72 hours — app prompts fresh sync after this
// Conflict: Server wins — full replace per store_id on new store session
// =============================================================================
const catalogCacheSchema = tableSchema({
  name: 'catalog_cache',
  columns: [
    { name: 'sku_id',           type: 'string'  },
    { name: 'barcode',          type: 'string'  },   // Indexed for O(1) lookup on scan
    { name: 'store_id',         type: 'string'  },   // Scope: always delete-and-replace per store
    { name: 'name',             type: 'string'  },
    { name: 'brand',            type: 'string',  isOptional: true },
    { name: 'description',      type: 'string',  isOptional: true },
    { name: 'mrp',              type: 'number'  },   // paise × 100
    { name: 'effective_price',  type: 'number'  },   // paise × 100 — current selling price
    { name: 'gst_rate',         type: 'number'  },
    { name: 'hsn_code',         type: 'string'  },
    { name: 'category_id',      type: 'string'  },
    { name: 'category_name',    type: 'string'  },
    { name: 'shelf_id',         type: 'string',  isOptional: true }, // Aisle+shelf for store map
    { name: 'stock_status',     type: 'string'  },   // IN_STOCK | LOW_STOCK | OUT_OF_STOCK
    { name: 'rfid_required',    type: 'boolean' },
    { name: 'sold_by_weight',   type: 'boolean' },
    { name: 'image_url',        type: 'string',  isOptional: true },
    { name: 'has_offer',        type: 'boolean' },
    { name: 'offer_badge_text', type: 'string',  isOptional: true }, // e.g. "20% off"
    { name: 'is_active',        type: 'boolean' },
    { name: 'server_updated_at',type: 'number'  }, // Server-side updated_at for differential sync
    //
    // INDEX: barcode is the hot lookup path. SQLite index on barcode column is
    // created in migrations (see below). ~50K rows/store, barcode lookup must be < 5ms.
  ],
})

// =============================================================================
// TABLE 3: offer_cache
// Active offers for the current store. Used for offline offer evaluation.
// Cart engine evaluates offers from this table when offline.
//
// Phase: 0 — required from day one
// Sync strategy: Full refresh every 5 minutes (when online); pulled on store bind
// Offline behavior: expires_at checked on every use — expired offers silently skipped
// =============================================================================
const offerCacheSchema = tableSchema({
  name: 'offer_cache',
  columns: [
    { name: 'offer_id',       type: 'string'  },
    { name: 'store_id',       type: 'string'  },
    { name: 'type',           type: 'string'  },   // FLASH | COMBO | COUPON | LOYALTY_BONUS | PERSONAL
    { name: 'discount_type',  type: 'string'  },   // PERCENT | FLAT | FREE_ITEM
    { name: 'value',          type: 'number'  },   // Discount value (% or paise)
    { name: 'min_cart',       type: 'number'  },   // Minimum cart value to activate (paise)
    { name: 'max_discount',   type: 'number'  },   // Cap on discount amount (paise)
    { name: 'rules_json',     type: 'string'  },   // Serialized JSON of offer conditions
    { name: 'applicable_skus',type: 'string'  },   // JSON array of sku_ids this offer applies to
    { name: 'expires_at',     type: 'number'  },   // Unix timestamp ms
    { name: 'is_active',      type: 'boolean' },
    //
    // DESIGN NOTE: rules_json is intentionally a JSON blob. Offer rules are complex
    // (buy-X-get-Y, combo quantities, tiered discounts). Storing as normalized tables
    // on-device would require joins at cart evaluation time. JSON blob + in-memory
    // parse on cart mutation is faster and simpler.
    //
    // CONFLICT RULE: Server refreshes this table every 5 minutes. No local writes.
    // Expired offers (expires_at < now) must be filtered at query time — not deleted
    // eagerly, in case clock skew causes a false expiry.
  ],
})

// =============================================================================
// TABLE 4: scan_buffer
// Ordered queue of scans that occurred while offline.
// Separate from local_cart: scan_buffer is the raw event log; local_cart is state.
//
// Phase: 1 (offline scan mode)
// Sync endpoint: POST /v1/cart/sync { scans: [{barcode, qty, scanned_at}] }
// Server deduplicates by barcode + scanned_at timestamp
// =============================================================================
const scanBufferSchema = tableSchema({
  name: 'scan_buffer',
  columns: [
    { name: 'barcode',       type: 'string'  },
    { name: 'store_id',      type: 'string'  },
    { name: 'qty',           type: 'number'  },
    { name: 'scanned_at',    type: 'number'  },  // Unix timestamp ms — ordering for sync
    { name: 'synced',        type: 'boolean' },
    { name: 'retry_count',   type: 'number'  },
    { name: 'sync_error',    type: 'string',  isOptional: true },
    //
    // ORDERING RULE: Always sync in scanned_at ascending order.
    // The server processes scan events in time order to maintain correct
    // inventory hold sequencing. Out-of-order sync = wrong inventory holds.
    //
    // CLEANUP: Delete synced=true rows after successful POST /v1/cart/sync.
    // Keep failed rows (retry_count < 3). After 3 failures, flag for manual review.
  ],
})

// =============================================================================
// TABLE 5: recent_orders_cache
// Last 50 orders for the current user. Powers Quick Reorder (D-3 screen) offline.
//
// Phase: 1
// Sync strategy: Pull on app foreground; limit 50 most recent
// Offline behavior: Serves D-3 Quick Reorder screen without network
// =============================================================================
const recentOrdersCacheSchema = tableSchema({
  name: 'recent_orders_cache',
  columns: [
    { name: 'order_id',    type: 'string'  },
    { name: 'store_id',    type: 'string'  },
    { name: 'store_name',  type: 'string'  },  // Denormalized for display
    { name: 'total',       type: 'number'  },  // paise × 100
    { name: 'item_count',  type: 'number'  },
    { name: 'items_json',  type: 'string'  },  // [{sku_id, name, qty, price, image_url}]
    { name: 'status',      type: 'string'  },  // COMPLETED | REFUNDED
    { name: 'created_at',  type: 'number'  },  // Unix timestamp ms
    { name: 'invoice_s3_key', type: 'string', isOptional: true }, // For offline invoice link
  ],
})

// =============================================================================
// TABLE 6: store_session
// Single-row table. Current store session state.
// Replaced entirely on each store bind.
//
// Phase: 0 — required from day one
// Sync strategy: Written on POST /v1/store/bind. Single row — always overwrite.
// =============================================================================
const storeSessionSchema = tableSchema({
  name: 'store_session',
  columns: [
    { name: 'store_id',          type: 'string'  },
    { name: 'store_name',        type: 'string'  },
    { name: 'store_short_code',  type: 'string'  },
    { name: 'lat',               type: 'number'  },
    { name: 'lng',               type: 'number'  },
    { name: 'session_started_at',type: 'number'  }, // Unix timestamp ms
    { name: 'catalog_sync_at',   type: 'number'  }, // Last successful full sync ts
    { name: 'catalog_version',   type: 'string'  }, // Hash from Redis store_catalog_version
    { name: 'jwt_expiry',        type: 'number'  }, // Store-scoped JWT expiry ts
    { name: 'is_open',           type: 'boolean' },
    { name: 'gate_count',        type: 'number'  }, // Number of exit gates
    { name: 'max_capacity',      type: 'number',  isOptional: true },
    //
    // SINGLE ROW: This table always has exactly 1 row per device.
    // On new store bind: DELETE all + INSERT new row.
    // catalog_version is compared with Redis store_catalog_version:{store_id} on
    // every app foreground event to decide whether delta sync is needed.
  ],
})

// =============================================================================
// TABLE 7: feature_flags_cache    [NEW — 100M scale Phase 2]
// Per-store feature flags. Prevents unnecessary A/B test flicker on app start.
// App reads flags from here on cold start; background-refreshes from server.
//
// Phase: 2
// Sync: On store bind + background every 5 minutes
// Server source: Redis feature_flags:{store_id} → PostgreSQL feature_flags table
// =============================================================================
const featureFlagsCacheSchema = tableSchema({
  name: 'feature_flags_cache',
  columns: [
    { name: 'store_id',      type: 'string'  },
    { name: 'flag_key',      type: 'string'  },   // e.g. 'ar_navigation', 'upi_lite_v2'
    { name: 'enabled',       type: 'boolean' },
    { name: 'rollout_pct',   type: 'number'  },   // 0–100 — server already resolved for this user
    { name: 'variant',       type: 'string',  isOptional: true }, // A/B variant if applicable
    { name: 'payload_json',  type: 'string',  isOptional: true }, // Extra config for the flag
    { name: 'fetched_at',    type: 'number'  }, // Unix ts — stale after 5 min
  ],
})

// =============================================================================
// TABLE 8: loyalty_balance_cache  [NEW — 100M scale Phase 1]
// Loyalty balance shown on home screen, cart, and payment screens.
// Prevents hitting server on every cart open just to show points balance.
//
// Phase: 1 (with loyalty feature rollout)
// Sync: On order.completed event + app foreground + manual refresh
// TTL: Stale after 5 minutes — show cached value with "~" prefix if stale
// =============================================================================
const loyaltyBalanceCacheSchema = tableSchema({
  name: 'loyalty_balance_cache',
  columns: [
    { name: 'balance',           type: 'number'  },  // Current point balance
    { name: 'tier',              type: 'string'  },  // BRONZE | SILVER | GOLD | PLATINUM
    { name: 'tier_name',         type: 'string'  },  // Display name e.g. "Gold Member"
    { name: 'points_to_next_tier',type: 'number' },  // How many more points to next tier
    { name: 'last_order_pts',    type: 'number'  },  // Points earned in last order
    { name: 'multiplier',        type: 'number'  },  // Current tier multiplier (e.g. 2.0)
    { name: 'fetched_at',        type: 'number'  },  // Unix ts — stale after 300s
    //
    // SINGLE ROW: Always 1 row. DELETE + INSERT on refresh.
    // Shows "~1,240 pts" (with tilde) when stale > 5 min to signal approximation.
  ],
})

// =============================================================================
// TABLE 9: notification_inbox     [NEW — 100M scale Phase 1]
// Last 30 notifications for in-app notification centre (A-10 screen).
// Allows A-10 to render instantly from local cache on screen open.
//
// Phase: 1
// Sync: On FCM push received + app foreground
// Limit: 30 most recent. Older notifications fetched from server on scroll.
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
    { name: 'sent_at',         type: 'number'  },  // Unix ts ms
    { name: 'order_id',        type: 'string',  isOptional: true }, // For tap-to-navigate
    //
    // DESIGN: This is a read-only cache. Writes happen when:
    //   (a) FCM push received → insert new row + trim to 30 rows
    //   (b) App foreground + last_sync > 30s → pull latest from server
    // Marking read: optimistic local update + fire-and-forget PATCH /v1/notifications/{id}/read
  ],
})

// =============================================================================
// TABLE 10: user_profile_cache    [NEW — 100M scale Phase 1]
// Basic user profile for display in settings and checkout screens.
// Avoids a network call to show the user's name and phone on every app open.
//
// Phase: 1
// Sync: On login + profile update + app foreground (if stale > 1hr)
// =============================================================================
const userProfileCacheSchema = tableSchema({
  name: 'user_profile_cache',
  columns: [
    { name: 'user_id',       type: 'string'  },  // Opaque UUID only — no full PII stored
    { name: 'display_name',  type: 'string'  },
    { name: 'phone_masked',  type: 'string'  },  // e.g. "98765*****" — never full phone
    { name: 'email',         type: 'string',  isOptional: true }, // If provided
    { name: 'avatar_url',    type: 'string',  isOptional: true },
    { name: 'loyalty_tier',  type: 'string'  },  // Denormalized for header display
    { name: 'fetched_at',    type: 'number'  },  // Unix ts — stale after 3600s
    //
    // PII NOTE: Full phone number is NEVER stored on device in WatermelonDB.
    // Masked version only. Actual auth is handled via JWT in react-native-keychain.
    // This table is for display purposes only.
    //
    // SINGLE ROW: Always 1 row. DELETE + INSERT on profile update.
  ],
})

// =============================================================================
// TABLE 11: store_session_cache   [NEW — 100M scale Phase 2]
// Extended store metadata cache. Separates from store_session (which is lean
// operational state) to cache richer store info for UI rendering.
//
// Phase: 2
// Sync: On store bind + background refresh every 10 minutes
// =============================================================================
const storeSessionCacheSchema = tableSchema({
  name: 'store_session_cache',
  columns: [
    { name: 'store_id',          type: 'string'  },
    { name: 'opening_time',      type: 'string'  },  // "09:00" — for store hours display
    { name: 'closing_time',      type: 'string'  },  // "22:00"
    { name: 'quiet_hours_start', type: 'string',  isOptional: true },
    { name: 'quiet_hours_end',   type: 'string',  isOptional: true },
    { name: 'features_json',     type: 'string'  },  // ["rfid_exit","self_checkout","pharmacy"]
    { name: 'aisle_map_svg_key', type: 'string',  isOptional: true }, // S3 key for store map SVG
    { name: 'contact_phone',     type: 'string',  isOptional: true },
    { name: 'avg_rating',        type: 'number',  isOptional: true },
    { name: 'gate_statuses_json',type: 'string',  isOptional: true }, // Last known gate states
    { name: 'fetched_at',        type: 'number'  },
    //
    // SINGLE ROW per store_id (but typically only 1 active store at a time).
    // Drives: store map display, store hours screen, gate status fallback display.
  ],
})

// =============================================================================
// COMPLETE SCHEMA EXPORT
// =============================================================================
export const schema = appSchema({
  version: 11,  // Increment this when any table or column changes
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
// WatermelonDB requires explicit migrations for schema changes.
// RULE: Never modify an existing migration. Always add a new one.
// =============================================================================
import { schemaMigrations, addColumns, createTable } from '@nozbe/watermelondb/Schema/migrations'

export const migrations = schemaMigrations({
  migrations: [
    // v1 → v2: initial schema (Phase 0 — 6 tables)
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
          { name: 'sync_retries', type: 'number' }, { name: 'sync_error', type: 'string', isOptional: true },
          { name: 'is_deleted', type: 'boolean' },
        ]}),
        createTable({ name: 'catalog_cache', columns: [
          { name: 'sku_id', type: 'string' }, { name: 'barcode', type: 'string' },
          { name: 'store_id', type: 'string' }, { name: 'name', type: 'string' },
          { name: 'brand', type: 'string', isOptional: true }, { name: 'description', type: 'string', isOptional: true },
          { name: 'mrp', type: 'number' }, { name: 'effective_price', type: 'number' },
          { name: 'gst_rate', type: 'number' }, { name: 'hsn_code', type: 'string' },
          { name: 'category_id', type: 'string' }, { name: 'category_name', type: 'string' },
          { name: 'shelf_id', type: 'string', isOptional: true }, { name: 'stock_status', type: 'string' },
          { name: 'rfid_required', type: 'boolean' }, { name: 'sold_by_weight', type: 'boolean' },
          { name: 'image_url', type: 'string', isOptional: true }, { name: 'has_offer', type: 'boolean' },
          { name: 'offer_badge_text', type: 'string', isOptional: true }, { name: 'is_active', type: 'boolean' },
          { name: 'server_updated_at', type: 'number' },
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
          { name: 'barcode', type: 'string' }, { name: 'store_id', type: 'string' },
          { name: 'qty', type: 'number' }, { name: 'scanned_at', type: 'number' },
          { name: 'synced', type: 'boolean' }, { name: 'retry_count', type: 'number' },
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
          { name: 'jwt_expiry', type: 'number' }, { name: 'is_open', type: 'boolean' },
          { name: 'gate_count', type: 'number' }, { name: 'max_capacity', type: 'number', isOptional: true },
        ]}),
      ],
    },
    // v2 → v3: Phase 1 additions — loyalty, notifications, user profile
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
          { name: 'deep_link', type: 'string', isOptional: true }, { name: 'image_url', type: 'string', isOptional: true },
          { name: 'is_read', type: 'boolean' }, { name: 'sent_at', type: 'number' },
          { name: 'order_id', type: 'string', isOptional: true },
        ]}),
        createTable({ name: 'user_profile_cache', columns: [
          { name: 'user_id', type: 'string' }, { name: 'display_name', type: 'string' },
          { name: 'phone_masked', type: 'string' }, { name: 'email', type: 'string', isOptional: true },
          { name: 'avatar_url', type: 'string', isOptional: true }, { name: 'loyalty_tier', type: 'string' },
          { name: 'fetched_at', type: 'number' },
        ]}),
      ],
    },
    // v3 → v4: Phase 2 — feature flags, store session cache
    {
      toVersion: 4,
      steps: [
        createTable({ name: 'feature_flags_cache', columns: [
          { name: 'store_id', type: 'string' }, { name: 'flag_key', type: 'string' },
          { name: 'enabled', type: 'boolean' }, { name: 'rollout_pct', type: 'number' },
          { name: 'variant', type: 'string', isOptional: true }, { name: 'payload_json', type: 'string', isOptional: true },
          { name: 'fetched_at', type: 'number' },
        ]}),
        createTable({ name: 'store_session_cache', columns: [
          { name: 'store_id', type: 'string' }, { name: 'opening_time', type: 'string' },
          { name: 'closing_time', type: 'string' }, { name: 'quiet_hours_start', type: 'string', isOptional: true },
          { name: 'quiet_hours_end', type: 'string', isOptional: true }, { name: 'features_json', type: 'string' },
          { name: 'aisle_map_svg_key', type: 'string', isOptional: true }, { name: 'contact_phone', type: 'string', isOptional: true },
          { name: 'avg_rating', type: 'number', isOptional: true }, { name: 'gate_statuses_json', type: 'string', isOptional: true },
          { name: 'fetched_at', type: 'number' },
        ]}),
        // Add brand + offer fields to catalog_cache (Phase 2 catalog enhancement)
        addColumns({ table: 'catalog_cache', columns: [
          // already defined in v2 — no new columns needed in v4 for catalog_cache
        ]}),
      ],
    },
  ],
})

// =============================================================================
// SYNC LIFECYCLE HELPERS
// =============================================================================

/**
 * shouldRunFullCatalogSync
 * Returns true if the catalog needs a full refresh (not just differential).
 * Triggers: new store session, catalog_version mismatch, or staleness > 72hr.
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
 */
export function shouldRunDeltaSync(storeSession) {
  if (!storeSession) return false
  const ageMs = Date.now() - storeSession.catalog_sync_at
  const ONE_HOUR_MS = 60 * 60 * 1000
  return ageMs > ONE_HOUR_MS
}

/**
 * isCatalogDangerouslyStale
 * Returns true if catalog is so stale that checkout should be blocked
 * and user prompted to sync before proceeding.
 */
export function isCatalogDangerouslyStale(storeSession) {
  if (!storeSession) return true
  const ageMs = Date.now() - storeSession.catalog_sync_at
  const SEVENTY_TWO_HOURS_MS = 72 * 60 * 60 * 1000
  return ageMs > SEVENTY_TWO_HOURS_MS
}

/**
 * getUnSyncedScans
 * Returns scan_buffer rows that haven't been synced yet, ordered by scanned_at asc.
 * Used by the background sync job to build the POST /v1/cart/sync payload.
 */
export async function getUnSyncedScans(database) {
  const scanBufferCollection = database.get('scan_buffer')
  return await scanBufferCollection
    .query(
      // WatermelonDB query: where synced = false AND retry_count < 3
      // ordered by scanned_at ascending
    )
    .fetch()
}
