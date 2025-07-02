package main_test

import (
	"log/slog"
	"testing"

	"github.com/romshark/demo-event-sourced-monolith/database/testdb"
	"github.com/romshark/demo-event-sourced-monolith/event"
	"github.com/romshark/demo-event-sourced-monolith/pkg/orchestrator"
	"github.com/romshark/demo-event-sourced-monolith/service/orders"
	"github.com/stretchr/testify/require"
)

func TestOrders(t *testing.T) {
	f := func(
		t *testing.T,
		name string,
		fn func(t *testing.T, o *orchestrator.Orchestrator),
	) {
		t.Run(name, func(t *testing.T) {
			ctx := t.Context()
			log := slog.Default()
			db := testdb.New(t, ctx, log)

			s := orders.New(log, db)

			orch, err := orchestrator.Make(ctx, db,
				[]orchestrator.ProjectionSynchronizer{s},
				[]orchestrator.Handler{})
			require.NoError(t, err)

			// No need to run the dispatcher for testing the orders services
			// Because it's not a handler but a projection.

			fn(t, orch)
		})
	}

	f(t, "place order empty delivery address",
		func(t *testing.T, o *orchestrator.Orchestrator) {
			expectVersion, err := o.Publish(t.Context(), &event.EventRegisterProduct{
				ID: 567, Name: "Test Product",
			})
			require.NoError(t, err)

			_, err = o.Publish(t.Context(), &event.EventPlaceOrder{
				ID:     123,
				UserID: 123,
				Items: []event.OrderItem{
					{ProductID: 567, Amount: 42},
				},
				DeliveryAddress: "", // This is wrong.
			})
			require.ErrorIs(t, err, orders.ErrDeliveryAddressEmpty)

			v, err := o.Version(t.Context())
			require.NoError(t, err)
			require.Equal(t, expectVersion, v)
		})
}
