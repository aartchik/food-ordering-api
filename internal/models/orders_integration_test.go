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
	got, err = m.Orders.UpdateStatusForPartner("partner", order.Order.ID, &OrderStatusUpdateInput{Status: OrderStatusAccepted, PartnerOrderID: "external-1", Version: 1})
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

func TestPartnerOrdersAreIsolatedAndFiltered(t *testing.T) {
	db := testDatabase(t)
	m := NewModels(db)
	_, firstItem := testCatalog(t, m, "partner-1")
	_, secondItem := testCatalog(t, m, "partner-2")

	firstOrder, err := m.Orders.CreateFromCart(testCheckout(t, m, firstItem.ID))
	if err != nil {
		t.Fatal(err)
	}
	secondInput := testCheckout(t, m, secondItem.ID)
	secondInput.IdempotencyKey = "checkout-2"
	secondOrder, err := m.Orders.CreateFromCart(secondInput)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Orders.UpdateStatusForPartner("partner-1", firstOrder.Order.ID, &OrderStatusUpdateInput{Status: OrderStatusAccepted, PartnerOrderID: "external-1", Version: 1}); err != nil {
		t.Fatal(err)
	}

	filters := PartnerOrderListFilter{
		Status: OrderStatusAccepted,
		Filters: Filters{
			Page:         1,
			PageSize:     10,
			Sort:         "-created_at",
			SortSafelist: []string{"-created_at"},
		},
	}
	orders, metadata, err := m.Orders.GetAllForPartner("partner-1", filters)
	if err != nil || len(orders) != 1 || metadata.TotalRecords != 1 || orders[0].Order.ID != firstOrder.Order.ID || len(orders[0].Items) != 1 {
		t.Fatalf("partner orders: %+v, %+v, %v", orders, metadata, err)
	}
	if _, err := m.Orders.GetForPartner("partner-1", secondOrder.Order.ID); !errors.Is(err, ErrRecordNotFound) {
		t.Fatalf("foreign order: %v", err)
	}
	if _, err := m.Orders.GetForPartner("partner-2", secondOrder.Order.ID); err != nil {
		t.Fatalf("own order: %v", err)
	}

	emptyFilters := filters
	emptyFilters.Status = OrderStatusCooking
	orders, metadata, err = m.Orders.GetAllForPartner("partner-1", emptyFilters)
	if err != nil || len(orders) != 0 || metadata != (Metadata{}) {
		t.Fatalf("empty result: %+v, %+v, %v", orders, metadata, err)
	}
}

func TestPartnerOrderStatusTransitionsAndConflicts(t *testing.T) {
	m := NewModels(testDatabase(t))
	_, item := testCatalog(t, m, "partner")
	order, err := m.Orders.CreateFromCart(testCheckout(t, m, item.ID))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := m.Orders.UpdateStatusForPartner("other", order.Order.ID, &OrderStatusUpdateInput{Status: OrderStatusAccepted, PartnerOrderID: "external", Version: 1}); !errors.Is(err, ErrRecordNotFound) {
		t.Fatalf("foreign update: %v", err)
	}
	if _, err := m.Orders.UpdateStatusForPartner("partner", order.Order.ID, &OrderStatusUpdateInput{Status: OrderStatusCooking, Version: 1}); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("invalid transition: %v", err)
	}
	accepted, err := m.Orders.UpdateStatusForPartner("partner", order.Order.ID, &OrderStatusUpdateInput{Status: OrderStatusAccepted, PartnerOrderID: "external", Version: 1})
	if err != nil || accepted.Order.Version != 2 || accepted.Order.Status != OrderStatusAccepted {
		t.Fatalf("accept: %+v, %v", accepted, err)
	}
	if _, err := m.Orders.UpdateStatusForPartner("partner", order.Order.ID, &OrderStatusUpdateInput{Status: OrderStatusCooking, Version: 1}); !errors.Is(err, ErrEditConflict) {
		t.Fatalf("stale version: %v", err)
	}
	cooking, err := m.Orders.UpdateStatusForPartner("partner", order.Order.ID, &OrderStatusUpdateInput{Status: OrderStatusCooking, Version: 2})
	if err != nil || cooking.Order.Version != 3 || cooking.Order.Status != OrderStatusCooking || cooking.Order.PartnerOrderID != "external" {
		t.Fatalf("cooking: %+v, %v", cooking, err)
	}
	cancelled, err := m.Orders.UpdateStatusForPartner("partner", order.Order.ID, &OrderStatusUpdateInput{Status: OrderStatusCancelled, Version: 3})
	if err != nil || cancelled.Order.Status != OrderStatusCancelled {
		t.Fatalf("cancel: %+v, %v", cancelled, err)
	}
	if _, err := m.Orders.UpdateStatusForPartner("partner", order.Order.ID, &OrderStatusUpdateInput{Status: OrderStatusAccepted, PartnerOrderID: "external", Version: 4}); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("terminal transition: %v", err)
	}
}

func TestConcurrentPartnerStatusUpdates(t *testing.T) {
	m := NewModels(testDatabase(t))
	_, item := testCatalog(t, m, "partner")
	order, err := m.Orders.CreateFromCart(testCheckout(t, m, item.ID))
	if err != nil {
		t.Fatal(err)
	}

	inputs := []*OrderStatusUpdateInput{
		{Status: OrderStatusAccepted, PartnerOrderID: "external", Version: 1},
		{Status: OrderStatusCancelled, Version: 1},
	}
	errs := make([]error, len(inputs))
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i, input := range inputs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, errs[i] = m.Orders.UpdateStatusForPartner("partner", order.Order.ID, input)
		}()
	}
	close(start)
	wg.Wait()

	successes, conflicts := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrEditConflict):
			conflicts++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("results: %v", errs)
	}
	stored, err := m.Orders.GetForPartner("partner", order.Order.ID)
	if err != nil || stored.Order.Version != 2 {
		t.Fatalf("stored order: %+v, %v", stored, err)
	}
}
