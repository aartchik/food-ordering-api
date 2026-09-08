package models

import (
	"testing"

	"food-ordering-api/internal/validator"
)

func TestValidateCheckoutInput(t *testing.T) {
	t.Parallel()

	input := &CheckoutInput{
		CartID: 1,
		Customer: Customer{
			Name:    "Anna",
			Phone:   "+79990000000",
			Address: "Moscow, Lesnaya 1",
		},
		IdempotencyKey: "checkout-1",
	}

	v := validator.New()
	ValidateCheckoutInput(v, input)

	if !v.Valid() {
		t.Fatalf("expected input to be valid, got errors: %v", v.Errors)
	}
}

func TestValidateCheckoutInputRejectsMissingCustomerData(t *testing.T) {
	t.Parallel()

	v := validator.New()
	ValidateCheckoutInput(v, &CheckoutInput{})

	for _, field := range []string{"cart_id", "customer.name", "customer.phone", "customer.address", "idempotency_key"} {
		if _, ok := v.Errors[field]; !ok {
			t.Fatalf("expected error for %s", field)
		}
	}
}

func TestValidateRestaurantCatalogInput(t *testing.T) {
	t.Parallel()

	input := &RestaurantCatalogInput{
		Restaurant: RestaurantInput{
			Name:    "Demo Bakery",
			Address: "Moscow, Lesnaya 1",
			IsOpen:  true,
		},
		Items: []*MenuItemInput{
			{
				PartnerItemID:  "croissant",
				Name:           "Butter croissant",
				PriceKopecks:   22000,
				IsAvailable:    true,
				PreparationMin: 7,
			},
		},
	}

	v := validator.New()
	ValidateRestaurantCatalogInput(v, input)

	if !v.Valid() {
		t.Fatalf("expected input to be valid, got errors: %v", v.Errors)
	}
}

func TestValidateRestaurantCatalogInputRejectsDuplicateItems(t *testing.T) {
	t.Parallel()

	input := &RestaurantCatalogInput{
		Restaurant: RestaurantInput{Name: "Demo Bakery"},
		Items: []*MenuItemInput{
			{PartnerItemID: "latte", Name: "Latte", PriceKopecks: 25000},
			{PartnerItemID: "latte", Name: "Latte", PriceKopecks: 25000},
		},
	}

	v := validator.New()
	ValidateRestaurantCatalogInput(v, input)

	if _, ok := v.Errors["items"]; !ok {
		t.Fatal("expected duplicate items error")
	}
}
