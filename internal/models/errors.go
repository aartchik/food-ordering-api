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
	ErrCartChanged         = errors.New("cart items changed")
)

type CartItemChange struct {
	MenuItemID          int64    `json:"menu_item_id"`
	ChangedFields       []string `json:"changed_fields"`
	CartName            string   `json:"cart_name"`
	CurrentName         string   `json:"current_name"`
	CartPriceKopecks    int      `json:"cart_price_kopecks"`
	CurrentPriceKopecks int      `json:"current_price_kopecks"`
	IsAvailable         bool     `json:"is_available"`
}

type CartChangedError struct {
	Items []CartItemChange `json:"items"`
}

func (e *CartChangedError) Error() string {
	return ErrCartChanged.Error()
}

func (e *CartChangedError) Unwrap() error {
	return ErrCartChanged
}
