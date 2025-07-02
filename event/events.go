package event

/*** REGISTRY ***/

var byTypeName = map[string]func() Event{
	"RegisterProduct":  func() (e Event) { return &EventRegisterProduct{} },
	"EditProductStock": func() (e Event) { return &EventEditProductStock{} },
	"PlaceOrder":       func() (e Event) { return &EventPlaceOrder{} },
	"EditOrder":        func() (e Event) { return &EventEditOrder{} },
	"CancelOrder":      func() (e Event) { return &EventCancelOrder{} },
	"CompleteOrder":    func() (e Event) { return &EventCompleteOrder{} },
}

/*** EVENT TYPES ***/

type EventRegisterProduct struct {
	event

	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func (EventRegisterProduct) typeName() string { return "RegisterProduct" }

type EventEditProductStock struct {
	event

	ProductID int64 `json:"product_id"`
	Stock     int   `json:"stock"`
}

func (EventEditProductStock) typeName() string { return "EditProductStock" }

type EventPlaceOrder struct {
	event

	ID              int64       `json:"id"`
	UserID          int64       `json:"user_id"`
	Items           []OrderItem `json:"items"`
	DeliveryAddress string      `json:"delivery_address"`
}

type OrderItem struct {
	ProductID int64 `json:"product_id"`
	Amount    int64 `json:"amount"`
}

func (EventPlaceOrder) typeName() string { return "PlaceOrder" }

type EventEditOrder struct {
	event

	ID int64 `json:"id"`

	// DeliveryAddress is "" if unchanged.
	DeliveryAddress string `json:"delivery_address"`
}

func (EventEditOrder) typeName() string { return "EditOrder" }

type EventCancelOrder struct {
	event

	ID int64 `json:"id"`
}

func (EventCancelOrder) typeName() string { return "CancelOrder" }

type EventCompleteOrder struct {
	event

	ID int64 `json:"id"`
}

func (EventCompleteOrder) typeName() string { return "CompleteOrder" }
