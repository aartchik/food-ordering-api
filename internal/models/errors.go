package models

import "errors"

var (
	ErrRecordNotFound      = errors.New("record not found")
	ErrInvalidInput        = errors.New("invalid input")
	ErrEditConflict        = errors.New("edit conflict")
	ErrCartIsEmpty         = errors.New("cart is empty")
	ErrItemUnavailable     = errors.New("menu item is unavailable")
	ErrMixedCart           = errors.New("cart can contain items from one restaurant only")
	ErrInvalidTransition   = errors.New("invalid order status transition")
	ErrIdempotencyConflict = errors.New("idempotency key was already used for another request")
)
