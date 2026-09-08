package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"food-ordering-api/internal/models"
)

type restaurantAPI interface {
	syncCatalog(context.Context, *models.RestaurantCatalogInput) error
	listOrders(context.Context) ([]*models.OrderView, error)
	updateOrderStatus(context.Context, int64, *models.OrderStatusUpdateInput) error
}

type serviceState struct {
	mu              sync.RWMutex
	ready           bool
	lastCatalogSync time.Time
	processedOrders int64
	lastError       string
}

func (s *serviceState) snapshot() stateSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return stateSnapshot{s.ready, s.lastCatalogSync, s.processedOrders, s.lastError}
}

type stateSnapshot struct {
	Ready           bool      `json:"ready"`
	LastCatalogSync time.Time `json:"last_catalog_sync,omitempty"`
	ProcessedOrders int64     `json:"processed_orders"`
	LastError       string    `json:"last_error,omitempty"`
}

type worker struct {
	api      restaurantAPI
	state    *serviceState
	interval time.Duration
	logger   *log.Logger
}

func (w *worker) run(ctx context.Context) {
	w.tick(ctx)
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.tick(ctx)
		}
	}
}

func (w *worker) tick(ctx context.Context) {
	state := w.state.snapshot()
	if !state.Ready {
		if err := w.api.syncCatalog(ctx, demoCatalog()); err != nil {
			w.recordError(fmt.Errorf("sync catalog: %w", err))
			return
		}
		w.state.mu.Lock()
		w.state.ready = true
		w.state.lastCatalogSync = time.Now().UTC()
		w.state.lastError = ""
		w.state.mu.Unlock()
	}

	orders, err := w.api.listOrders(ctx)
	if err != nil {
		w.recordError(fmt.Errorf("list orders: %w", err))
		return
	}
	for _, order := range orders {
		if order == nil || order.Order == nil {
			continue
		}
		input, ok := nextStatus(order.Order)
		if !ok {
			continue
		}
		if err := w.api.updateOrderStatus(ctx, order.Order.ID, input); err != nil {
			w.recordError(fmt.Errorf("update order %d: %w", order.Order.ID, err))
			continue
		}
		w.state.mu.Lock()
		w.state.processedOrders++
		w.state.lastError = ""
		w.state.mu.Unlock()
	}
}

func (w *worker) recordError(err error) {
	w.logger.Print(err)
	w.state.mu.Lock()
	w.state.lastError = err.Error()
	w.state.mu.Unlock()
}

func nextStatus(order *models.Order) (*models.OrderStatusUpdateInput, bool) {
	input := &models.OrderStatusUpdateInput{Version: order.Version}
	switch order.Status {
	case models.OrderStatusPendingPartner:
		input.Status = models.OrderStatusAccepted
		input.PartnerOrderID = fmt.Sprintf("demo-%d", order.ID)
	case models.OrderStatusAccepted:
		input.Status = models.OrderStatusCooking
	case models.OrderStatusCooking:
		input.Status = models.OrderStatusReady
	case models.OrderStatusReady:
		input.Status = models.OrderStatusDelivering
	case models.OrderStatusDelivering:
		input.Status = models.OrderStatusDelivered
	default:
		return nil, false
	}
	return input, true
}
