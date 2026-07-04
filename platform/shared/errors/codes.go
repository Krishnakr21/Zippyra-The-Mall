package errors

const (
	// ── Auth ──────────────────────────────────────────────────────────────
	ErrInvalidPhone      = "INVALID_PHONE_FORMAT"
	ErrOTPNotFound       = "OTP_NOT_FOUND"
	ErrOTPExpired        = "OTP_EXPIRED"
	ErrOTPInvalid        = "OTP_INVALID"
	ErrOTPMaxAttempts    = "OTP_MAX_ATTEMPTS_EXCEEDED"
	ErrRateLimitExceeded = "RATE_LIMIT_EXCEEDED"
	ErrTokenInvalid      = "TOKEN_INVALID"
	ErrTokenExpired      = "TOKEN_EXPIRED"
	ErrTokenBlacklisted  = "TOKEN_BLACKLISTED"
	ErrUnauthorized      = "UNAUTHORIZED"
	ErrForbidden         = "FORBIDDEN"
	ErrWrongUserType     = "WRONG_USER_TYPE"

	// ── Store ─────────────────────────────────────────────────────────────
	ErrStoreNotFound     = "STORE_NOT_FOUND"
	ErrStoreClosed       = "STORE_CLOSED"
	ErrStoreAtCapacity   = "STORE_AT_CAPACITY"
	ErrStoreAlreadyBound = "STORE_ALREADY_BOUND"
	ErrQRTokenInvalid    = "QR_TOKEN_INVALID"
	ErrQRTokenExpired    = "QR_TOKEN_EXPIRED"
	ErrQRAlreadyUsed     = "QR_ALREADY_USED"

	// ── Catalog ───────────────────────────────────────────────────────────
	ErrProductNotFound  = "PRODUCT_NOT_FOUND"
	ErrBarcodeNotFound  = "BARCODE_NOT_FOUND"
	ErrBarcodeInvalid   = "BARCODE_INVALID"
	ErrCategoryNotFound = "CATEGORY_NOT_FOUND"

	// ── Cart ──────────────────────────────────────────────────────────────
	ErrCartNotFound       = "CART_NOT_FOUND"
	ErrCartEmpty          = "CART_EMPTY"
	ErrCartLocked         = "CART_LOCKED"
	ErrItemNotInCart      = "ITEM_NOT_IN_CART"
	ErrInvalidQuantity    = "INVALID_QUANTITY"
	ErrPriceChanged       = "PRICE_CHANGED"
	ErrInsufficientStock  = "INSUFFICIENT_STOCK"
	ErrOfferNotApplicable = "OFFER_NOT_APPLICABLE"
	ErrCouponInvalid      = "COUPON_INVALID"
	ErrCouponExpired      = "COUPON_EXPIRED"

	// ── Payment ───────────────────────────────────────────────────────────
	ErrPaymentNotFound         = "PAYMENT_NOT_FOUND"
	ErrPaymentFailed           = "PAYMENT_FAILED"
	ErrPaymentPending          = "PAYMENT_PENDING"
	ErrPaymentAlreadyDone      = "PAYMENT_ALREADY_COMPLETED"
	ErrPaymentGatewayDown      = "PAYMENT_GATEWAY_UNAVAILABLE"
	ErrWebhookInvalidSignature = "WEBHOOK_INVALID_SIGNATURE"
	ErrIdempotencyConflict     = "IDEMPOTENCY_KEY_CONFLICT"
	ErrRefundFailed            = "REFUND_FAILED"
	ErrInsufficientLoyalty     = "INSUFFICIENT_LOYALTY_POINTS"

	// ── Order ─────────────────────────────────────────────────────────────
	ErrOrderNotFound       = "ORDER_NOT_FOUND"
	ErrOrderAlreadyExists  = "ORDER_ALREADY_EXISTS"
	ErrOrderNotPaid        = "ORDER_NOT_PAID"
	ErrReturnWindowClosed  = "RETURN_WINDOW_CLOSED"
	ErrItemNotReturnable   = "ITEM_NOT_RETURNABLE"
	ErrReturnAlreadyExists = "RETURN_ALREADY_REQUESTED"

	// ── Exit ──────────────────────────────────────────────────────────────
	ErrExitTokenInvalid = "EXIT_TOKEN_INVALID"
	ErrExitTokenExpired = "EXIT_TOKEN_EXPIRED"
	ErrExitTokenUsed    = "EXIT_TOKEN_ALREADY_USED"
	ErrExitWrongStore   = "EXIT_TOKEN_WRONG_STORE"
	ErrRFIDCheckFailed  = "RFID_CHECK_FAILED"

	// ── Inventory ─────────────────────────────────────────────────────────
	ErrGRNNotFound        = "GRN_NOT_FOUND"
	ErrInvalidStockAdjust = "INVALID_STOCK_ADJUSTMENT"

	// ── Device ────────────────────────────────────────────────────────────
	ErrDeviceNotFound = "DEVICE_NOT_FOUND"
	ErrDeviceOffline  = "DEVICE_OFFLINE"

	// ── User ──────────────────────────────────────────────────────────────
	ErrUserNotFound  = "USER_NOT_FOUND"
	ErrUserSuspended = "USER_SUSPENDED"

	// ── Compliance ────────────────────────────────────────────────────────
	ErrGSTINInvalid = "GSTIN_INVALID"
	ErrHSNNotFound  = "HSN_CODE_NOT_FOUND"

	// ── System ────────────────────────────────────────────────────────────
	ErrInternal           = "INTERNAL_SERVER_ERROR"
	ErrServiceUnavailable = "SERVICE_UNAVAILABLE"
	ErrRequestTooLarge    = "REQUEST_TOO_LARGE"
	ErrValidationFailed   = "VALIDATION_FAILED"
	ErrNotFound           = "NOT_FOUND"
	ErrConflict           = "CONFLICT"
	ErrTimeout            = "REQUEST_TIMEOUT"
)
