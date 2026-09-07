package models

import (
	"time"

	"food-ordering-api/internal/validator"
)

type Customer struct {
	Name    string `json:"name"`
	Phone   string `json:"phone"`
	Address string `json:"address"`
	Comment string `json:"comment,omitempty"`
}

type Order struct {
	ID             int64     `json:"id"`
	CartID         int64     `json:"cart_id"`
	RestaurantID   int64     `json:"restaurant_id"`
	PartnerID      string    `json:"partner_id"`
	PartnerOrderID string    `json:"partner_order_id,omitempty"`
	Status         string    `json:"status"`
	Customer       Customer  `json:"customer"`
	TotalKopecks   int       `json:"total_kopecks"`
	IdempotencyKey string    `json:"idempotency_key,omitempty"`
	Version        int32     `json:"version"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type OrderItem struct {
	ID            int64     `json:"id"`
	OrderID       int64     `json:"order_id"`
	MenuItemID    int64     `json:"menu_item_id"`
	PartnerItemID string    `json:"partner_item_id"`
	Name          string    `json:"name"`
	Quantity      int       `json:"quantity"`
	PriceKopecks  int       `json:"price_kopecks"`
	CreatedAt     time.Time `json:"created_at"`
}

type OrderView struct {
	Order *Order       `json:"order"`
	Items []*OrderItem `json:"items"`
}

type CheckoutInput struct {
	CartID         int64    `json:"cart_id"`
	Customer       Customer `json:"customer"`
	IdempotencyKey string   `json:"idempotency_key"`
}

type OrderStatusUpdateInput struct {
	Status         string `json:"status"`
	PartnerOrderID string `json:"partner_order_id"`
}

func ValidateCheckoutInput(v *validator.Validator, input *CheckoutInput) {
	v.Check(input.CartID > 0, "cart_id", "must be provided")
	v.Check(validator.NotBlank(input.Customer.Name), "customer.name", "must be provided")
	v.Check(validator.MaxChars(input.Customer.Name, 120), "customer.name", "must not be more than 120 characters long")
	v.Check(validator.NotBlank(input.Customer.Phone), "customer.phone", "must be provided")
	v.Check(validator.MaxChars(input.Customer.Phone, 32), "customer.phone", "must not be more than 32 characters long")
	v.Check(validator.NotBlank(input.Customer.Address), "customer.address", "must be provided")
	v.Check(validator.MaxChars(input.Customer.Address, 300), "customer.address", "must not be more than 300 characters long")
	v.Check(validator.MaxChars(input.Customer.Comment, 500), "customer.comment", "must not be more than 500 characters long")
	v.Check(validator.NotBlank(input.IdempotencyKey), "idempotency_key", "must be provided")
	v.Check(validator.MaxChars(input.IdempotencyKey, 120), "idempotency_key", "must not be more than 120 characters long")
}

func ValidateOrderStatusUpdateInput(v *validator.Validator, input *OrderStatusUpdateInput) {
	ValidateOrderStatus(v, input.Status)
	v.Check(validator.MaxChars(input.PartnerOrderID, 120), "partner_order_id", "must not be more than 120 characters long")
}
