package service

import (
	"testing"
	"time"

	"ntdkhiem/ppbudget-go/internal/domain"
	"ntdkhiem/ppbudget-go/pkg/money"
)

// Pure unit tests -- no database. Table-driven per the rules_*_test.go pattern.
//
// These pin the correctness rules that motivated the feature. Two of them
// describe ways the engine could hand a user a 6% annual excise tax, which is
// why they assert on refusing to answer as much as on answering.

func ptr[T any](v T) *T { return &v }

// testLimits mirrors the shape of a seeded tax year without depending on the
// real figures, so a seed correction never breaks these tests. The relationships
// between the numbers are what matter here, not their absolute values.
func testLimits() domain.TaxLimits {
	return domain.TaxLimits{
		TaxYear: 2026,
		Amounts: map[string]money.Money{
			domain.LimitElectiveDeferral402g:   2_450_000,
			domain.LimitCatchup50Plus:          800_000,
			domain.LimitCatchup60To63:          1_125_000,
			domain.LimitHSASelfOnly:            440_000,
			domain.LimitHSAFamily:              875_000,
			domain.LimitHSACatchup55Plus:       100_000,
			domain.LimitIRAContribution:        750_000,
			domain.LimitIRACatchup50Plus:       110_000,
			domain.LimitTotalAdditions415c:     7_200_000,
			domain.LimitDependentCareFSACap:    750_000,
			domain.LimitSocialSecurityWageBase: 18_450_000,
			domain.LimitRothMAGISingleStart:    15_300_000,
			domain.LimitRothMAGISingleEnd:      16_800_000,
			domain.LimitRothMAGIMFJStart:       24_200_000,
			domain.LimitRothMAGIMFJEnd:         25_200_000,
			"roth_magi_phaseout_mfs_start":     0,
			"roth_magi_phaseout_mfs_end":       1_000_000,
			domain.LimitSafeHarborAGIThreshold: 15_000_000,
			domain.LimitSupplementalThreshold:  100_000_000,
			domain.LimitAddlMedicareSingle:     20_000_000,
			domain.LimitAddlMedicareMFJ:        25_000_000,
		},
		Rates: map[string]float64{
			domain.RateSupplementalLow:    0.22,
			domain.RateSupplementalHigh:   0.37,
			domain.RateFICASocialSecurity: 0.062,
			domain.RateFICAMedicare:       0.0145,
			domain.RateFICAAddlMedicare:   0.009,
			domain.RateSafeHarborStandard: 1.00,
			domain.RateSafeHarborHighAGI:  1.10,
		},
	}
}

func dobForAgeAtYearEnd(age, taxYear int) *time.Time {
	d := time.Date(taxYear-age, time.June, 15, 0, 0, 0, 0, time.UTC)
	return &d
}

// The catch-up ladder is NOT monotonic in age: SECURE 2.0 gives 60-to-63-year
// olds a larger catch-up that reverts to the ordinary one at 64. A naive
// threshold ladder over-states the limit for everyone 64 and older.
func TestElectiveDeferralLimitCatchupTiers(t *testing.T) {
	limits := testLimits()
	const year = 2026

	tests := []struct {
		name         string
		dob          *time.Time
		wantCatchup  money.Money
		catchupKnown bool
	}{
		{"unanswered date of birth yields base only, flagged unknown", nil, 0, false},
		{"age 35: no catch-up", dobForAgeAtYearEnd(35, year), 0, true},
		{"age 49: still no catch-up", dobForAgeAtYearEnd(49, year), 0, true},
		{"age 50: ordinary catch-up begins", dobForAgeAtYearEnd(50, year), 800_000, true},
		{"age 59: still ordinary", dobForAgeAtYearEnd(59, year), 800_000, true},
		{"age 60: super catch-up", dobForAgeAtYearEnd(60, year), 1_125_000, true},
		{"age 63: last super year", dobForAgeAtYearEnd(63, year), 1_125_000, true},
		{"age 64: REVERTS to ordinary", dobForAgeAtYearEnd(64, year), 800_000, true},
		{"age 70: ordinary", dobForAgeAtYearEnd(70, year), 800_000, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ElectiveDeferralLimit(limits, tc.dob, year)
			if got.Catchup != tc.wantCatchup {
				t.Errorf("catch-up: got %d want %d", got.Catchup, tc.wantCatchup)
			}
			if got.CatchupKnown != tc.catchupKnown {
				t.Errorf("CatchupKnown: got %v want %v", got.CatchupKnown, tc.catchupKnown)
			}
			if want := got.Base + got.Catchup; got.Total != want {
				t.Errorf("Total: got %d want %d", got.Total, want)
			}
		})
	}
}

