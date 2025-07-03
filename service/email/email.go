package email

import (
	"context"
	"log/slog"
	"time"

	"github.com/romshark/demo-event-sourced-monolith/event"
)

type Service struct {
	log *slog.Logger
}

func New(log *slog.Logger) *Service {
	return &Service{
		log: log,
	}
}

func (s *Service) ProjectionID() int32 { return 1 }

func (m *Service) Backoff() (min, max time.Duration, factor, jitter float64) {
	min, max = time.Second, 60*time.Second
	factor, jitter = 2.0, 0.3
	return
}

func (s *Service) Handle(ctx context.Context, version int64, e event.Event) error {
	// This service just simulates sending emails.
	switch e := e.(type) {
	case *event.EventPlaceOrder:
		s.log.Info("order placement notification email sent",
			slog.Int64("user.id", e.UserID),
			slog.Int64("order.id", e.ID),
			slog.String("deliveryAddress", e.DeliveryAddress),
			slog.Any("items", e.Items))
	case *event.EventEditOrder:
		if e.DeliveryAddress != "" {
			send()
			s.log.Info("order delivery address change notification email sent",
				slog.Int64("order.id", e.ID),
				slog.String("newAddress", e.DeliveryAddress))
		}
	case *event.EventCancelOrder:
		send()
		s.log.Info("order cancelation notification email sent",
			slog.Int64("order.id", e.ID))
	}

	return nil
}

func send() {
	time.Sleep(300 * time.Millisecond)
}
