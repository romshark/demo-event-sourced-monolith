package event

import "github.com/romshark/conductor"

func init() {
	conductor.MustRegisterEventType[*EventRegisterProduct]("register-product")
	conductor.MustRegisterEventType[*EventEditProductStock]("edit-product-stock")
	conductor.MustRegisterEventType[*EventPlaceOrder]("place-order")
	conductor.MustRegisterEventType[*EventEditOrder]("edit-order")
	conductor.MustRegisterEventType[*EventCancelOrder]("cancel-order")
	conductor.MustRegisterEventType[*EventCompleteOrder]("complete-order")
}

type EventRegisterProduct struct {
	conductor.EventMetadata

	ID          int64  `json:"id,string"`
	ProductName string `json:"product-name"`
	Description string `json:"description,omitempty"`
	SKU         string `json:"sku,omitempty"`
}

type EventEditProductStock struct {
	conductor.EventMetadata

	ProductID int64 `json:"product_id,string"`
	Stock     int   `json:"stock"`
}

type EventPlaceOrder struct {
	conductor.EventMetadata

	ID              int64       `json:"id,string"`
	UserID          int64       `json:"user_id,string"`
	Items           []OrderItem `json:"items"`
	DeliveryAddress string      `json:"delivery_address"`
}

type Price struct {
	// Currency is an ISO 4217 3-letter code.
	Currency string `json:"currency"`

	// MinUnits is the amount in minor units (cents, pence, fils, etc.),
	// sent as a JSON string to avoid JS/JSON precision loss.
	MinUnits int64 `json:"min_units,string"`
}

type OrderItem struct {
	ProductID int64 `json:"product_id,string"`
	Amount    int32 `json:"amount"`
	Price     Price `json:"price"`
}

type EventEditOrder struct {
	conductor.EventMetadata

	ID int64 `json:"id,string"`

	// DeliveryAddress is "" if unchanged.
	DeliveryAddress string `json:"delivery_address"`
}

type EventCancelOrder struct {
	conductor.EventMetadata

	ID int64 `json:"id,string"`
}

type EventCompleteOrder struct {
	conductor.EventMetadata

	ID int64 `json:"id,string"`
}
