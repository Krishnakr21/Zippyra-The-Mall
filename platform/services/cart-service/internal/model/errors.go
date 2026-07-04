package model

import "errors"

var (
	ErrBarcodeNotFound   = errors.New("barcode not found")
	ErrInsufficientStock = errors.New("insufficient stock")
	ErrBarcodeInvalid    = errors.New("invalid barcode")
	ErrStoreNotFound     = errors.New("store session not found")
	ErrCartLocked        = errors.New("cart already locked")
	ErrPriceChanged      = errors.New("price changed during checkout")
	ErrCartEmpty         = errors.New("cart is empty")
)

