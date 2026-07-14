// Package demoinfra boots the app's entire infrastructure in-process for
// demo deployments: a real embedded Postgres (fergusstrange/embedded-postgres),
// an in-memory Redis (miniredis), and an in-memory S3 server (gofakes3).
//
// Everything is ephemeral — all data is lost when the process exits. This is
// intentional: demo mode targets zero-dependency showcase deployments where
// no real users or durable data are expected.
package demoinfra

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"aicompass/foundation/logger"

	"github.com/alicebob/miniredis/v2"
	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
)

// Credentials used by the embedded Postgres. They only ever accept loopback
// connections from this process, so well-known values are fine.
const (
	dbUser     = "demo"
	dbPassword = "demo"
	dbName     = "projections"
)

// Overrides carries the connection settings the composition root must apply
// to the app config when demo mode is active.
type Overrides struct {
	DBUser     string
	DBPassword string
	DBHost     string // host:port
	DBName     string

	RedisAddr string // host:port, no password

	S3Endpoint string // host:port, plain HTTP, credentials ignored
}

// Infra owns the running embedded services. Stop it on shutdown.
type Infra struct {
	pg     *embeddedpostgres.EmbeddedPostgres
	pgData string
	redis  *miniredis.Miniredis
	s3     *http.Server
}

// Start boots embedded Postgres, miniredis, and gofakes3, creating the given
// S3 buckets up front. On any failure it tears down whatever already started.
func Start(ctx context.Context, log *logger.Logger, buckets []string) (*Infra, Overrides, error) {
	infra := &Infra{}
	ov := Overrides{DBUser: dbUser, DBPassword: dbPassword, DBName: dbName}

	// ── In-memory Redis ──────────────────────────────────────────────────
	redis, err := miniredis.Run()
	if err != nil {
		return nil, Overrides{}, fmt.Errorf("demo redis: %w", err)
	}
	infra.redis = redis
	ov.RedisAddr = redis.Addr()
	log.Info(ctx, "demo-infra", "status", "in-memory redis started", "addr", ov.RedisAddr)

	// ── In-memory S3 ─────────────────────────────────────────────────────
	s3backend := s3mem.New()
	for _, bucket := range buckets {
		if err := s3backend.CreateBucket(bucket); err != nil {
			infra.Stop()
			return nil, Overrides{}, fmt.Errorf("demo s3 bucket %q: %w", bucket, err)
		}
	}
	s3listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		infra.Stop()
		return nil, Overrides{}, fmt.Errorf("demo s3 listener: %w", err)
	}
	infra.s3 = &http.Server{
		Handler:           gofakes3.New(s3backend).Server(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		if serveErr := infra.s3.Serve(s3listener); serveErr != nil && serveErr != http.ErrServerClosed {
			log.Error(ctx, "demo-infra", "status", "s3 server stopped", "error", serveErr)
		}
	}()
	ov.S3Endpoint = s3listener.Addr().String()
	log.Info(ctx, "demo-infra", "status", "in-memory s3 started", "endpoint", ov.S3Endpoint, "buckets", buckets)

	// ── Embedded Postgres ────────────────────────────────────────────────
	// A real Postgres run from this process: binaries are downloaded to a
	// per-user cache on first start (and reused after), the data directory
	// is a fresh temp dir each boot, so the database starts empty and the
	// app's normal migrations recreate the schema.
	pgPort, err := freePort()
	if err != nil {
		infra.Stop()
		return nil, Overrides{}, fmt.Errorf("demo postgres port: %w", err)
	}
	dataDir, err := os.MkdirTemp("", "aicompass-demo-pg-*")
	if err != nil {
		infra.Stop()
		return nil, Overrides{}, fmt.Errorf("demo postgres data dir: %w", err)
	}
	infra.pgData = dataDir

	log.Info(ctx, "demo-infra", "status", "starting embedded postgres (first run downloads binaries, ~30s)", "port", pgPort)
	// Version is pinned explicitly: Dockerfile.demo pre-seeds the binary
	// cache for exactly this version so container boots never download.
	// Keep the two in sync when upgrading.
	pg := embeddedpostgres.NewDatabase(embeddedpostgres.DefaultConfig().
		Version(embeddedpostgres.V18).
		Username(dbUser).
		Password(dbPassword).
		Database(dbName).
		Port(pgPort).
		DataPath(dataDir).
		StartTimeout(120 * time.Second).
		Logger(os.Stdout))
	if err := pg.Start(); err != nil {
		infra.Stop()
		return nil, Overrides{}, fmt.Errorf("demo postgres: %w", err)
	}
	infra.pg = pg
	ov.DBHost = fmt.Sprintf("localhost:%d", pgPort)
	log.Info(ctx, "demo-infra", "status", "embedded postgres started", "host", ov.DBHost)

	return infra, ov, nil
}

// Stop tears down all embedded services. Safe to call on a partially
// started Infra.
func (i *Infra) Stop() {
	if i.pg != nil {
		_ = i.pg.Stop()
	}
	if i.pgData != "" {
		_ = os.RemoveAll(i.pgData)
	}
	if i.s3 != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = i.s3.Shutdown(ctx)
	}
	if i.redis != nil {
		i.redis.Close()
	}
}

// freePort asks the kernel for an unused TCP port.
func freePort() (uint32, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := l.Addr().(*net.TCPAddr).Port
	if err := l.Close(); err != nil {
		return 0, err
	}
	return uint32(port), nil
}
