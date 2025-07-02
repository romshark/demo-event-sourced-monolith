// Package orchestrator provides the system orchestrator that appends events onto
// the event log, synchronizes projections and handlers.
package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/romshark/demo-event-sourced-monolith/database"
	"github.com/romshark/demo-event-sourced-monolith/event"
)

var ErrProjectionIDCollision = errors.New("projection identifier must be globally unique")

var errNoNextVersion = errors.New("no next version")

type ProjectionIDReader interface {
	// ProjectionID returns a globally unique identifier of the projection.
	// This identifier is associated with a version in the database.
	ProjectionID() int32
}

// ProjectionSynchronizer synchronizes a projection when an event is being processed.
// ProjectionSynchronizer has the ability to check for invariants and reject events
// from being appended to the event log and published.
type ProjectionSynchronizer interface {
	ProjectionIDReader

	// Sync applies event onto the projection.
	// If Sync returns an error, the transaction is rolled back and
	// event publishing is rejected.
	Sync(ctx context.Context, event event.Event, tx *database.Tx) error
}

// Handler is different from a ProjectionSynchronizer because it's executed
// after an event was published. Unlike ProjectionSynchronizer a Handler can't prevent
// an event from being published.
type Handler interface {
	ProjectionIDReader

	// Handle is expected to handle the event and return nil.
	// If Handle returns an error its version is not updated and Handle is called
	// again, until it returns nil. Handle is always called by a new goroutine.
	// The orchestrator can't guarantee exactly-once delivery, instead it guarantees
	// at-least-once, meaning that if Handle managed to execute successfuly and
	// returned nil but the system crashed before it could update the handler version
	// then Handle will be called at least twice or more times for this version.
	Handle(ctx context.Context, version int64, e event.Event) error
}

type handlerWithLock struct {
	lock sync.Mutex
	Handler
}

// Orchestrator is the core backbone of the system responsible for synchronizing
// projections, synchronizing handlers and appending events onto the immutable event log.
// Create an instance of the orchestrator using Make and run the dispatcher using
// RunHandlerDispatcher in a new goroutine.
type Orchestrator struct {
	lock sync.Mutex
	wg   sync.WaitGroup
	db   database.Database

	// projectionsByID maps both synchronizers and handlers.
	projectionsByID map[int32]ProjectionIDReader

	synchronizersByID map[int32]ProjectionSynchronizer
	handlersByID      map[int32]*handlerWithLock
	handlerQueue      chan int32
}

// Wait blocks the calling goroutine until all
// asynchronous orchestrator tasks have finished.
func (o *Orchestrator) Wait() { o.wg.Wait() }