// Catch-up eligibility turns on age at 31 December, so a late-December birthday
// qualifies for the whole year. Using current age would withhold it for most of
// the year from everyone born late in the calendar.
func TestAgeAtYearEndUsesDecember31(t *testing.T) {
	limits := testLimits()
	dec31Birthday := time.Date(1976, time.December, 31, 0, 0, 0, 0, time.UTC)

	got := ElectiveDeferralLimit(limits, &dec31Birthday, 2026)
	if got.Catchup == 0 {
		t.Errorf("someone turning 50 on 31 December 2026 is eligible for the whole year; got no catch-up")
	}
}

// "Max your HSA" is not computable from the HDHP flag alone. Family coverage is
// thousands higher than self-only, and an employer seed reduces the remaining
// room -- either one wrong produces an excess contribution.
func TestHSAContributionRoom(t *testing.T) {
	limits := testLimits()
	const year = 2026

	tests := []struct {
		name           string
		tier           *string
		dob            *time.Time
		employer       money.Money
		contributedYTD money.Money
		wantRemaining  money.Money
		wantComputable bool
	}{
		{
			name: "coverage tier unanswered: refuses to compute",
			tier: nil, wantComputable: false,
		},
		{
			name: "self-only, nothing contributed",
			tier: ptr("self_only"), wantRemaining: 440_000, wantComputable: true,
		},
		{
			name: "family is materially higher than self-only",
			tier: ptr("family"), wantRemaining: 875_000, wantComputable: true,
		},
		{
			name:          "employer seed reduces remaining room",
			tier:          ptr("family"),
			employer:      150_000,
			wantRemaining: 725_000, wantComputable: true,
		},
		{
			name:           "already contributed reduces it further",
			tier:           ptr("family"),
			employer:       150_000,
			contributedYTD: 300_000,
			wantRemaining:  425_000, wantComputable: true,
		},
		{
			name: "55+ catch-up applies",
			tier: ptr("self_only"), dob: dobForAgeAtYearEnd(55, year),
			wantRemaining: 540_000, wantComputable: true,
		},
		{
			name:           "over-contributed reports NEGATIVE room, not zero",
			tier:           ptr("self_only"),
			contributedYTD: 500_000,
			wantRemaining:  -60_000, wantComputable: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := HSAContributionRoom(limits, tc.tier, tc.dob, year, tc.employer, tc.contributedYTD)
			if got.Computable != tc.wantComputable {
				t.Fatalf("Computable: got %v want %v", got.Computable, tc.wantComputable)
			}
			if !tc.wantComputable {
				return
			}
			if got.Remaining != tc.wantRemaining {
				t.Errorf("Remaining: got %d want %d", got.Remaining, tc.wantRemaining)
			}
		})
	}
}

// The flaw that motivated the whole feature. The source plan automated a direct
// Roth contribution in three separate chapters while the user's own income made
// them ineligible -- a 6% excise tax per year until corrected.
func TestEvaluateRothEligibility(t *testing.T) {
	limits := testLimits()

	tests := []struct {
		name        string
		filing      *string
		ownMAGI     money.Money
		spouse      *money.Money
		wantStatus  string
		wantMissing []string
	}{
		{
			name:   "filing status unanswered: unknown, not eligible",
			filing: nil, ownMAGI: 10_000_000,
			wantStatus: RothUnknown, wantMissing: []string{"filing_status"},
		},
		{
			name:   "single, comfortably under: eligible",
			filing: ptr("single"), ownMAGI: 12_000_000,
			wantStatus: RothEligible,
		},
		{
			name:   "single, inside the phase-out: partial",
			filing: ptr("single"), ownMAGI: 16_000_000,
			wantStatus: RothPartial,
		},
		{
			name:   "single, above the phase-out: ineligible",
			filing: ptr("single"), ownMAGI: 17_500_000,
			wantStatus: RothIneligible,
		},
		{
			// The specific trap. Asking filing status while not asking spouse
			// income is worse than asking neither: the household looks eligible
			// on the user's own income alone.
			name:   "MFJ without spouse income: refuses to answer",
			filing: ptr("mfj"), ownMAGI: 18_500_000, spouse: nil,
			wantStatus: RothUnknown, wantMissing: []string{"spouse_gross_annual"},
		},
		{
			name:   "MFJ, household under the joint threshold: eligible",
			filing: ptr("mfj"), ownMAGI: 12_000_000, spouse: ptr(money.Money(8_000_000)),
			wantStatus: RothEligible,
		},
		{
			// Each spouse alone is under the SINGLE threshold; together they are
			// past the JOINT one. Evaluating on own income would say eligible.
			name:   "MFJ, each under alone but household over: ineligible",
			filing: ptr("mfj"), ownMAGI: 14_500_000, spouse: ptr(money.Money(12_000_000)),
			wantStatus: RothIneligible,
		},
		{
			name:   "MFS is effectively ineligible above a very low threshold",
			filing: ptr("mfs"), ownMAGI: 5_000_000,
			wantStatus: RothIneligible,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := EvaluateRothEligibility(limits, tc.filing, tc.ownMAGI, tc.spouse)
			if got.Status != tc.wantStatus {
				t.Errorf("Status: got %q want %q (MAGI %d, window %d-%d)",
					got.Status, tc.wantStatus, got.MAGI, got.Start, got.End)
			}
			if len(tc.wantMissing) > 0 {
				if len(got.MissingFields) != len(tc.wantMissing) {
					t.Fatalf("MissingFields: got %v want %v", got.MissingFields, tc.wantMissing)
				}
				for i, want := range tc.wantMissing {
					if got.MissingFields[i] != want {
						t.Errorf("MissingFields[%d]: got %q want %q", i, got.MissingFields[i], want)
					}
				}
			}
		})
	}
}

