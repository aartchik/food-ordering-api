package models

import "food-ordering-api/internal/validator"

const (
	OrderStatusPendingPartner = "pending_partner"
	OrderStatusAccepted       = "accepted"
	OrderStatusCooking        = "cooking"
	OrderStatusReady          = "ready"
	OrderStatusDelivering     = "delivering"
	OrderStatusDelivered      = "delivered"
	OrderStatusCancelled      = "cancelled"
)

func ValidateOrderStatus(v *validator.Validator, status string) {
	v.Check(validator.PermittedValue(status,
		OrderStatusPendingPartner,
		OrderStatusAccepted,
		OrderStatusCooking,
		OrderStatusReady,
		OrderStatusDelivering,
		OrderStatusDelivered,
		OrderStatusCancelled,
	), "status", "invalid order status")
}

func CanTransitionOrderStatus(from, to string) bool {
	transitions := map[string][]string{
		OrderStatusPendingPartner: {OrderStatusAccepted, OrderStatusCancelled},
		OrderStatusAccepted:       {OrderStatusCooking, OrderStatusCancelled},
		OrderStatusCooking:        {OrderStatusReady, OrderStatusCancelled},
		OrderStatusReady:          {OrderStatusDelivering},
		OrderStatusDelivering:     {OrderStatusDelivered},
	}
	return validator.PermittedValue(to, transitions[from]...)
}
