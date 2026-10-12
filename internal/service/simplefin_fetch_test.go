package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// day is defined in budget_month_test.go.

func TestSimplefinWindows(t *testing.T) {
	now := time.Date(2026, time.October, 12, 15, 0, 0, 0, time.UTC)

	type w struct{ start, end string }
	format := func(ws []fetchWindow) []w {
		out := make([]w, len(ws))
		for i, x := range ws {
			f := func(t time.Time) string {
				if t.IsZero() {
					return "-"
				}
				return t.Format(time.DateOnly)
			}
			out[i] = w{f(x.start), f(x.end)}
		}
		return out
	}

	tests := []struct {
		name  string
		start time.Time
		want  []w
	}{
		{"no start date: one request as before", time.Time{}, []w{{"-", "-"}}},
		{"start in the future: one request", day(2026, time.November, 1), []w{{"2026-11-01", "-"}}},
		{"30 days: one open-ended window", now.AddDate(0, 0, -30).Truncate(24 * time.Hour), []w{{"2026-09-12", "-"}}},
		// Exactly one window long still fits in one request.
		{"85 days to the minute", now.AddDate(0, 0, -simplefinWindowDays), []w{{"2026-07-19", "-"}}},
		{"Jan 1 to Oct 12: four windows, each end the next start", day(2026, time.January, 1), []w{
			{"2026-01-01", "2026-03-27"},
			{"2026-03-27", "2026-06-20"},
			{"2026-06-20", "2026-09-13"},
			{"2026-09-13", "-"},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := format(simplefinWindows(tt.start, now))
			if fmt.Sprint(got) != fmt.Sprint(tt.want) {
				t.Errorf("windows = %v, want %v", got, tt.want)
			}
		})
	}
}

// fakeBridge stands in for SimpleFIN Bridge: it returns the transactions in
// [start-date, end-date), and like the real one, given a range over 90 days it
// keeps only the latest 90 and says so in `errors`.
type fakeBridge struct {
	mu       sync.Mutex
	now      time.Time
	txns     map[string][]sfTransaction // account id -> all of its transactions
	errlist  []sfError
	failCall int // 1-based request number to answer with a 500; 0 never
	requests []fetchWindow
	// pendingAsked records, per request, whether it sent pending=1. As the
	// protocol says, pending transactions are left out unless it did.
	pendingAsked []bool
}

