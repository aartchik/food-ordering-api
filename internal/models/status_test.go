package models

import (
	"testing"

	"food-ordering-api/internal/validator"
)

func TestValidateOrderStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status string
		valid  bool
	}{
		{name: "pending partner", status: OrderStatusPendingPartner, valid: true},
		{name: "accepted", status: OrderStatusAccepted, valid: true},
		{name: "cooking", status: OrderStatusCooking, valid: true},
		{name: "ready", status: OrderStatusReady, valid: true},
		{name: "delivering", status: OrderStatusDelivering, valid: true},
		{name: "delivered", status: OrderStatusDelivered, valid: true},
		{name: "cancelled", status: OrderStatusCancelled, valid: true},
		{name: "invalid", status: "paid", valid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			v := validator.New()
			ValidateOrderStatus(v, tt.status)
			if v.Valid() != tt.valid {
				t.Fatalf("valid = %v, want %v", v.Valid(), tt.valid)
			}
		})
	}
}

func TestCanTransitionOrderStatus(t *testing.T) {
	t.Parallel()
	allowed := map[string][]string{
		OrderStatusPendingPartner: {OrderStatusAccepted, OrderStatusCancelled},
		OrderStatusAccepted:       {OrderStatusCooking, OrderStatusCancelled},
		OrderStatusCooking:        {OrderStatusReady, OrderStatusCancelled},
		OrderStatusReady:          {OrderStatusDelivering},
		OrderStatusDelivering:     {OrderStatusDelivered},
	}
	statuses := []string{OrderStatusPendingPartner, OrderStatusAccepted, OrderStatusCooking, OrderStatusReady, OrderStatusDelivering, OrderStatusDelivered, OrderStatusCancelled}
	for _, from := range statuses {
		for _, to := range statuses {
			want := false
			for _, candidate := range allowed[from] {
				want = want || candidate == to
			}
			if got := CanTransitionOrderStatus(from, to); got != want {
				t.Errorf("CanTransitionOrderStatus(%q, %q) = %v, want %v", from, to, got, want)
			}
		}
	}
}
