package main

import (
	"encoding/json"
	"fmt"
	"time"
)

type Money int64

type TransactionLink struct {
	TransactionID string    `json:"transaction_id"`
	Amount        Money     `json:"amount"`
	Description   string    `json:"description,omitempty"`
	Date          time.Time `json:"date,omitempty"`
}

func main() {
	raw := `[{"transaction_id" : "01cc9dbf-2dae-45f1-a781-72638c8a85f5", "amount" : 54420, "description" : "CL *Chase Travel", "date" : "2026-07-01"}, {"transaction_id" : "d1c9aa25-383a-46f9-afd6-3a79701084aa", "amount" : 27440, "description" : "CL *Chase Travel", "date" : "2026-07-01"}, {"transaction_id" : "eb890372-6fdb-4cac-9096-adda0747e388", "amount" : 29500, "description" : "AUTOZIPPER", "date" : "2026-07-01"}]`

	var links []TransactionLink
	err := json.Unmarshal([]byte(raw), &links)
	if err != nil {
		fmt.Printf("Unmarshal Error: %v\n", err)
	}
	fmt.Printf("Parsed %d links\n", len(links))
}
