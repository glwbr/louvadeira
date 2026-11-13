package storage_test

import (
	"context"
	"path/filepath"
	"testing"

	"louvadeira/internal/storage"
)

func TestOpenAndMigrateCreatesTheSchema(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(ctx, filepath.Join(t.TempDir(), "nested", "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	for range 2 {
		if err := storage.Migrate(ctx, db); err != nil {
			t.Fatalf("Migrate: %v", err)
		}
	}
	var foreignKeys int
	if err := db.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil || foreignKeys != 1 {
		t.Fatalf("foreign_keys = %d, err = %v", foreignKeys, err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO app_settings (key, value) VALUES ('k', 'v')"); err != nil {
		t.Fatalf("app_settings missing: %v", err)
	}
}
