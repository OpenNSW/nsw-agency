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

	"github.com/OpenNSW/nsw-agency/backend/internal/application"
	"github.com/OpenNSW/nsw-agency/backend/internal/auth"
	"github.com/OpenNSW/nsw-agency/backend/internal/feedback"
	"github.com/OpenNSW/nsw-agency/backend/internal/form"
	"github.com/OpenNSW/nsw-agency/backend/internal/storage"
	"github.com/OpenNSW/nsw-agency/backend/internal/taskconfig"
	"github.com/OpenNSW/nsw-agency/backend/internal/user"
	"github.com/OpenNSW/nsw-agency/backend/pkg/httpclient"
)

func main() {
	cfg, err := LoadConfig()
	if err != nil {
		log.Fatalf("FATAL: failed to load configuration: %v", err)
	}

	slog.Info("NSW Agency service configuration",
		"db_driver", cfg.DB.Driver,
		"db_path", cfg.DB.Path,
		"port", cfg.Port,
		"config_dir", cfg.ConfigDir,
	)

	// Initialize database store
	store, err := application.NewApplicationStore(cfg.DB)
	if err != nil {
		log.Fatalf("failed to create application store: %v", err)
	}

	// Initialize user store
	userStore, err := user.NewUserStore(cfg)
	if err != nil {
		log.Fatalf("failed to create user store: %v", err)
	}
	defer func() {
		if err := userStore.Close(); err != nil {
			slog.Error("failed to close user store", "error", err)
		}
	}()

	// Initialize auth manager with JIT user provisioning
	authManager, err := auth.NewManager(userStore, cfg.Auth)
	if err != nil {
		log.Fatalf("failed to initialize auth manager: %v", err)
	}
	defer func() {
		if err := authManager.Close(); err != nil {
			slog.Error("failed to close auth manager", "error", err)
		}
	}()

	// Initialize task config store
	configStore, err := taskconfig.NewTaskConfigStore(cfg.ConfigDir, cfg.DefaultTaskConfigID)
	if err != nil {
		log.Fatalf("failed to create task config store: %v", err)
	}
	// Initialize form store
	formStore, err := form.NewFormStore(cfg.ConfigDir)
	if err != nil {
		log.Fatalf("failed to create form store: %v", err)
	}

	// Create OAuth2 Authenticator for NSW API
	nswOAuth2Client := httpclient.NewOAuth2Authenticator(
		cfg.NSW.ClientID,
		cfg.NSW.ClientSecret,
		cfg.NSW.TokenURL,
		cfg.NSW.Scopes,
	)

	// Initialize HTTP client for NSW API integration with optional TLS configuration
	nswHttpClient := httpclient.NewClientBuilder().
		WithBaseURL(cfg.NSW.BaseURL).
		WithTimeout(10 * time.Second).
		WithAuthenticator(nswOAuth2Client).
		WithTLS(&httpclient.TLSConfig{InsecureSkipVerify: cfg.NSW.TokenInsecureSkipVerify}).
		Build()

	// Initialize Agency service
	service := application.NewService(store, configStore, formStore, nswHttpClient)
	defer func() {
		if err := service.Close(); err != nil {
			slog.Error("failed to close service", "error", err)
		}
	}()

	// Initialize handlers
	handler, err := application.NewHandler(service, cfg.MaxRequestBytes)
	if err != nil {
		slog.Error("failed to create Agency handler", "error", err)
		return
	}

	// Initialize storage service and handler
	storageService := storage.NewService(nswHttpClient)
	storageHandler := storage.NewHandler(storageService, cfg.MaxRequestBytes)

	feedbackHandler := feedback.NewHandler(service)

	// Set up HTTP routes
	mux := http.NewServeMux()
	// Health check
	mux.HandleFunc("GET /health", handler.HandleHealth)

	// Endpoint for services to inject data (service-to-service, no user auth)
	mux.HandleFunc("POST /api/v1/inject", handler.HandleInjectData)

	// Endpoints for UI to fetch and manage applications (protected by JIT user auth)
	protect := authManager.RequireAuthMiddleware()
	mux.Handle("GET /api/v1/consignments", protect(http.HandlerFunc(handler.HandleGetConsignments)))
	mux.Handle("GET /api/v1/applications", protect(http.HandlerFunc(handler.HandleGetApplications)))
	mux.Handle("GET /api/v1/applications/{taskId}", protect(http.HandlerFunc(handler.HandleGetApplication)))
	mux.Handle("POST /api/v1/applications/{taskId}/review", protect(http.HandlerFunc(handler.HandleReviewApplication)))
	mux.Handle("POST /api/v1/applications/{taskId}/feedback", protect(http.HandlerFunc(feedbackHandler.HandleFeedback)))
	mux.Handle("POST /api/v1/storage", protect(http.HandlerFunc(storageHandler.HandleCreateUpload)))
	mux.Handle("GET /api/v1/storage/{key}", protect(http.HandlerFunc(storageHandler.HandleGetUploadURL)))

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
		slog.Info("starting Agency service", "port", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("failed to start server", "error", err)
			quit <- syscall.SIGTERM
		}
	}()

	// Wait for interrupt signal
	<-quit
	slog.Info("shutting down Agency service...")

	// Create a context with timeout for graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Attempt graceful shutdown of HTTP server
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
	} else {
		slog.Info("server gracefully stopped")
	}

	slog.Info("NSW Agency service stopped")
}
