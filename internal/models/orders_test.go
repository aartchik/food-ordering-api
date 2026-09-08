package models

import (
	"errors"
	"strings"
	"testing"
)

func TestOrderModelRejectsInvalidCheckout(t *testing.T) {
	t.Parallel()
	for _, input := range []*CheckoutInput{nil, {CartID: 1}, {}} {
		order, err := (OrderModel{}).CreateFromCart(input)
		if !errors.Is(err, ErrInvalidInput) || order != nil {
			t.Fatalf("got (%v, %v), want (nil, ErrInvalidInput)", order, err)
		}
	}
}

func TestOrderModelRejectsInvalidStatusUpdate(t *testing.T) {
	t.Parallel()
	for _, input := range []*OrderStatusUpdateInput{
		nil,
		{Status: "unknown"},
		{Status: OrderStatusPendingPartner, PartnerOrderID: strings.Repeat("a", 121), Version: 1},
	} {
		order, err := (OrderModel{}).UpdateStatusForPartner("partner", 1, input)
		if !errors.Is(err, ErrInvalidInput) || order != nil {
			t.Fatalf("got (%v, %v), want (nil, ErrInvalidInput)", order, err)
		}
	}
}

func TestOrderModelRejectsInvalidID(t *testing.T) {
	t.Parallel()
	for _, id := range []int64{0, -1} {
		model := OrderModel{}
		if _, err := model.Get(id); !errors.Is(err, ErrRecordNotFound) {
			t.Fatalf("Get(%d): got %v, want ErrRecordNotFound", id, err)
		}
		if _, err := model.UpdateStatusForPartner("partner", id, nil); !errors.Is(err, ErrRecordNotFound) {
			t.Fatalf("UpdateStatusForPartner(%d): got %v, want ErrRecordNotFound", id, err)
		}
	}
}
