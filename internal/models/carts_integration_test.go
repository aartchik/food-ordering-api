//go:build integration

package models

import (
	"errors"
	"sync"
	"testing"
)

func TestCartQuantityLimit(t *testing.T) {
	m := NewModels(testDatabase(t))
	_, item := testCatalog(t, m, "partner")
	input := testCheckout(t, m, item.ID)
	if _, err := m.Carts.UpdateItem(input.CartID, item.ID, 99); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Carts.AddItem(input.CartID, item.ID, 1); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("quantity overflow: %v", err)
	}
	if _, err := m.Carts.UpdateItem(input.CartID, item.ID, 100); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid quantity: %v", err)
	}
	cart, err := m.Carts.Get(input.CartID)
	if err != nil || len(cart.Items) != 1 || cart.Items[0].Quantity != 99 {
		t.Fatalf("cart after rejection: %+v, %v", cart, err)
	}
}

func TestCartConcurrentRestaurants(t *testing.T) {
	m := NewModels(testDatabase(t))
	_, first := testCatalog(t, m, "first")
	_, second := testCatalog(t, m, "second")
	cart, err := m.Carts.Insert("")
	if err != nil {
		t.Fatal(err)
	}
	errs := make([]error, 2)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i, item := range []*MenuItem{first, second} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, errs[i] = m.Carts.AddItem(cart.Cart.ID, item.ID, 1)
		}()
	}
	close(start)
	wg.Wait()
	success, conflicts := 0, 0
	for _, err := range errs {
		if err == nil {
			success++
		} else if errors.Is(err, ErrMixedCart) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflicts != 1 {
		t.Fatalf("results: %v", errs)
	}
	got, err := m.Carts.Get(cart.Cart.ID)
	if err != nil || len(got.Items) != 1 {
		t.Fatalf("mixed cart: %+v, %v", got, err)
	}
}
