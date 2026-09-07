//go:build integration

package models

import (
	"errors"
	"sync"
	"testing"
)

func TestCheckoutSnapshotsAndStatus(t *testing.T) {
	db := testDatabase(t)
	m := NewModels(db)
	_, item := testCatalog(t, m, "partner")
	input := testCheckout(t, m, item.ID)
	order, err := m.Orders.CreateFromCart(input)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE menu_items SET name = 'New bread', price_kopecks = 20000 WHERE id = $1`, item.ID); err != nil {
		t.Fatal(err)
	}
	got, err := m.Orders.Get(order.Order.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Order.TotalKopecks != 30000 || got.Order.Status != OrderStatusPendingPartner || len(got.Items) != 1 || got.Items[0].Name != "Bread" || got.Items[0].PriceKopecks != 15000 || got.Items[0].Quantity != 2 {
		t.Fatalf("unexpected order snapshot: %+v, %+v", got.Order, got.Items)
	}
	got, err = m.Orders.UpdateStatus(order.Order.ID, &OrderStatusUpdateInput{Status: OrderStatusAccepted, PartnerOrderID: "external-1"})
	if err != nil || got.Order.Status != OrderStatusAccepted || got.Order.PartnerOrderID != "external-1" || got.Order.Version != 2 {
		t.Fatalf("status update: %+v, %v", got, err)
	}
	orders, meta, err := m.Orders.GetAllByCustomerPhone(input.Customer.Phone, Filters{Page: 1, PageSize: 10, Sort: "id", SortSafelist: []string{"id"}})
	if err != nil || len(orders) != 1 || meta.TotalRecords != 1 || orders[0].Order.ID != order.Order.ID {
		t.Fatalf("order list: %+v, %+v, %v", orders, meta, err)
	}
}

func TestCheckoutConcurrentIdempotency(t *testing.T) {
	db := testDatabase(t)
	m := NewModels(db)
	_, item := testCatalog(t, m, "partner")
	input := testCheckout(t, m, item.ID)
	const workers = 8
	orders := make([]*OrderView, workers)
	errs := make([]error, workers)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			orders[i], errs[i] = m.Orders.CreateFromCart(input)
		}()
	}
	close(start)
	wg.Wait()
	for i := range workers {
		if errs[i] != nil {
			t.Fatalf("checkout %d: %v", i, errs[i])
		}
		if orders[i].Order.ID != orders[0].Order.ID {
			t.Fatal("duplicate order created")
		}
	}
	for _, table := range []string{"orders", "order_items"} {
		var count int
		if err := db.QueryRow("SELECT count(*) FROM " + table).Scan(&count); err != nil || count != 1 {
			t.Fatalf("%s count: %d, %v", table, count, err)
		}
	}
}

func TestCheckoutRejectsEmptyAndUnavailable(t *testing.T) {
	db := testDatabase(t)
	m := NewModels(db)
	_, item := testCatalog(t, m, "partner")
	input := testCheckout(t, m, item.ID)
	if _, err := db.Exec(`UPDATE menu_items SET is_available = false WHERE id = $1`, item.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Orders.CreateFromCart(input); !errors.Is(err, ErrItemUnavailable) {
		t.Fatalf("unavailable item: %v", err)
	}
	if _, err := m.Carts.DeleteItem(input.CartID, item.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Orders.CreateFromCart(input); !errors.Is(err, ErrCartIsEmpty) {
		t.Fatalf("empty cart: %v", err)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM orders`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("orders after rejection: %d, %v", count, err)
	}
}

func TestCheckoutRollsBackWhenItemInsertFails(t *testing.T) {
	db := testDatabase(t)
	m := NewModels(db)
	_, item := testCatalog(t, m, "partner")
	input := testCheckout(t, m, item.ID)
	if _, err := db.Exec(`ALTER TABLE order_items ADD CONSTRAINT test_failure CHECK (quantity < 2)`); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Orders.CreateFromCart(input); err == nil {
		t.Fatal("expected insert failure")
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM orders`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("order was not rolled back: %d, %v", count, err)
	}
	cart, err := m.Carts.Get(input.CartID)
	if err != nil || len(cart.Items) != 1 || cart.Items[0].Quantity != 2 {
		t.Fatalf("cart changed after rollback: %+v, %v", cart, err)
	}
}
