package migration_test

import (
	"path/filepath"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// TestMigrationFilesIntegrity ensures no duplicate sequence numbers exist in migration directory
func TestMigrationFilesIntegrity(t *testing.T) {
	absPath, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("failed to resolve migration path: %v", err)
	}

	sourceURL := "file://" + filepath.ToSlash(absPath)
	// We instantiate the file driver using a dummy postgres URL format or just check source driver
	_, err = migrate.New(sourceURL, "postgres://user:pass@localhost:5432/dummy?sslmode=disable")
	// If there are duplicate migration files, New() returns error immediately before connecting to DB!
	if err != nil && err.Error() != "failed to connect to `host=localhost user=user database=dummy`: dial error (dial tcp [::1]:5432: connect: connection refused)" {
		// Verify error is not a duplicate file error
		if containsDuplicateError(err.Error()) {
			t.Fatalf("Duplicate migration file detected: %v", err)
		}
	}
}

func containsDuplicateError(msg string) bool {
	return len(msg) > 0 && (contains(msg, "duplicate migration") || contains(msg, "duplicate file"))
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || contains(s[1:], substr)))
}
