package db

// Gated on TEST_DATABASE_URL, whose schema these tests never touch: the
// connection is used only to create a scratch database, which is migrated
// from empty and dropped afterwards. Migrating the shared database would race
// every other integration test using it.

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
)

// scratchDB creates an empty database beside the one TEST_DATABASE_URL names
// and returns its URL; it is dropped when the test ends.
func scratchDB(t *testing.T) string {
	t.Helper()
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration test")
	}
	u, err := url.Parse(base)
	if err != nil || u.Scheme == "" {
		t.Fatalf("TEST_DATABASE_URL must be a URL: %v", err)
	}

	suffix := make([]byte, 6)
	if _, err := rand.Read(suffix); err != nil {
		t.Fatal(err)
	}
	name := "ppbudget_migrate_test_" + hex.EncodeToString(suffix)

	ctx := context.Background()
	admin, err := pgx.Connect(ctx, base)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { admin.Close(context.Background()) })
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatalf("create scratch database: %v", err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)"); err != nil {
			t.Logf("cleanup: failed to drop %s: %v", name, err)
		}
	})

	u.Path = "/" + name
	return u.String()
}

// newestOnDisk reads the highest migration number from the files themselves,
// independently of Migrate's own listing.
func newestOnDisk(t *testing.T) uint {
	t.Helper()
	entries, err := os.ReadDir("migrations")
	if err != nil {
		t.Fatal(err)
	}
	var newest uint64
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".up.sql") {
			continue
		}
		n, err := strconv.ParseUint(strings.SplitN(e.Name(), "_", 2)[0], 10, 64)
		if err != nil {
			t.Fatalf("migration %s has no numeric prefix", e.Name())
		}
		newest = max(newest, n)
	}
	return uint(newest)
}

func setVersion(t *testing.T, dbURL string, version uint, dirty bool) {
	t.Helper()
	conn, err := pgx.Connect(context.Background(), dbURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer conn.Close(context.Background())
	if _, err := conn.Exec(context.Background(),
		"UPDATE schema_migrations SET version = $1, dirty = $2", int64(version), dirty); err != nil {
		t.Fatalf("set version: %v", err)
	}
}

// Every migration applies cleanly from an empty database, and a second start
// finds nothing to do.
func TestMigrateFromEmptyIntegration(t *testing.T) {
	dbURL := scratchDB(t)
	newest := newestOnDisk(t)

	res, err := Migrate(dbURL)
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if res.From != 0 || res.To != newest || res.Latest != newest {
		t.Errorf("first run: got %+v, want from 0 to %d", res, newest)
	}

	res, err = Migrate(dbURL)
	if err != nil {
		t.Fatalf("second Migrate: %v", err)
	}
	if res.From != newest || res.To != newest {
		t.Errorf("second run should change nothing; got %+v", res)
	}
}

// Instances starting together queue on the advisory lock instead of both
// running the same migrations, which would fail on the second CREATE TABLE.
func TestMigrateConcurrentStartsIntegration(t *testing.T) {
	dbURL := scratchDB(t)
	newest := newestOnDisk(t)

	var wg sync.WaitGroup
	errs := make([]error, 3)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = Migrate(dbURL)
		}()
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Errorf("instance %d: %v", i, err)
		}
	}
	if res, err := Migrate(dbURL); err != nil || res.To != newest {
		t.Errorf("after concurrent starts: %+v, %v; want version %d", res, err, newest)
	}
}

// A migration that failed part way is refused, not retried over.
func TestMigrateRefusesADirtySchemaIntegration(t *testing.T) {
	dbURL := scratchDB(t)
	if _, err := Migrate(dbURL); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	newest := newestOnDisk(t)
	setVersion(t, dbURL, newest, true)

	res, err := Migrate(dbURL)
	if err == nil || !strings.Contains(err.Error(), "dirty") {
		t.Fatalf("expected a dirty-schema error, got %v", err)
	}
	if res.From != newest {
		t.Errorf("the error should carry the dirty version; got %+v", res)
	}
}

// A database migrated by a newer build is left alone, so rolling back a
// deploy does not fail to start or try to undo the schema.
func TestMigrateLeavesANewerSchemaAloneIntegration(t *testing.T) {
	dbURL := scratchDB(t)
	if _, err := Migrate(dbURL); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	newest := newestOnDisk(t)
	setVersion(t, dbURL, newest+1, false)

	res, err := Migrate(dbURL)
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if !res.Ahead() || res.From != newest+1 || res.To != newest+1 {
		t.Errorf("got %+v, want the newer version untouched", res)
	}
}

func TestMigrateOnStart(t *testing.T) {
	cases := []struct {
		value string
		set   bool
		want  bool
		err   bool
	}{
		{set: false, want: true},
		{value: "", set: true, want: true},
		{value: "false", set: true, want: false},
		{value: "0", set: true, want: false},
		{value: "true", set: true, want: true},
		{value: "sometimes", set: true, err: true},
	}
	for _, c := range cases {
		if c.set {
			t.Setenv("MIGRATE_ON_START", c.value)
		} else {
			t.Setenv("MIGRATE_ON_START", "")
			os.Unsetenv("MIGRATE_ON_START")
		}
		got, err := MigrateOnStart()
		if (err != nil) != c.err || (!c.err && got != c.want) {
			t.Errorf("MIGRATE_ON_START=%q (set %v): got %v, %v", c.value, c.set, got, err)
		}
	}
}