// Make creates and initializes a new orchestrator instance.
// Make automatically synchronizes all projection synchronizers to
// the latest available system version blocking until they are up to date.
// Make doesn't synchronize handlers, that is done by RunHandlerDispatcher.
func Make(
	ctx context.Context,
	db database.Database,
	synchronizers []ProjectionSynchronizer,
	handlers []Handler,
) (*Orchestrator, error) {
	o := &Orchestrator{
		db: db,
		projectionsByID: make(map[int32]ProjectionIDReader,
			len(synchronizers)+len(handlers)),
		synchronizersByID: make(map[int32]ProjectionSynchronizer, len(synchronizers)),
		handlersByID:      make(map[int32]*handlerWithLock, len(handlers)),
		handlerQueue:      make(chan int32, len(handlers)*4),
	}

	err := db.TxRW(ctx, func(ctx context.Context, tx *database.Tx) error {
		for i, s := range synchronizers {
			id := s.ProjectionID()
			if _, ok := o.projectionsByID[id]; ok {
				// ID isn't unique.
				return fmt.Errorf("%w (synchronizer index: %d): %d",
					ErrProjectionIDCollision, i, id)
			}
			o.projectionsByID[id] = s
			o.synchronizersByID[id] = s

			if err := initProjectionVersion(ctx, tx, id); err != nil {
				return err
			}
		}

		for i, h := range handlers {
			id := h.ProjectionID()
			if _, ok := o.projectionsByID[id]; ok {
				// ID isn't unique.
				return fmt.Errorf("%w (handler index: %d): %d",
					ErrProjectionIDCollision, i, id)
			}
			o.projectionsByID[id] = h
			o.handlersByID[id] = &handlerWithLock{Handler: h}

			if err := initProjectionVersion(ctx, tx, id); err != nil {
				return err
			}

			// Queue handler for the dispatcher to pick up.
			o.handlerQueue <- id
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	for _, s := range synchronizers {
		for {
			id := s.ProjectionID()
			_, err := o.syncProjectionToNextVersion(ctx, s)
			if errors.Is(err, errNoNextVersion) {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("syncing projection %d: %w", id, err)
			}
		}
	}

	return o, nil
}

// syncProjectionToNextVersion returns errNoNextVersion
// if the synchronizer is already up to date.
func (o *Orchestrator) syncProjectionToNextVersion(
	ctx context.Context, s ProjectionSynchronizer,
) (newVersion int64, err error) {
	err = o.db.TxRW(ctx, func(ctx context.Context, tx *database.Tx) error {
		id := s.ProjectionID()

		v, err := queryProjectionVersion(ctx, tx, id)
		if err != nil {
			return err
		}

		sysVersion, err := o.querySystemVersion(ctx, tx)
		if err != nil {
			return err
		}

		if v >= sysVersion {
			return errNoNextVersion
		}

		v++ // Move to next version.
		ev, err := o.queryEvent(ctx, v)
		if err != nil {
			return fmt.Errorf(
				"querying event at version %d: %w",
				v, err,
			)
		}
		if err := s.Sync(ctx, ev, tx); err != nil {
			return err
		}

		if err := setProjectionVersion(ctx, tx, id, v); err != nil {
			return err
		}
		newVersion = v
		return nil
	})
	return newVersion, err
}

// RunHandlerDispatcher runs the orchestrator dispatcher that calls the handlers.
func (o *Orchestrator) RunHandlerDispatcher(ctx context.Context, log *slog.Logger) error {
	for {
		select {
		case <-ctx.Done():
			// Stop dispatcher.
			return ctx.Err()
		case id := <-o.handlerQueue:
			log := log.With(slog.Int("handler.id", int(id)))
			log.Debug("update handler")
			// TODO: add back-off to avoid spamming when Handler keeps failing.
			o.wg.Add(1)
			// Don't pass ctx to avoid this asynchronous task being canceled.
			go o.syncHandler(context.Background(), log, id)
		}
	}
}

func (o *Orchestrator) syncHandler(
	ctx context.Context, log *slog.Logger, id int32,
) {
	var err error
	var curVer, nextVersion, sysVersion int64

	h := o.handlersByID[id]

	h.lock.Lock()
	defer func() {
		h.lock.Unlock()
		o.wg.Done()
	}()

	err = o.db.TxRW(ctx, func(ctx context.Context, tx *database.Tx) error {
		sysVersion, err = o.querySystemVersion(ctx, tx)
		if err != nil {
			return err
		}

		curVer, err = queryProjectionVersion(ctx, tx, id)
		if err != nil {
			return err
		}

		nextVersion = curVer + 1
		if nextVersion > sysVersion {
			return errNoNextVersion
		}
		return nil
	})
	if errors.Is(err, errNoNextVersion) {
		// Handler is finally up to date.
		log.Debug("handler is up to date",
			slog.Int64("handlerVersion", curVer),
			slog.Int64("systemVersion", sysVersion))
		return
	}
	if err != nil {
		log.Error("syncing handler", slog.Any("err", err))
		// Put the handler back in the queue for retry.
		o.handlerQueue <- id
		return
	}

	// Handler requires update.
	o.syncHandlerToNextVersion(ctx, log, nextVersion, id)
}

func (o *Orchestrator) syncHandlerToNextVersion(
	ctx context.Context, log *slog.Logger, nextVersion int64, id int32,
) {
	h := o.handlersByID[id]

	ev, err := o.queryEvent(ctx, nextVersion)
	if err != nil {
		log.Error("querying event",
			slog.Int64("version", nextVersion),
			slog.Any("err", err))
		return
	}

	if err := h.Handle(ctx, nextVersion, ev); err != nil {
		log.Error("handler error",
			slog.Int("handler.id", int(id)),
			slog.Int64("version", nextVersion),
			slog.Any("err", err))
		return
	}

	// If the system crashes here before it manages to update the projection version
	// Then we'll end up invoking this handler multiple times for this event
	// leading to at-least-once delivery.

	// Update handler version in db.
	err = o.db.TxRW(ctx, func(ctx context.Context, tx *database.Tx) error {
		log.Debug("set handler projection version",
			slog.Int("handler.id", int(id)),
			slog.Int64("version", nextVersion))
		return setProjectionVersion(ctx, tx, id, nextVersion)
	})
	if err != nil {
		log.Error("updating handler version in db", slog.Any("err", err))
		return
	}

	// Put the handler back in the queue for potential finalization.
	o.handlerQueue <- id
}

func (o *Orchestrator) queryEvent(
	ctx context.Context, version int64,
) (event.Event, error) {
	var e string
	err := o.db.QueryRow(ctx, `
		SELECT event from system.events WHERE id=$1
    `, version).Scan(&e)
	if err != nil {
		return nil, fmt.Errorf("querying event by version: %w", err)
	}
	return event.UnmarshalJSON(e)
}

func appendEvent(
	ctx context.Context, tx *database.Tx, e event.Event,
) (newVersion int64, err error) {
	j := event.MustMarshalJSON(e)
	if e.Time().IsZero() {
		return 0, fmt.Errorf("event has zero time: %#v", e)
	}
	err = tx.QueryRow(ctx,
		`INSERT INTO system.events (event, time) VALUES ($1, $2) RETURNING id`,
		j, e.Time(),
	).Scan(&newVersion)
	if err != nil {
		return 0, fmt.Errorf("appending event: %w", err)
	}
	return newVersion, nil
}

func queryProjectionVersion(
	ctx context.Context, tx *database.Tx, id int32,
) (int64, error) {
	var version int64
	err := tx.QueryRow(ctx, `
		SELECT version from system.projection_versions WHERE id=$1
	`, id).Scan(&version)
	if err != nil {
		return 0, fmt.Errorf("querying projection version %d: %w", id, err)
	}
	return version, err
}

func initProjectionVersion(ctx context.Context, tx *database.Tx, id int32) error {
	if _, err := tx.Exec(ctx,
		`
			INSERT INTO system.projection_versions (id, version) VALUES ($1, 0)
			ON CONFLICT (id) DO NOTHING
		`,
		id,
	); err != nil {
		return fmt.Errorf("creating projection_versions row for %d: %w",
			id, err)
	}
	return nil
}

func setProjectionVersion(
	ctx context.Context, tx *database.Tx, id int32, version int64,
) error {
	_, err := tx.Exec(ctx, `
		UPDATE system.projection_versions SET version=$1 WHERE id=$2
	`, version, id)
	if err != nil {
		return fmt.Errorf("setting projection (%d) to version (%d): %w",
			id, version, err)
	}
	return nil
}

// Version returns the current version of the system (id of latest event).
func (o *Orchestrator) Version(ctx context.Context) (version int64, err error) {
	o.lock.Lock()
	defer o.lock.Unlock()
	errTx := o.db.TxReadOnly(ctx, func(ctx context.Context, tx *database.Tx) error {
		version, err = o.querySystemVersion(ctx, tx)
		return err
	})
	return version, errTx
}

func (o *Orchestrator) querySystemVersion(ctx context.Context, tx *database.Tx) (int64, error) {
	var version int64
	err := tx.QueryRow(ctx, `
		SELECT COALESCE(MAX(id), 0) FROM system.events
	`).Scan(&version)
	if err != nil {
		return 0, fmt.Errorf("querying system version: %w", err)
	}
	return version, err
}

// Publish starts a new read-write database transaction,
// calls all synchronizers' Sync method and irreversibly appends e onto the
// system event log if all synchronizations are successful.
// If any Sync call returns an error, the transaction is rolled back,
// the event isn't appended and the error is returned.
func (o *Orchestrator) Publish(
	ctx context.Context, e event.Event,
) (newVersion int64, err error) {
	errTx := o.db.TxRW(ctx, func(ctx context.Context, tx *database.Tx) error {
		newVersion, err = o.PublishWithTx(ctx, e, tx)
		return err
	})
	return newVersion, errTx
}

// PublishWithTx is similar to Publish but doesn't start a transaction
// and instead uses tx.
func (o *Orchestrator) PublishWithTx(
	ctx context.Context, e event.Event, tx *database.Tx,
) (newVersion int64, err error) {
	o.lock.Lock()
	defer o.lock.Unlock()

	if e.Time().IsZero() {
		// If time was missing so far then use current time now.
		e = event.New(time.Now(), e)
	}

	for _, s := range o.synchronizersByID {
		if err := s.Sync(ctx, e, tx); err != nil {
			return 0, fmt.Errorf("synchronizing %T: %w", s, err)
		}
	}

	// All synchronizers are finished. Append e to the immutable event log.
	newVersion, err = appendEvent(ctx, tx, e)
	if err != nil {
		return 0, err
	}

	// Queue all handler for sync.
	for id := range o.handlersByID {
		o.handlerQueue <- id
	}

	return newVersion, nil
}
