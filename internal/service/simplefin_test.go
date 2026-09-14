package service

import (
	"encoding/json"
	"testing"
	"time"
)

func TestSnapshotFromSFAccount(t *testing.T) {
	now := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		acc  SFAccount

		wantBalance      int64
		wantAvailableNil bool
		wantAvailable    int64
		wantAsOf         time.Time
		wantReportedAt   time.Time
		wantErr          bool
	}{
		{
			name: "positive balance with balance-date",
			acc: SFAccount{
				Balance:     "1234.56",
				BalanceDate: time.Date(2026, 9, 10, 15, 30, 0, 0, time.UTC).Unix(),
			},
			wantBalance:    123456,
			wantAsOf:       time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC),
			wantReportedAt: time.Date(2026, 9, 10, 15, 30, 0, 0, time.UTC),
		},
		{
			name: "negative balance (credit card)",
			acc: SFAccount{
				Balance:     "-1234.56",
				BalanceDate: time.Date(2026, 9, 10, 15, 30, 0, 0, time.UTC).Unix(),
			},
			wantBalance:    -123456,
			wantAsOf:       time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC),
			wantReportedAt: time.Date(2026, 9, 10, 15, 30, 0, 0, time.UTC),
		},
		{
			name: "balance-date zero falls back to now",
			acc: SFAccount{
				Balance:     "500.00",
				BalanceDate: 0,
			},
			wantBalance:    50000,
			wantAsOf:       time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC),
			wantReportedAt: now,
		},
		{
			name: "balance-date late in UTC day maps to correct UTC date",
			acc: SFAccount{
				Balance:     "10.00",
				BalanceDate: time.Date(2026, 9, 10, 23, 59, 59, 0, time.UTC).Unix(),
			},
			wantBalance:    1000,
			wantAsOf:       time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC),
			wantReportedAt: time.Date(2026, 9, 10, 23, 59, 59, 0, time.UTC),
		},
		{
			name: "empty balance is an error",
			acc: SFAccount{
				Balance:     "",
				BalanceDate: time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC).Unix(),
			},
			wantErr: true,
		},
		{
			name: "unparseable balance is an error",
			acc: SFAccount{
				Balance:     "not-a-number",
				BalanceDate: time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC).Unix(),
			},
			wantErr: true,
		},
		{
			name: "available-balance present",
			acc: SFAccount{
				Balance:          "1000.00",
				AvailableBalance: "800.00",
				BalanceDate:      time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC).Unix(),
			},
			wantBalance:   100000,
			wantAvailable: 80000,
			wantAsOf:      time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "available-balance absent",
			acc: SFAccount{
				Balance:     "1000.00",
				BalanceDate: time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC).Unix(),
			},
			wantBalance:      100000,
			wantAvailableNil: true,
			wantAsOf:         time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "available-balance garbage is ignored, not an error",
			acc: SFAccount{
				Balance:          "1000.00",
				AvailableBalance: "garbage",
				BalanceDate:      time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC).Unix(),
			},
			wantBalance:      100000,
			wantAvailableNil: true,
			wantAsOf:         time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			balance, available, asOf, reportedAt, err := snapshotFromSFAccount(tt.acc, now)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got none (balance=%v)", balance)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if balance.ToInt64() != tt.wantBalance {
				t.Errorf("balance = %d, want %d", balance.ToInt64(), tt.wantBalance)
			}

			if tt.wantAvailableNil {
				if available != nil {
					t.Errorf("available = %v, want nil", available)
				}
			} else if tt.wantAvailable != 0 {
				if available == nil {
					t.Fatalf("available = nil, want %d", tt.wantAvailable)
				}
				if available.ToInt64() != tt.wantAvailable {
					t.Errorf("available = %d, want %d", available.ToInt64(), tt.wantAvailable)
				}
			}

			if !tt.wantAsOf.IsZero() && !asOf.Equal(tt.wantAsOf) {
				t.Errorf("asOf = %v, want %v", asOf, tt.wantAsOf)
			}
			if asOf.Location() != time.UTC {
				t.Errorf("asOf location = %v, want UTC", asOf.Location())
			}

			if !tt.wantReportedAt.IsZero() && !reportedAt.Equal(tt.wantReportedAt) {
				t.Errorf("reportedAt = %v, want %v", reportedAt, tt.wantReportedAt)
			}
			if reportedAt.Location() != time.UTC {
				t.Errorf("reportedAt location = %v, want UTC", reportedAt.Location())
			}
		})
	}
}

func TestSFAccountUnmarshal(t *testing.T) {
	raw := `{
		"id": "ACT-12345",
		"name": "Fidelity Brokerage",
		"currency": "USD",
		"balance": "45678.90",
		"balance-date": 1757512345,
		"available-balance": "1234.56",
		"transactions": []
	}`

	var acc SFAccount
	if err := json.Unmarshal([]byte(raw), &acc); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if acc.ID != "ACT-12345" {
		t.Errorf("ID = %q, want %q", acc.ID, "ACT-12345")
	}
	if acc.Balance != "45678.90" {
		t.Errorf("Balance = %q, want %q", acc.Balance, "45678.90")
	}
	if acc.BalanceDate != 1757512345 {
		t.Errorf("BalanceDate = %d, want %d", acc.BalanceDate, 1757512345)
	}
	if acc.AvailableBalance != "1234.56" {
		t.Errorf("AvailableBalance = %q, want %q", acc.AvailableBalance, "1234.56")
	}
}
