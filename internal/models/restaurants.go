package models

import (
	"time"

	"food-ordering-api/internal/validator"
)

type Restaurant struct {
	ID          int64     `json:"id"`
	PartnerID   string    `json:"partner_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Address     string    `json:"address"`
	IsOpen      bool      `json:"is_open"`
	Version     int32     `json:"version"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type RestaurantCatalogInput struct {
	Restaurant Restaurant       `json:"restaurant"`
	Items      []*MenuItemInput `json:"items"`
}

func ValidateRestaurant(v *validator.Validator, restaurant *Restaurant) {
	v.Check(validator.NotBlank(restaurant.Name), "name", "must be provided")
	v.Check(validator.MaxChars(restaurant.Name, 120), "name", "must not be more than 120 characters long")
	v.Check(validator.MaxChars(restaurant.Description, 500), "description", "must not be more than 500 characters long")
	v.Check(validator.MaxChars(restaurant.Address, 300), "address", "must not be more than 300 characters long")
}

func ValidateRestaurantCatalogInput(v *validator.Validator, input *RestaurantCatalogInput) {
	ValidateRestaurant(v, &input.Restaurant)
	v.Check(len(input.Items) > 0, "items", "must contain at least one menu item")
	v.Check(len(input.Items) <= 500, "items", "must not contain more than 500 menu items")

	partnerItemIDs := make([]string, 0, len(input.Items))
	for _, item := range input.Items {
		ValidateMenuItemInput(v, item)
		if item == nil {
			v.AddError("items", "must not contain null items")
			continue
		}
		partnerItemIDs = append(partnerItemIDs, item.PartnerItemID)
		if !validator.NotBlank(item.PartnerItemID) {
			v.AddError("items", "item partner ids must be provided")
		}
	}

	v.Check(validator.Unique(partnerItemIDs), "items", "item partner ids must be unique")
}
