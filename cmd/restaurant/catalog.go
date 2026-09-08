package main

import "food-ordering-api/internal/models"

func demoCatalog() *models.RestaurantCatalogInput {
	return &models.RestaurantCatalogInput{
		Restaurant: models.RestaurantInput{
			Name:        "Demo Bakery",
			Description: "Fresh pastries and coffee",
			Address:     "Moscow, Lesnaya street, 1",
			IsOpen:      true,
		},
		Items: []*models.MenuItemInput{
			{PartnerItemID: "croissant", Name: "Butter croissant", Description: "Classic flaky croissant", PriceKopecks: 22000, IsAvailable: true, PreparationMin: 7},
			{PartnerItemID: "latte", Name: "Latte", Description: "Double espresso with milk", PriceKopecks: 28000, IsAvailable: true, PreparationMin: 5},
			{PartnerItemID: "cheesecake", Name: "Cheesecake", Description: "Vanilla cheesecake", PriceKopecks: 35000, IsAvailable: true, PreparationMin: 3},
		},
	}
}
