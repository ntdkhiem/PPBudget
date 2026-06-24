package main

import (
	"context"
	"fmt"
	"ntdkhiem/ppbudget-go/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	ctx := context.Background()
	cfg := config.Load()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		panic(err)
	}
	defer pool.Close()

	_, err = pool.Exec(ctx, "ALTER TABLE transactions ADD COLUMN linked_transaction_id UUID REFERENCES transactions(id) ON DELETE SET NULL;")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Println("Success")
	}
}
