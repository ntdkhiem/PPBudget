package main

import (
	"context"
	"fmt"
	"ntdkhiem/ppbudget-go/internal/config"
	"ntdkhiem/ppbudget-go/internal/repository"
	"ntdkhiem/ppbudget-go/internal/service"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"log/slog"
)

func main() {
	ctx := context.Background()
	cfg := config.Load()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		panic(err)
	}
	defer pool.Close()

	repo := repository.New(pool)
	svc := service.New(repo, logger)

	rules, err := repo.ListRulesDetailed(ctx)
	if err != nil {
		panic(err)
	}

	totalUpdated := 0
	for _, r := range rules {
		count, err := svc.ApplyRule(ctx, r.ID, true, nil, nil)
		if err != nil {
			fmt.Printf("Error applying rule %s: %v\n", r.Name, err)
			continue
		}
		totalUpdated += count
	}

	fmt.Printf("Total updated: %d\n", totalUpdated)
}
