package models

import (
	"time"

	"food-ordering-api/internal/validator"
)

type Cart struct {
	ID        int64     `json:"id"`
	UserID    string    `json:"user_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CartItem struct {
	ID           int64     `json:"id"`
	CartID       int64     `json:"cart_id"`
	MenuItemID   int64     `json:"menu_item_id"`
	Quantity     int       `json:"quantity"`
	Name         string    `json:"name"`
	PriceKopecks int       `json:"price_kopecks"`
	CreatedAt    time.Time `json:"created_at"`
}

type CartView struct {
	Cart         *Cart       `json:"cart"`
	Items        []*CartItem `json:"items"`
	TotalKopecks int         `json:"total_kopecks"`
}

type CartItemInput struct {
	MenuItemID int64 `json:"menu_item_id"`
	Quantity   int   `json:"quantity"`
}

func ValidateCartItemInput(v *validator.Validator, input *CartItemInput) {
	v.Check(input.MenuItemID > 0, "menu_item_id", "must be provided")
	v.Check(input.Quantity >= 1 && input.Quantity <= 99, "quantity", "must be between 1 and 99")
}
