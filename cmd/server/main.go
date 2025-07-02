package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"

	"github.com/romshark/demo-event-sourced-monolith/database"
	"github.com/romshark/demo-event-sourced-monolith/pkg/orchestrator"
	"github.com/romshark/demo-event-sourced-monolith/server"
	"github.com/romshark/demo-event-sourced-monolith/service/email"
	"github.com/romshark/demo-event-sourced-monolith/service/orders"
)

func main() {
	fDebug := flag.Bool("debug", false, "enables debug logging")
	fHost := flag.String("host", "localhost:9090", "host address")
	flag.Parse()

	logLevel := slog.LevelInfo
	if *fDebug {
		logLevel = slog.LevelDebug
	}
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))

	ctx := context.Background()

	if os.Getenv("PGDSN") == "" {
		panic("missing env var PGDSN")
	}

	db, err := database.Open(ctx, log, os.Getenv("PGDSN"), 0)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	serviceOrders := orders.New(log, db)
	serviceEmail := email.New(log)

	orchestrator, err := orchestrator.Make(
		ctx, db,
		[]orchestrator.ProjectionSynchronizer{
			serviceOrders,
		},
		[]orchestrator.Handler{
			serviceEmail,
		})
	if err != nil {
		panic(fmt.Errorf("initializing orchestrator: %w", err))
	}

	srv := server.New(log, serviceOrders)

	httpServer := &http.Server{
		Addr:    *fHost,
		Handler: srv,
	}

	var wg sync.WaitGroup

	ctx, cancel := signal.NotifyContext(ctx)
	defer cancel()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := orchestrator.RunHandlerDispatcher(ctx, log); err != nil {
			if !errors.Is(err, context.Canceled) {
				log.Error("running orchestrator dispatcher", slog.Any("err", err))
			}
		}
		log.Info("orchestrator dispatcher stopped")

		// Wait for the orchestrator to finish its asynchronous tasks.
		orchestrator.Wait()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Info("listening", slog.String("host", *fHost))
		err := httpServer.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			panic(err)
		}
		log.Info("HTTP server shut down")
	}()

	<-ctx.Done() // Await shutdown signal.
	log.Info("shutdown signal received")

	if err := httpServer.Shutdown(context.Background()); err != nil {
		log.Error("shutting down HTTP server", slog.Any("err", err))
	}

	wg.Wait() // Wait for all other system modules to stop gracefuly.
	log.Info("shutdown complete")
}
