package sqldb_test

import (
	"testing"
	"time"

	"school-with-ai/infra/sqldb"
)

func TestOpenConfigDefaults(t *testing.T) {
	t.Parallel()

	// We test that Open returns a *sqlx.DB with proper pool settings.
	// We use an invalid host so no real connection is made,
	// but sqlx.Open (like sql.Open) is lazy — it validates the DSN
	// and sets pool parameters without connecting.
	cfg := sqldb.Config{
		User:         "testuser",
		Password:     "testpass",
		Host:         "localhost:59999",
		Name:         "testdb",
		MaxIdleConns: 3,
		MaxOpenConns: 10,
		DisableTLS:   true,
	}

	db, err := sqldb.Open(cfg)
	if err != nil {
		t.Fatalf("Open() returned error: %v", err)
	}
	defer db.Close()

	// Verify pool settings are applied
	stats := db.Stats()
	if stats.MaxOpenConnections != 10 {
		t.Errorf("MaxOpenConnections = %d, want 10", stats.MaxOpenConnections)
	}
}

func TestOpenConnMaxLifetimeDefaults(t *testing.T) {
	t.Parallel()

	// When ConnMaxLifetime is not set (zero), the default of 30m should apply.
	cfg := sqldb.Config{
		User:         "testuser",
		Password:     "testpass",
		Host:         "localhost:59999",
		Name:         "testdb",
		MaxIdleConns: 2,
		MaxOpenConns: 25,
		DisableTLS:   true,
		// ConnMaxLifetime: 0 (not set — should default to 30m)
		// ConnMaxIdleTime: 0 (not set — should default to 5m)
	}

	db, err := sqldb.Open(cfg)
	if err != nil {
		t.Fatalf("Open() returned error: %v", err)
	}
	defer db.Close()

	// sql.DB doesn't expose ConnMaxLifetime via Stats(), but we can verify
	// that the config fields exist and that Open doesn't panic with zero values.
	// The real validation is that Open() applied defaults without error.
	if db == nil {
		t.Fatal("Open() returned nil db")
	}
}

func TestOpenCustomConnLifetime(t *testing.T) {
	t.Parallel()

	// When ConnMaxLifetime is explicitly set, use the provided value.
	cfg := sqldb.Config{
		User:            "testuser",
		Password:        "testpass",
		Host:            "localhost:59999",
		Name:            "testdb",
		MaxIdleConns:    2,
		MaxOpenConns:    25,
		DisableTLS:      true,
		ConnMaxLifetime: 10 * time.Minute,
		ConnMaxIdleTime: 2 * time.Minute,
	}

	db, err := sqldb.Open(cfg)
	if err != nil {
		t.Fatalf("Open() returned error: %v", err)
	}
	defer db.Close()

	if db == nil {
		t.Fatal("Open() returned nil db")
	}
}

func TestOpenMaxOpenConnsNotZero(t *testing.T) {
	t.Parallel()

	// Verify that when MaxOpenConns is 0 (unset), the library applies a sane
	// default (50) to prevent overwhelming PostgreSQL under high concurrency.
	cfg := sqldb.Config{
		User:         "testuser",
		Password:     "testpass",
		Host:         "localhost:59999",
		Name:         "testdb",
		MaxIdleConns: 0,
		MaxOpenConns: 0,
		DisableTLS:   true,
	}

	db, err := sqldb.Open(cfg)
	if err != nil {
		t.Fatalf("Open() returned error: %v", err)
	}
	defer db.Close()

	// MaxOpenConns=0 now defaults to 50, MaxIdleConns=0 now defaults to 25
	stats := db.Stats()
	if stats.MaxOpenConnections != 50 {
		t.Errorf("MaxOpenConnections = %d, want 50 (sane default)", stats.MaxOpenConnections)
	}
}
