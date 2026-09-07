package models

import (
	"time"

	"food-ordering-api/internal/validator"
)

type MenuItem struct {
	ID             int64     `json:"id"`
	RestaurantID   int64     `json:"restaurant_id"`
	PartnerItemID  string    `json:"partner_item_id"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	PriceKopecks   int       `json:"price_kopecks"`
	IsAvailable    bool      `json:"is_available"`
	PreparationMin int       `json:"preparation_min"`
	Version        int32     `json:"version"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type MenuItemInput struct {
	PartnerItemID  string `json:"partner_item_id"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	PriceKopecks   int    `json:"price_kopecks"`
	IsAvailable    bool   `json:"is_available"`
	PreparationMin int    `json:"preparation_min"`
}

func ValidateMenuItemInput(v *validator.Validator, item *MenuItemInput) {
	if item == nil {
		v.AddError("items", "must not contain null items")
		return
	}

	v.Check(validator.NotBlank(item.PartnerItemID), "partner_item_id", "must be provided")
	v.Check(validator.MaxChars(item.PartnerItemID, 120), "partner_item_id", "must not be more than 120 characters long")
	v.Check(validator.NotBlank(item.Name), "name", "must be provided")
	v.Check(validator.MaxChars(item.Name, 120), "name", "must not be more than 120 characters long")
	v.Check(validator.MaxChars(item.Description, 500), "description", "must not be more than 500 characters long")
	v.Check(item.PriceKopecks > 0, "price_kopecks", "must be greater than zero")
	v.Check(item.PreparationMin >= 0 && item.PreparationMin <= 240, "preparation_min", "must be between 0 and 240")
}
