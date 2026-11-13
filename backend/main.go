// Command server runs the Geofencing & Real-time Alert System API.
//
// Configuration (environment variables):
//
//	DATABASE_URL        PostgreSQL connection string. When set the server
//	                    uses PostgreSQL + PostGIS; otherwise it falls back to
//	                    an in-memory store (development only).
//	PORT                HTTP listen port (default 8080).
//	RATE_LIMIT_PER_MIN  Max location updates per IP per minute (default 300,
//	                    <= 0 disables the limiter).
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"geofencing/backend/internal/api"
	"geofencing/backend/internal/store"
	"geofencing/backend/internal/ws"
)

func main() {
	logger := log.New(os.Stdout, "", log.LstdFlags|log.Lmsgprefix)
	logger.SetPrefix("geofence-api ")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	st, err := openStore(ctx, logger)
	if err != nil {
		logger.Fatalf("store init: %v", err)
	}
	defer st.Close()

	hub := ws.NewHub()
	go hub.Run()
	defer hub.Stop()

	srv := api.NewServer(st, hub, logger)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	httpServer := &http.Server{
		Addr:              ":" + port,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		logger.Printf("listening on :%s (store: %T)", port, st)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatalf("http server: %v", err)
		}
	}()

	<-ctx.Done()
	logger.Printf("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Printf("shutdown: %v", err)
	}
}

// openStore selects PostgreSQL when DATABASE_URL is set (retrying briefly so
// the server can start alongside its database container) and falls back to
// the in-memory store otherwise.
func openStore(ctx context.Context, logger *log.Logger) (store.Store, error) {
	if url := store.LoadDatabaseURL(); url != "" {
		logger.Printf("using PostgreSQL store")
		return store.NewPostgresStoreWithRetry(ctx, url, 30, 2*time.Second)
	}
	logger.Printf("DATABASE_URL not set - using in-memory store (data is not persisted)")
	return store.NewMemoryStore(), nil
}
