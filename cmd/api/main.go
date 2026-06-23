package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ntdkhiem/firefly-go/internal/config"
	"ntdkhiem/firefly-go/internal/db"
	"ntdkhiem/firefly-go/internal/handler"
	"ntdkhiem/firefly-go/internal/middleware"
	"ntdkhiem/firefly-go/internal/repository"
	"ntdkhiem/firefly-go/internal/service"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg := config.Load()
	cfg.MustLoad()

	// 1. Init DB Pool
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// 2. Init layers
	repo := repository.New(pool)
	svc := service.New(repo, logger)
	h := handler.New(svc, logger, cfg)

	// 3. Setup Router
	r := chi.NewRouter()

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000"}, // Next.js dev server
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-API-Key"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	// Global Middleware
	r.Use(chimw.RequestID)
	r.Use(middleware.StructuredLogger(logger))
	r.Use(chimw.Recoverer)

	// Routes
	r.Route("/api/v1", func(r chi.Router) {
		// Public route
		r.Post("/auth/login", h.Login)

		// Secure ingestion endpoint (Machine-to-Machine)
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAPIKey(cfg.IngestAPIKey))
			r.Post("/ingest", h.Ingest)
		})

		// Secure user routes (Dashboard)
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireJWT(cfg.JWTSecret))
			
			// Accounts
			r.Get("/accounts", h.ListAccounts)
			r.Post("/accounts", h.CreateAccount)
			r.Get("/accounts/{id}", h.GetAccount)
			r.Put("/accounts/{id}", h.UpdateAccount)
			r.Delete("/accounts/{id}", h.DeleteAccount)

			// Transactions
			r.Get("/transactions", h.ListTransactions)
			r.Post("/transactions", h.CreateTransaction)
			r.Put("/transactions/{id}", h.UpdateTransaction)
			r.Delete("/transactions/{id}", h.DeleteTransaction)
			r.Patch("/transactions/{id}/review", h.ReviewTransaction)

			// Transfers
			r.Post("/transfers", h.CreateTransfer)

			// Categories
			r.Get("/categories", h.ListCategories)
			r.Post("/categories", h.CreateCategory)
			r.Put("/categories/{id}", h.UpdateCategory)
			r.Delete("/categories/{id}", h.DeleteCategory)

			// Rules
			r.Get("/rules", h.ListRules)
			r.Post("/rules", h.CreateRule)
			r.Get("/rules/{id}", h.GetRule)
			r.Put("/rules/{id}", h.UpdateRule)
			r.Delete("/rules/{id}", h.DeleteRule)
			r.Post("/rules/{id}/apply", h.ApplyRule)

			// Budgets
			r.Get("/budgets/summary", h.GetBudgetsSummary)
			r.Post("/budgets", h.CreateBudget)
			r.Put("/budgets/{id}", h.UpdateBudget)
			r.Delete("/budgets/{id}", h.DeleteBudget)

			// Reports
			r.Get("/reports/summary", h.GetReportsSummary)
			r.Get("/reports/net-worth", h.GetNetWorthTrend)
			r.Get("/reports/spending", h.GetSpendingByCategory)

			// Subscriptions
			r.Get("/subscriptions", h.ListSubscriptions)
			r.Post("/subscriptions", h.CreateSubscription)
			r.Delete("/subscriptions/{id}", h.DeleteSubscription)
		})
	})

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// 4. Start Server with Graceful Shutdown
	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	go func() {
		logger.Info("server starting", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server failed to start", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("server shutting down...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server forced to shutdown", "error", err)
	}

	logger.Info("server exiting")
}
