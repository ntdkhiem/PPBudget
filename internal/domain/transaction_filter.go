package domain

import "time"

// Date bases for TransactionFilter.DateBasis.
const (
	DateBasisPosted = "posted"
	DateBasisBudget = "budget"
)

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
	// DateBasis says which date StartDate and EndDate bound: the posted date
	// (the default, "" or DateBasisPosted) or the month the transaction counts
	// toward (DateBasisBudget). Ordering and paging always follow the posted
	// date.
	DateBasis string
	Search    string
	Limit     int // 1-100; anything else means the default page size of 100
}
