package storage

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

// CONSTRAINT: whatsmeow refuses to upgrade its schema unless foreign keys are on for the connection.
func DSN(path string) string {
	return "file:" + path +
		"?_pragma=foreign_keys(1)" +
		"&_pragma=journal_mode(WAL)" +
		"&_pragma=busy_timeout(5000)" +
		"&_pragma=synchronous(NORMAL)"
}

func Open(ctx context.Context, path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, fmt.Errorf("criar pasta do banco: %w", err)
	}
	db, err := sql.Open("sqlite", DSN(path))
	if err != nil {
		return nil, fmt.Errorf("abrir banco %s: %w", path, err)
	}
	// TRADEOFF: one connection serialises access; plenty for one church and it rules out SQLITE_BUSY between writers.
	db.SetMaxOpenConns(1)
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("conectar ao banco %s: %w", path, err)
	}
	return db, nil
}

func Migrate(ctx context.Context, db *sql.DB) error {
	files, err := fs.Sub(migrations, "migrations")
	if err != nil {
		return err
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, db, files)
	if err != nil {
		return fmt.Errorf("preparar migrações: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("aplicar migrações: %w", err)
	}
	return nil
}
