package main
import (
	"context"
	"fmt"
	"ntdkhiem/ppbudget-go/internal/config"
	"ntdkhiem/ppbudget-go/internal/db"
	"ntdkhiem/ppbudget-go/internal/repository"
)
func main() {
	cfg := config.Load()
	cfg.MustLoad()
	ctx := context.Background()
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil { panic(err) }
	repo := repository.New(pool)
	_, err = repo.GetTransactionsByDateRange(ctx, nil, nil)
	fmt.Println("Error:", err)
}