// FICA is computed, never asked. The old intake bundled it with income tax,
// which made the combined number useless downstream and impossible to split.
func TestComputeFICA(t *testing.T) {
	limits := testLimits()

	t.Run("social security stops at the wage base", func(t *testing.T) {
		under := ComputeFICA(limits, 10_000_000, ptr("single"))
		over := ComputeFICA(limits, 30_000_000, ptr("single"))

		wantCapped := money.Money(float64(18_450_000) * 0.062)
		if over.SocialSecurity != wantCapped {
			t.Errorf("SS above the base should cap at %d, got %d", wantCapped, over.SocialSecurity)
		}
		if under.SocialSecurity >= over.SocialSecurity && under.SocialSecurity != wantCapped {
			t.Errorf("SS below the base should be uncapped: %d vs %d", under.SocialSecurity, over.SocialSecurity)
		}
	})

	t.Run("medicare is uncapped", func(t *testing.T) {
		f := ComputeFICA(limits, 30_000_000, ptr("single"))
		want := money.Money(float64(30_000_000) * 0.0145)
		if f.Medicare != want {
			t.Errorf("Medicare: got %d want %d", f.Medicare, want)
		}
	})

	t.Run("additional medicare applies only above the threshold", func(t *testing.T) {
		below := ComputeFICA(limits, 15_000_000, ptr("single"))
		if below.Additional != 0 {
			t.Errorf("no surtax expected below the threshold, got %d", below.Additional)
		}
		above := ComputeFICA(limits, 25_000_000, ptr("single"))
		want := money.Money(float64(25_000_000-20_000_000) * 0.009)
		if above.Additional != want {
			t.Errorf("surtax: got %d want %d", above.Additional, want)
		}
	})

	t.Run("the surtax threshold is higher for joint filers", func(t *testing.T) {
		single := ComputeFICA(limits, 22_000_000, ptr("single"))
		joint := ComputeFICA(limits, 22_000_000, ptr("mfj"))
		if single.Additional == 0 {
			t.Error("single filer at 220k should owe the surtax")
		}
		if joint.Additional != 0 {
			t.Errorf("joint filer at 220k is below the joint threshold, got %d", joint.Additional)
		}
	})
}

// Safe harbour turns the equity tax-gap action from "set some money aside" into
// a dated, sized instruction. It cannot be guessed at.
func TestSafeHarborTarget(t *testing.T) {
	limits := testLimits()

	t.Run("refuses without prior-year figures", func(t *testing.T) {
		got := SafeHarborTarget(limits, nil, nil)
		if got.Computable {
			t.Error("should not be computable with no prior-year data")
		}
		if len(got.MissingFields) != 2 {
			t.Errorf("should name both missing fields, got %v", got.MissingFields)
		}
	})

	t.Run("standard rate below the AGI threshold", func(t *testing.T) {
		got := SafeHarborTarget(limits, ptr(money.Money(4_000_000)), ptr(money.Money(12_000_000)))
		if !got.Computable || got.Rate != 1.00 {
			t.Fatalf("expected the standard rate, got %+v", got)
		}
		if got.Target != 4_000_000 {
			t.Errorf("Target: got %d want 4000000", got.Target)
		}
	})

	t.Run("higher rate above the AGI threshold", func(t *testing.T) {
		got := SafeHarborTarget(limits, ptr(money.Money(4_000_000)), ptr(money.Money(20_000_000)))
		if got.Rate != 1.10 {
			t.Fatalf("expected the high-AGI rate, got %v", got.Rate)
		}
		if got.Target != 4_400_000 {
			t.Errorf("Target: got %d want 4400000", got.Target)
		}
	})
}

// The 22%-to-37% step happens once aggregate supplemental wages cross the
// threshold WITHIN the year, so it is a dated event rather than a fixed
// property of the user.
func TestSupplementalWithholdingRateCrossover(t *testing.T) {
	limits := testLimits()

	if got := SupplementalWithholdingRate(limits, 5_000_000); got != 0.22 {
		t.Errorf("below the threshold: got %v want 0.22", got)
	}
	if got := SupplementalWithholdingRate(limits, 150_000_000); got != 0.37 {
		t.Errorf("above the threshold: got %v want 0.37", got)
	}
}
