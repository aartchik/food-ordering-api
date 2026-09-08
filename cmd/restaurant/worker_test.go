package main

import (
	"context"
	"errors"
	"io"
	"log"
	"testing"
	"time"

	"food-ordering-api/internal/models"
)

type fakeRestaurantAPI struct {
	syncErr error
	listErr error
	orders  []*models.OrderView
	updates []*models.OrderStatusUpdateInput
}

func (f *fakeRestaurantAPI) syncCatalog(_ context.Context, catalog *models.RestaurantCatalogInput) error {
	if catalog == nil || catalog.Restaurant.Name != "Demo Bakery" {
		return errors.New("invalid catalog")
	}
	return f.syncErr
}

func (f *fakeRestaurantAPI) listOrders(context.Context) ([]*models.OrderView, error) {
	return f.orders, f.listErr
}

func (f *fakeRestaurantAPI) updateOrderStatus(_ context.Context, _ int64, input *models.OrderStatusUpdateInput) error {
	f.updates = append(f.updates, input)
	return nil
}

func TestWorkerTick(t *testing.T) {
	api := &fakeRestaurantAPI{orders: []*models.OrderView{
		{Order: &models.Order{ID: 1, Status: models.OrderStatusPendingPartner, Version: 1}},
		{Order: &models.Order{ID: 2, Status: models.OrderStatusAccepted, Version: 2}},
		{Order: &models.Order{ID: 3, Status: models.OrderStatusCancelled, Version: 3}},
		nil,
	}}
	state := &serviceState{}
	w := &worker{api: api, state: state, interval: time.Second, logger: log.New(io.Discard, "", 0)}
	w.tick(context.Background())

	snapshot := state.snapshot()
	if !snapshot.Ready || snapshot.LastCatalogSync.IsZero() || snapshot.ProcessedOrders != 2 || snapshot.LastError != "" {
		t.Fatalf("state: %+v", snapshot)
	}
	if len(api.updates) != 2 || api.updates[0].Status != models.OrderStatusAccepted || api.updates[0].PartnerOrderID != "demo-1" || api.updates[1].Status != models.OrderStatusCooking {
		t.Fatalf("updates: %+v", api.updates)
	}
}

func TestWorkerRetriesCatalog(t *testing.T) {
	api := &fakeRestaurantAPI{syncErr: errors.New("unavailable")}
	state := &serviceState{}
	w := &worker{api: api, state: state, interval: time.Second, logger: log.New(io.Discard, "", 0)}
	w.tick(context.Background())
	if snapshot := state.snapshot(); snapshot.Ready || snapshot.LastError == "" {
		t.Fatalf("state: %+v", snapshot)
	}
	api.syncErr = nil
	w.tick(context.Background())
	if snapshot := state.snapshot(); !snapshot.Ready || snapshot.LastError != "" {
		t.Fatalf("state after retry: %+v", snapshot)
	}
}

func TestNextStatus(t *testing.T) {
	tests := []struct {
		from, to string
		ok       bool
	}{
		{models.OrderStatusPendingPartner, models.OrderStatusAccepted, true},
		{models.OrderStatusAccepted, models.OrderStatusCooking, true},
		{models.OrderStatusCooking, models.OrderStatusReady, true},
		{models.OrderStatusReady, models.OrderStatusDelivering, true},
		{models.OrderStatusDelivering, models.OrderStatusDelivered, true},
		{models.OrderStatusDelivered, "", false},
		{models.OrderStatusCancelled, "", false},
	}
	for _, tt := range tests {
		input, ok := nextStatus(&models.Order{ID: 9, Status: tt.from, Version: 4})
		if ok != tt.ok || ok && (input.Status != tt.to || input.Version != 4) {
			t.Fatalf("%s: %+v, %v", tt.from, input, ok)
		}
	}
}
