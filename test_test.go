package main_test

import (
	"log/slog"
	"testing"

	"github.com/romshark/conductor"
	"github.com/romshark/demo-event-sourced-monolith/database"
	"github.com/romshark/demo-event-sourced-monolith/event"
	"github.com/romshark/demo-event-sourced-monolith/service/orders"
	"github.com/stretchr/testify/require"
)

func TestOrders(t *testing.T) {
	f := func(
		t *testing.T,
		name string,
		fn func(t *testing.T, log *slog.Logger, o *conductor.Conductor),
	) {
		t.Run(name, func(t *testing.T) {
			ctx := t.Context()
			log := slog.Default()
			db := database.NewTestDB(t, log)

			s := orders.New(log, db)

			orch, err := conductor.Make(ctx, conductor.DefaultEventCodec, log, db,
				[]conductor.StatelessProcessor{s},
				[]conductor.StatefulProcessor{},
				[]conductor.Reactor{})
			require.NoError(t, err)

			// No need to run the dispatcher for testing the orders services
			// Because it's not a reactor.

			fn(t, log, orch)
		})
	}

	f(t, "place order empty delivery address",
		func(t *testing.T, log *slog.Logger, o *conductor.Conductor) {
			expectVersion, err := o.SyncAppend(
				t.Context(), log, &event.EventRegisterProduct{
					ID: 567, ProductName: "Test Product",
				},
			)
			require.NoError(t, err)

			_, err = o.SyncAppend(
				t.Context(),
				log,
				&event.EventPlaceOrder{
					ID:     123,
					UserID: 123,
					Items: []event.OrderItem{
						{ProductID: 567, Amount: 42},
					},
					DeliveryAddress: "", // This is wrong.
				},
			)
			require.ErrorIs(t, err, orders.ErrDeliveryAddressEmpty)

			v, err := o.Version(t.Context())
			require.NoError(t, err)
			require.Equal(t, expectVersion, v)
		})
}
