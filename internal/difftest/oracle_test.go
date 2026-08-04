package difftest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aybavs/sql-query-engine/internal/catalog"
	"github.com/aybavs/sql-query-engine/internal/value"
)

func fixture(t *testing.T) (*catalog.Catalog, string) {
	t.Helper()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "users.csv"), []byte("1,alice,30\n2,bob,\n"), 0o644)
	cat := catalog.New()
	cat.Add(&catalog.Table{Name: "users", File: "users.csv", Columns: []catalog.Column{
		{Name: "id", Type: value.TInt},
		{Name: "name", Type: value.TText},
		{Name: "age", Type: value.TInt},
	}})
	return cat, dir
}

func TestOracleLoadsAndQueries(t *testing.T) {
	cat, dir := fixture(t)
	o, err := NewOracle(cat, dir)
	if err != nil {
		t.Fatalf("NewOracle: %v", err)
	}
	defer o.Close()

	rows, err := o.Query("SELECT name FROM users ORDER BY name")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %v, want 2", rows)
	}
	if s, ok := rows[0][0].(string); !ok || s != "alice" {
		t.Fatalf("first row = %v, want alice", rows[0])
	}
}

func TestOracleKeepsEmptyFieldsNull(t *testing.T) {
	cat, dir := fixture(t)
	o, err := NewOracle(cat, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()

	rows, err := o.Query("SELECT age FROM users WHERE name = 'bob'")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0][0] != nil {
		t.Fatalf("bob's age = %v, want NULL", rows)
	}
}

func TestOracleReportsInvalidQuery(t *testing.T) {
	cat, dir := fixture(t)
	o, err := NewOracle(cat, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()

	if _, err := o.Query("SELECT nope FROM users"); err == nil {
		t.Fatal("expected an error for an unknown column")
	}
}
