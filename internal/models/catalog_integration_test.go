//go:build integration

package models

import (
	"errors"
	"testing"
)

func TestCatalogUpsertAndMenuRollback(t *testing.T) {
	m := NewModels(testDatabase(t))
	r, item := testCatalog(t, m, "partner")
	id := r.ID
	r.Name = "Updated Bakery"
	if err := m.Restaurants.UpsertByPartnerID("partner", r); err != nil {
		t.Fatal(err)
	}
	got, err := m.Restaurants.Get(id)
	if err != nil || got.ID != id || got.Version != 2 || got.Name != r.Name {
		t.Fatalf("restaurant upsert: %+v, %v", got, err)
	}
	list, meta, err := m.Restaurants.GetAll(RestaurantListFilter{Query: "Bakery", OnlyOpen: true, Filters: Filters{Page: 1, PageSize: 1, Sort: "id", SortSafelist: []string{"id"}}})
	if err != nil || len(list) != 1 || meta.TotalRecords != 1 {
		t.Fatalf("restaurant search: %+v, %+v, %v", list, meta, err)
	}
	err = m.MenuItems.UpsertForRestaurant(id, []*MenuItemInput{
		{PartnerItemID: "bread", Name: "Changed", PriceKopecks: 20000, IsAvailable: true},
		{PartnerItemID: "invalid", Name: "Invalid", PriceKopecks: -1},
	})
	if err == nil {
		t.Fatal("expected invalid price to reject menu batch")
	}
	unchanged, err := m.MenuItems.Get(item.ID)
	if err != nil || unchanged.Name != "Bread" || unchanged.PriceKopecks != 15000 {
		t.Fatalf("menu batch was not rolled back: %+v, %v", unchanged, err)
	}
}

func TestCartOperationsAndRestaurantRestriction(t *testing.T) {
	m := NewModels(testDatabase(t))
	_, item := testCatalog(t, m, "partner-1")
	_, other := testCatalog(t, m, "partner-2")
	input := testCheckout(t, m, item.ID)
	cart, err := m.Carts.AddItem(input.CartID, item.ID, 1)
	if err != nil || len(cart.Items) != 1 || cart.Items[0].Quantity != 3 || cart.TotalKopecks != 45000 {
		t.Fatalf("add existing item: %+v, %v", cart, err)
	}
	if _, err := m.Carts.AddItem(input.CartID, other.ID, 1); !errors.Is(err, ErrMixedCart) {
		t.Fatalf("mixed restaurant cart: %v", err)
	}
	cart, err = m.Carts.UpdateItem(input.CartID, item.ID, 1)
	if err != nil || len(cart.Items) != 1 || cart.TotalKopecks != 15000 {
		t.Fatalf("update quantity: %+v, %v", cart, err)
	}
	cart, err = m.Carts.DeleteItem(input.CartID, item.ID)
	if err != nil || len(cart.Items) != 0 || cart.TotalKopecks != 0 {
		t.Fatalf("delete item: %+v, %v", cart, err)
	}
	if _, err := m.Carts.DeleteItem(input.CartID, item.ID); !errors.Is(err, ErrRecordNotFound) {
		t.Fatalf("delete missing item: %v", err)
	}
}
