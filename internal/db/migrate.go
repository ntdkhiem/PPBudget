package db

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"time"

	"github.com/golang-migrate/migrate/v4"
	pgxmigrate "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
)

// The migrations compiled into the binary, so the schema a build expects
// ships with it rather than being a separate step someone has to remember.
//
//go:embed migrations/*.sql
var migrationFiles embed.FS

// migrateLockTimeout is how long an instance waits for another one to finish
// migrating before giving up. The driver's default is fifteen seconds, which
// a slow migration on a large table outlasts -- and the instance that gave up
// would then refuse to start for no fault of its own.
const migrateLockTimeout = 5 * time.Minute

// MigrationResult is the schema version Migrate found and left.
type MigrationResult struct {
	// From and To are the versions before and after; 0 is an empty database.
	From, To uint
	// Latest is the newest migration compiled into this binary. From above it
	// means the database was migrated by a newer build.
	Latest uint
}

// Ahead reports whether the database was migrated by a newer build than this
// one, in which case Migrate left it alone.
func (r MigrationResult) Ahead() bool { return r.From > r.Latest }

// MigrateOnStart reports whether the server should apply migrations as it
// starts: yes unless MIGRATE_ON_START says otherwise, for deployments that
// prefer to run `make migrate-up` themselves.
func MigrateOnStart() (bool, error) {
	v, ok := os.LookupEnv("MIGRATE_ON_START")
	if !ok || v == "" {
		return true, nil
	}
	on, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("MIGRATE_ON_START must be true or false, got %q", v)
	}
	return on, nil
}

// Migrate brings the schema up to date with the migrations compiled into the
// binary.
//
// Applied at startup so a build never runs against the schema it replaced.
// Done by hand, the migration and the deploy were two steps, and whichever
// came first broke the other: 0031 drops the table the previous server reads.
//
// The bookkeeping is the migrate CLI's own schema_migrations table, so a
// database migrated with `make migrate-up` carries on from where it is, and
// instances starting together queue on the driver's advisory lock instead of
// racing each other through the same migrations.
//
// A dirty version -- a migration that failed part way -- is refused rather
// than retried: what it left behind needs a person to look at, and applying
// newer migrations on top would bury it. A database already past this build's
// newest migration is left alone, so rolling back a deploy does not also try
// to roll back the schema.
func Migrate(dbURL string) (MigrationResult, error) {
	var res MigrationResult

	src, err := iofs.New(migrationFiles, "migrations")
	if err != nil {
		return res, fmt.Errorf("unable to read embedded migrations: %w", err)
	}
	if res.Latest, err = latestVersion(src); err != nil {
		src.Close()
		return res, err
	}

	// A connection of its own: the driver holds its advisory lock on one
	// session for the whole run, which a pooled connection cannot promise.
	conn, err := sql.Open("pgx", dbURL)
	if err != nil {
		src.Close()
		return res, fmt.Errorf("unable to open database for migration: %w", err)
	}
	driver, err := pgxmigrate.WithInstance(conn, &pgxmigrate.Config{})
	if err != nil {
		src.Close()
		conn.Close()
		return res, fmt.Errorf("unable to prepare database for migration: %w", err)
	}
	m, err := migrate.NewWithInstance("iofs", src, "pgx5", driver)
	if err != nil {
		src.Close()
		driver.Close()
		return res, fmt.Errorf("unable to set up migrations: %w", err)
	}
	defer m.Close()
	m.LockTimeout = migrateLockTimeout

	from, dirty, err := m.Version()
	switch {
	case errors.Is(err, migrate.ErrNilVersion):
		from = 0 // nothing applied yet
	case err != nil:
		return res, fmt.Errorf("unable to read schema version: %w", err)
	case dirty:
		res.From, res.To = from, from
		return res, fmt.Errorf("schema is dirty at version %d: that migration failed part way. "+
			"Repair the schema by hand, then record the version it is really at with "+
			"`migrate force <version>` before starting again", from)
	}
	res.From, res.To = from, from

	if res.Ahead() {
		return res, nil
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		// Read back where it stopped, so the error names the version to repair.
		if v, _, verr := m.Version(); verr == nil {
			res.To = v
		}
		return res, fmt.Errorf("unable to apply migrations: %w", err)
	}

	to, _, err := m.Version()
	if err != nil {
		return res, fmt.Errorf("unable to read schema version after migrating: %w", err)
	}
	res.To = to
	return res, nil
}

// latestVersion is the newest migration in src.
func latestVersion(src source.Driver) (uint, error) {
	v, err := src.First()
	if err != nil {
		return 0, fmt.Errorf("no embedded migrations found: %w", err)
	}
	for {
		next, err := src.Next(v)
		if errors.Is(err, fs.ErrNotExist) {
			return v, nil
		}
		if err != nil {
			return 0, fmt.Errorf("unable to list embedded migrations: %w", err)
		}
		v = next
	}
}
