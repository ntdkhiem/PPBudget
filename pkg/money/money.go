package money

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Money represents an amount in cents to prevent floating point issues.
type Money int64

func NewFromString(s string) (Money, error) {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, fmt.Errorf("invalid money format: %w", err)
	}
	return Money(math.Round(f * 100)), nil
}

func (m Money) ToInt64() int64 {
	return int64(m)
}

func (m Money) String() string {
	abs := math.Abs(float64(m))
	return fmt.Sprintf("%.2f", abs/100)
}
