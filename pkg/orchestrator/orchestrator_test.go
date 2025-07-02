package orchestrator_test

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/romshark/demo-event-sourced-monolith/database"
	"github.com/romshark/demo-event-sourced-monolith/database/testdb"
	"github.com/romshark/demo-event-sourced-monolith/event"
	"github.com/romshark/demo-event-sourced-monolith/pkg/orchestrator"

	"github.com/stretchr/testify/require"
)

type MockSynchronizer struct {
	projectionID int32
	returnErr    error

	WG            sync.WaitGroup
	ReceivedEvent []event.Event
}

func (m *MockSynchronizer) ProjectionID() int32 { return m.projectionID }

func (m *MockSynchronizer) Sync(
	ctx context.Context, e event.Event, tx *database.Tx,
) error {
	defer m.WG.Done()
	m.ReceivedEvent = append(m.ReceivedEvent, e)
	return m.returnErr
}

type MockHandler struct {
	projectionID int32
	returnErr    error

	WG               sync.WaitGroup
	ReceivedVersions []int64
	ReceivedEvent    []event.Event
}

func (m *MockHandler) ProjectionID() int32 { return m.projectionID }

func (m *MockHandler) Handle(ctx context.Context, version int64, e event.Event) error {
	defer m.WG.Done()
	m.ReceivedVersions = append(m.ReceivedVersions, version)
	m.ReceivedEvent = append(m.ReceivedEvent, e)
	return m.returnErr
}

func TestOrchestrator(t *testing.T) {
	ctx := t.Context()
	log := slog.Default()
	db := testdb.New(t, ctx, log)

	serviceFirst := &MockSynchronizer{projectionID: 1}
	serviceSecond := &MockSynchronizer{projectionID: 2}
	handlerFirst := &MockHandler{projectionID: 3}
	handlerSecond := &MockHandler{projectionID: 4}

	// We expect 11 existing events and 1 new.
	serviceFirst.WG.Add(12)
	serviceSecond.WG.Add(12)
	handlerFirst.WG.Add(12)
	handlerSecond.WG.Add(12)

	orch, err := orchestrator.Make(
		ctx, db,
		[]orchestrator.ProjectionSynchronizer{serviceFirst, serviceSecond},
		[]orchestrator.Handler{handlerFirst, handlerSecond},
	)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := orch.RunHandlerDispatcher(ctx, log); err != nil {
			if !errors.Is(err, context.Canceled) {
				panic(err)
			}
		}
	}()

	vBefore, err := orch.Version(ctx)
	require.NoError(t, err)

	// There are already 11 events in the log by default.
	require.Equal(t, int64(11), vBefore)

	vPub, err := orch.Publish(ctx, event.New(time.Time{}, &event.EventPlaceOrder{
		ID:     42,
		UserID: 404,
	}))
	require.NoError(t, err)
	require.Equal(t, int64(12), vPub)

	serviceFirst.WG.Wait()
	serviceSecond.WG.Wait()
	handlerFirst.WG.Wait()
	handlerSecond.WG.Wait()

	// Make sure all projections were affected correctly.
	for _, s := range [...]*MockSynchronizer{serviceFirst, serviceSecond} {
		require.Len(t, s.ReceivedEvent, 12)
		require.IsType(t, &event.EventPlaceOrder{}, s.ReceivedEvent[11])
		require.Equal(t, int64(42), s.ReceivedEvent[11].(*event.EventPlaceOrder).ID)
		require.Equal(t, int64(404), s.ReceivedEvent[11].(*event.EventPlaceOrder).UserID)
	}

	// Make sure all handlers were affected correctly.
	for _, h := range [...]*MockHandler{handlerFirst, handlerSecond} {
		require.Len(t, h.ReceivedEvent, 12)
		require.IsType(t, &event.EventPlaceOrder{}, h.ReceivedEvent[11])
		require.Equal(t, int64(42), h.ReceivedEvent[11].(*event.EventPlaceOrder).ID)
		require.Equal(t, int64(404), h.ReceivedEvent[11].(*event.EventPlaceOrder).UserID)
	}

	// Wait for the orchestrator to finish asynchronous tasks to avoid error logs.
	orch.Wait()

	vAfter, err := orch.Version(ctx)
	require.NoError(t, err)
	require.Equal(t, vPub, vAfter)
}