func (b *fakeBridge) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b.mu.Lock()
	defer b.mu.Unlock()

	parse := func(key string) time.Time {
		if v := r.URL.Query().Get(key); v != "" {
			n, _ := strconv.ParseInt(v, 10, 64)
			return time.Unix(n, 0).UTC()
		}
		return time.Time{}
	}
	start, end := parse("start-date"), parse("end-date")
	b.requests = append(b.requests, fetchWindow{start: start, end: end})
	withPending := r.URL.Query().Get("pending") == "1"
	b.pendingAsked = append(b.pendingAsked, withPending)
	if b.failCall == len(b.requests) {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	resp := sfResponse{ErrList: b.errlist}
	to := end
	if to.IsZero() {
		to = b.now
	}
	if !start.IsZero() && to.Sub(start) > 90*24*time.Hour {
		start = to.AddDate(0, 0, -90)
		resp.Errors = append(resp.Errors, "Date range was limited to 90 days")
	}

	ids := make([]string, 0, len(b.txns))
	for id := range b.txns {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		// The balance is the request number, so the merge's choice of which
		// window's details to keep is visible.
		acc := SFAccount{ID: id, Name: "Account " + id, Currency: "USD", Balance: strconv.Itoa(len(b.requests))}
		for _, txn := range b.txns[id] {
			if txn.Pending && !withPending {
				continue
			}
			posted := txn.date(b.now)
			if (start.IsZero() || !posted.Before(start)) && (end.IsZero() || posted.Before(end)) {
				acc.Transactions = append(acc.Transactions, txn)
			}
		}
		resp.Accounts = append(resp.Accounts, acc)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func posted(id string, t time.Time) sfTransaction {
	return sfTransaction{ID: id, Posted: t.Unix(), Amount: "-1.00", Description: id}
}

func newFakeBridge(t *testing.T, b *fakeBridge) string {
	t.Helper()
	srv := httptest.NewServer(b)
	t.Cleanup(srv.Close)
	// The access URL carries credentials, as SimpleFIN's do.
	return strings.Replace(srv.URL, "http://", "http://user:pass@", 1)
}

func TestFetchSimplefinAccountsWindowsAndMerges(t *testing.T) {
	now := time.Date(2026, time.October, 12, 15, 0, 0, 0, time.UTC)
	bridge := &fakeBridge{
		now: now,
		txns: map[string][]sfTransaction{
			"checking": {
				posted("jan", day(2026, time.January, 5)),
				posted("feb", day(2026, time.February, 10)),
				posted("boundary", day(2026, time.March, 27)), // exactly a window edge
				posted("apr", day(2026, time.April, 2)),
				posted("jul", day(2026, time.July, 15)),
				posted("oct", day(2026, time.October, 1)),
			},
			"card": {posted("card-mar", day(2026, time.March, 1))},
		},
		errlist: []sfError{{Code: "con.auth", Msg: "Reconnect Example Bank", ConnID: "c1"}},
	}
	accessURL := newFakeBridge(t, bridge)

	accounts, warnings, err := fetchSimplefinAccounts(context.Background(), http.DefaultClient, accessURL, day(2026, time.January, 1), now, false)
	if err != nil {
		t.Fatalf("fetchSimplefinAccounts: %v", err)
	}

	if len(bridge.requests) != 4 {
		t.Fatalf("made %d requests, want 4: %v", len(bridge.requests), bridge.requests)
	}
	for i, r := range bridge.requests {
		if r.end.IsZero() != (i == 3) {
			t.Errorf("request %d end-date = %v; only the last request should leave it off", i+1, r.end)
		}
	}

	byID := map[string]SFAccount{}
	for _, a := range accounts {
		byID[a.ID] = a
	}
	if len(accounts) != 2 {
		t.Fatalf("got %d accounts, want 2", len(accounts))
	}
	var got []string
	for _, txn := range byID["checking"].Transactions {
		got = append(got, txn.ID)
	}
	sort.Strings(got)
	if want := "[apr boundary feb jan jul oct]"; fmt.Sprint(got) != want {
		t.Errorf("checking transactions = %v, want %s (all of them, none twice)", got, want)
	}
	if n := len(byID["card"].Transactions); n != 1 {
		t.Errorf("card transactions = %d, want 1", n)
	}
	if b := byID["checking"].Balance; b != "4" {
		t.Errorf("balance = %s, want the newest window's (4)", b)
	}

	// Asked for in windows, nothing was truncated; the bank message comes
	// through, once, though every response carried it.
	if fmt.Sprint(warnings) != "[Reconnect Example Bank]" {
		t.Errorf("warnings = %v, want [Reconnect Example Bank]", warnings)
	}
}

// One request for the whole range is what the importer used to make: the
// bridge truncates it, and that warning is now reported rather than dropped.
func TestFetchSimplefinAccountsReportsTruncation(t *testing.T) {
	now := time.Date(2026, time.October, 12, 15, 0, 0, 0, time.UTC)
	bridge := &fakeBridge{now: now, txns: map[string][]sfTransaction{"checking": {posted("jan", day(2026, time.January, 5))}}}
	accessURL := newFakeBridge(t, bridge)

	data, err := getSimplefinAccounts(context.Background(), http.DefaultClient,
		fmt.Sprintf("%s/accounts?start-date=%d", accessURL, day(2026, time.January, 1).Unix()))
	if err != nil {
		t.Fatalf("getSimplefinAccounts: %v", err)
	}
	if len(data.Accounts[0].Transactions) != 0 {
		t.Errorf("single request returned January; the fake should have truncated it")
	}
	if fmt.Sprint(data.messages()) != "[Date range was limited to 90 days]" {
		t.Errorf("messages = %v, want the truncation notice", data.messages())
	}
}

func TestFetchSimplefinAccountsFailsWholeRange(t *testing.T) {
	now := time.Date(2026, time.October, 12, 15, 0, 0, 0, time.UTC)
	bridge := &fakeBridge{now: now, txns: map[string][]sfTransaction{"checking": nil}, failCall: 2}
	accessURL := newFakeBridge(t, bridge)

	accounts, _, err := fetchSimplefinAccounts(context.Background(), http.DefaultClient, accessURL, day(2026, time.January, 1), now, false)
	if err == nil {
		t.Fatalf("want an error when one window fails, got %d accounts", len(accounts))
	}
	if !strings.Contains(err.Error(), "500") || !strings.Contains(err.Error(), "2026-03-27") {
		t.Errorf("error = %q, want the status and the failed window's start", err)
	}
	if accounts != nil {
		t.Errorf("accounts = %v, want none: an import must not run on part of the range", accounts)
	}
}

func TestSFResponseMessages(t *testing.T) {
	var r sfResponse
	if err := json.Unmarshal([]byte(`{
		"errors": ["v1 message"],
		"errlist": [{"code": "act.closed", "msg": "Account closed", "account_id": "a1"}, {"code": "gen.unknown"}],
		"accounts": []
	}`), &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := fmt.Sprint(r.messages()); got != "[v1 message Account closed gen.unknown]" {
		t.Errorf("messages = %s", got)
	}
}

func TestSFTransactionDate(t *testing.T) {
	now := time.Date(2026, time.October, 12, 15, 0, 0, 0, time.UTC)
	posted, happened := day(2026, time.October, 9), day(2026, time.October, 8)
	tests := []struct {
		name string
		txn  sfTransaction
		want time.Time
	}{
		{"posted", sfTransaction{Posted: posted.Unix(), TransactedAt: happened.Unix()}, posted},
		// Pending transactions may come with posted 0.
		{"pending with transacted_at", sfTransaction{Pending: true, TransactedAt: happened.Unix()}, happened},
		{"pending with neither", sfTransaction{Pending: true}, now},
	}
	for _, tt := range tests {
		if got := tt.txn.date(now); !got.Equal(tt.want) {
			t.Errorf("%s: date = %s, want %s", tt.name, got, tt.want)
		}
	}
}

func TestFetchSimplefinAccountsPendingParam(t *testing.T) {
	now := time.Date(2026, time.October, 12, 15, 0, 0, 0, time.UTC)
	for _, include := range []bool{true, false} {
		bridge := &fakeBridge{now: now, txns: map[string][]sfTransaction{"checking": {
			posted("cleared", day(2026, time.October, 1)),
			{ID: "authorized", Pending: true, TransactedAt: day(2026, time.October, 11).Unix(), Amount: "-5.00"},
		}}}
		accounts, _, err := fetchSimplefinAccounts(context.Background(), http.DefaultClient, newFakeBridge(t, bridge), day(2026, time.January, 1), now, include)
		if err != nil {
			t.Fatalf("include=%v: %v", include, err)
		}
		for i, asked := range bridge.pendingAsked {
			if asked != include {
				t.Errorf("include=%v: request %d sent pending=1: %v", include, i+1, asked)
			}
		}
		if got, want := len(accounts[0].Transactions), map[bool]int{true: 2, false: 1}[include]; got != want {
			t.Errorf("include=%v: got %d transactions, want %d", include, got, want)
		}
	}
}
