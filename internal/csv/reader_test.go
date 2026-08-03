package csv

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aybavs/sql-query-engine/internal/catalog"
	"github.com/aybavs/sql-query-engine/internal/value"
)

func TestReadTypedRows(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "users.csv")
	os.WriteFile(path, []byte("1,alice,30\n2,bob,\n"), 0o644)

	cols := []catalog.Column{
		{Name: "id", Type: value.TInt},
		{Name: "name", Type: value.TText},
		{Name: "age", Type: value.TInt},
	}
	rows, err := Read(path, cols)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	if rows[0][0].I != 1 || rows[0][1].S != "alice" || rows[0][2].I != 30 {
		t.Fatalf("row0 = %v", rows[0])
	}
	if !rows[1][2].IsNull() {
		t.Fatal("empty age field must be NULL")
	}
}

func TestReadRejectsBadInt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "t.csv")
	os.WriteFile(path, []byte("notanint\n"), 0o644)
	if _, err := Read(path, []catalog.Column{{Name: "id", Type: value.TInt}}); err == nil {
		t.Fatal("expected parse error for bad int")
	}
}
