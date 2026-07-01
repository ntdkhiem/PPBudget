package main

import (
	"context"
	"fmt"
	"ntdkhiem/ppbudget-go/internal/repository"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	pool, err := pgxpool.New(context.Background(), "postgres://ppbudget:ppbudget_password@localhost:5432/ppbudget?sslmode=disable")
	if err != nil {
		panic(err)
	}
	repo := repository.New(pool)
	
	month, _ := time.Parse("2006-01", "2026-07")
	summaries, err := repo.GetBudgetsSummary(context.Background(), month)
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
	} else {
		fmt.Printf("Success, got %d summaries\n", len(summaries))
	}
}
