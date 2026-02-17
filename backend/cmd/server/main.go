package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/OpenNSW/nsw/oga/internal"
)

func main() {
	cfg := internal.LoadConfig()

	slog.Info("OGA service configuration",
		"db_path", cfg.DBPath,
		"port", cfg.Port,
		"forms_path", cfg.FormsPath,
	)

	// Initialize database store
	store, err := internal.NewApplicationStore(cfg.DBPath)
	if err != nil {
		log.Fatalf("failed to create application store: %v", err)
	}
	// Initialize form store
	formStore, err := internal.NewFormStore(cfg.FormsPath, cfg.DefaultFormID)
	if err != nil {
		log.Fatalf("failed to create form store: %v", err)
	}

	// Initialize OGA service
	service := internal.NewOGAService(store, formStore)
	defer func() {
		if err := service.Close(); err != nil {
			slog.Error("failed to close service", "error", err)
		}
	}()

	// Initialize handler
	handler := internal.NewOGAHandler(service)

	// Set up HTTP routes
	mux := http.NewServeMux()
	// Health check
	mux.HandleFunc("GET /health", handler.HandleHealth)
	// Endpoint for services to inject data
	mux.HandleFunc("POST /api/oga/inject", handler.HandleInjectData)
	// Endpoints for UI to fetch and manage applications
	mux.HandleFunc("GET /api/oga/applications", handler.HandleGetApplications)
	mux.HandleFunc("GET /api/oga/applications/{taskId}", handler.HandleGetApplication)
	mux.HandleFunc("POST /api/oga/applications/{taskId}/review", handler.HandleReviewApplication)

	// Set up graceful shutdown
	serverAddr := fmt.Sprintf(":%s", cfg.Port)

	// CORS middleware
	allowAll := len(cfg.AllowedOrigins) == 1 && cfg.AllowedOrigins[0] == "*"
	allowedSet := make(map[string]struct{}, len(cfg.AllowedOrigins))
	for _, o := range cfg.AllowedOrigins {
		allowedSet[o] = struct{}{}
	}

	corsHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if allowAll {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		} else if _, ok := allowedSet[origin]; ok {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		mux.ServeHTTP(w, r)
	})

	server := &http.Server{
		Addr:    serverAddr,
		Handler: corsHandler,
	}

	// Channel to listen for interrupt signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Start server in a goroutine
	go func() {
		slog.Info("starting OGA service", "port", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("failed to start server", "error", err)
			quit <- syscall.SIGTERM
		}
	}()

	// Wait for interrupt signal
	<-quit
	slog.Info("shutting down OGA service...")

	// Create a context with timeout for graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Attempt graceful shutdown of HTTP server
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
	} else {
		slog.Info("server gracefully stopped")
	}

	slog.Info("OGA service stopped")
}
