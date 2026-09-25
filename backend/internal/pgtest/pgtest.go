// Package pgtest da a los tests de integración una base de datos PostgreSQL aislada y desechable.
//
// Se activa con TEST_DATABASE_URL (una URL con permisos para crear bases, p. ej.
// postgres://postgres:test@127.0.0.1:55433/postgres?sslmode=disable). Si no está definida, los tests
// se saltan: `go test ./...` sigue funcionando sin Postgres.
package pgtest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/soypiipe/energy-ai/backend/internal/db"
)

// New crea una base nueva con las migraciones aplicadas y devuelve su pool.
// La base se elimina al terminar el test.
func New(t *testing.T) *pgxpool.Pool {
	t.Helper()
	adminURL := os.Getenv("TEST_DATABASE_URL")
	if adminURL == "" {
		t.Skip("TEST_DATABASE_URL no definida: se salta el test de integración")
	}
	ctx := context.Background()

	admin, err := pgxpool.New(ctx, adminURL)
	if err != nil {
		t.Fatalf("conectar a %s: %v", "TEST_DATABASE_URL", err)
	}
	suffix := make([]byte, 6)
	if _, err := rand.Read(suffix); err != nil {
		t.Fatal(err)
	}
	name := "test_" + hex.EncodeToString(suffix)
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		admin.Close()
		t.Fatalf("crear base de pruebas: %v", err)
	}

	u, err := url.Parse(adminURL)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	pool, err := db.Connect(ctx, u.String())
	if err != nil {
		t.Fatalf("conectar a la base de pruebas: %v", err)
	}
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrar: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
		_, _ = admin.Exec(ctx, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
		admin.Close()
	})
	return pool
}
