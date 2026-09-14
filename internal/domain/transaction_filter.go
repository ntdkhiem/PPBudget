package domain

import "time"

type TransactionFilter struct {
	UserID         string
	AccountID      string
	AccountIDs     []string
	CategoryIDs    []string
	Type           string
	CursorDate     *time.Time
	CursorID       *string
	UnreviewedOnly bool
	StartDate      *time.Time
	EndDate        *time.Time
	Search         string
	Limit          int // 1-100; anything else means the default page size of 100
}
