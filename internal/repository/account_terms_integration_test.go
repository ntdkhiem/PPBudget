package repository

// Gated on TEST_DATABASE_URL; see balances_integration_test.go for the helpers
// and the throwaway-database warning.

import (
	"context"
	"errors"
	"testing"
	"time"

	"ntdkhiem/ppbudget-go/internal/domain"
	apperrors "ntdkhiem/ppbudget-go/internal/errors"
)

// A save names the terms it changes. Replacing the whole row let setup's APR
// wipe a card's promotional rate, so these pin that a partial save leaves the
// rest alone and that an explicit null is what clears a term.
func TestAccountTermsPatchKeepsWhatItDoesNotNameIntegration(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	repo := New(pool)
	userID := createTestUser(t, pool, "terms-patch")

	cardID, err := repo.CreateAccount(ctx, userID, "Patch Test Card", "liability", "USD", 0, nil)
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	expiry := time.Date(2027, 3, 1, 0, 0, 0, 0, time.UTC)
	read := func() domain.AccountTerms {
		t.Helper()
		all, err := repo.ListAccountTerms(ctx, userID)
		if err != nil {
			t.Fatalf("ListAccountTerms: %v", err)
		}
		return all[cardID]
	}

	if err := repo.PatchAccountTerms(ctx, userID, domain.AccountTerms{
		AccountID: cardID, APR: ptr(0.2249), PromoAPR: ptr(0.0), PromoExpiresOn: &expiry, DueDay: ptr(14),
	}, []string{"apr", "promo_apr", "promo_expires_on", "due_day"}); err != nil {
		t.Fatalf("first save: %v", err)
	}

	// Setup's rate screen sends the APR alone.
	if err := repo.PatchAccountTerms(ctx, userID,
		domain.AccountTerms{AccountID: cardID, APR: ptr(0.2599)}, []string{"apr"}); err != nil {
		t.Fatalf("APR-only save: %v", err)
	}
	got := read()
	if got.APR == nil || *got.APR != 0.2599 {
		t.Errorf("apr: got %v want 0.2599", got.APR)
	}
	if got.PromoAPR == nil || got.PromoExpiresOn == nil || !got.PromoExpiresOn.Equal(expiry) {
		t.Errorf("the promotional rate should survive an APR-only save; got %v until %v", got.PromoAPR, got.PromoExpiresOn)
	}
	if got.DueDay == nil || *got.DueDay != 14 {
		t.Errorf("due day should survive; got %v", got.DueDay)
	}

	// The Go-side upsert writes what it holds and nothing else.
	if err := repo.UpsertAccountTerms(ctx, userID, domain.AccountTerms{AccountID: cardID, APR: ptr(0.2)}); err != nil {
		t.Fatalf("UpsertAccountTerms: %v", err)
	}
	if got := read(); got.PromoAPR == nil || got.DueDay == nil {
		t.Errorf("UpsertAccountTerms should not clear terms it does not hold; got %+v", got)
	}

	// Changing one half of the promotion is fine while the row holds the other.
	// An upsert checked its proposed insert row -- no date -- and refused this.
	if err := repo.PatchAccountTerms(ctx, userID,
		domain.AccountTerms{AccountID: cardID, PromoAPR: ptr(0.029)}, []string{"promo_apr"}); err != nil {
		t.Fatalf("changing only the promo rate: %v", err)
	}
	if got := read(); got.PromoAPR == nil || *got.PromoAPR != 0.029 || got.PromoExpiresOn == nil {
		t.Errorf("promo rate should change and keep its date; got %v until %v", got.PromoAPR, got.PromoExpiresOn)
	}

	// Naming half of the promotion is refused with a readable reason.
	err = repo.PatchAccountTerms(ctx, userID, domain.AccountTerms{AccountID: cardID}, []string{"promo_apr"})
	if !errors.Is(err, apperrors.ErrInvalidInput) {
		t.Errorf("clearing the promo rate without its date should be invalid input, got %v", err)
	}

	// Explicit nulls clear, and only what they name.
	if err := repo.PatchAccountTerms(ctx, userID, domain.AccountTerms{AccountID: cardID},
		[]string{"promo_apr", "promo_expires_on"}); err != nil {
		t.Fatalf("clearing the promotion: %v", err)
	}
	got = read()
	if got.PromoAPR != nil || got.PromoExpiresOn != nil {
		t.Errorf("promotion should be cleared; got %v until %v", got.PromoAPR, got.PromoExpiresOn)
	}
	if got.APR == nil || *got.APR != 0.2 {
		t.Errorf("clearing the promotion should leave the APR; got %v", got.APR)
	}

	if err := repo.PatchAccountTerms(ctx, userID, domain.AccountTerms{AccountID: cardID}, []string{"bogus"}); !errors.Is(err, apperrors.ErrInvalidInput) {
		t.Errorf("an unknown term should be invalid input, got %v", err)
	}
}
