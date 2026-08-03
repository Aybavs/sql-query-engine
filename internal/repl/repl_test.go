package repl

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aybavs/sql-query-engine/internal/catalog"
	"github.com/aybavs/sql-query-engine/internal/value"
)

func TestReplRunsQuery(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "users.csv"), []byte("1,alice,30\n2,bob,15\n"), 0o644)
	cat := catalog.New()
	cat.Add(&catalog.Table{Name: "users", File: "users.csv", Columns: []catalog.Column{
		{Name: "id", Type: value.TInt},
		{Name: "name", Type: value.TText},
		{Name: "age", Type: value.TInt},
	}})

	in := strings.NewReader("SELECT name FROM users WHERE age >= 18\n")
	var out bytes.Buffer
	Run(cat, dir, in, &out)

	if !strings.Contains(out.String(), "alice") || strings.Contains(out.String(), "bob") {
		t.Fatalf("output = %q, want alice and not bob", out.String())
	}
}

func TestReplReportsError(t *testing.T) {
	dir := t.TempDir()
	cat := catalog.New()
	in := strings.NewReader("SELECT * FROM missing\n")
	var out bytes.Buffer
	Run(cat, dir, in, &out)
	if !strings.Contains(strings.ToLower(out.String()), "error") {
		t.Fatalf("expected an error line, got %q", out.String())
	}
}

func TestLoadSchema(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "schema.txt")
	os.WriteFile(path, []byte("users(id INT, name TEXT, age INT)\n"), 0o644)
	cat, err := LoadSchema(path)
	if err != nil {
		t.Fatal(err)
	}
	tbl, ok := cat.Table("users")
	if !ok || len(tbl.Columns) != 3 || tbl.Columns[2].Type != value.TInt {
		t.Fatalf("schema parse wrong: %#v", tbl)
	}
	if tbl.File != "users.csv" {
		t.Fatalf("file = %q, want users.csv", tbl.File)
	}
}

func TestLoadSchemaRejectsBadLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "schema.txt")
	os.WriteFile(path, []byte("garbage\n"), 0o644)
	if _, err := LoadSchema(path); err == nil {
		t.Fatal("expected error for malformed schema line")
	}
}

func TestReplRunsAggregateQuery(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "users.csv"), []byte("1,alice,30\n2,bob,15\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cat := catalog.New()
	cat.Add(&catalog.Table{Name: "users", File: "users.csv", Columns: []catalog.Column{{Name: "id", Type: value.TInt}, {Name: "name", Type: value.TText}, {Name: "age", Type: value.TInt}}})
	in := strings.NewReader("SELECT COUNT(*), AVG(age) FROM users\n")
	var out bytes.Buffer
	Run(cat, dir, in, &out)
	if !strings.Contains(out.String(), "2") || !strings.Contains(out.String(), "22.5") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestReplReportsAggregatePlannerError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "users.csv"), []byte("1,alice,30\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cat := catalog.New()
	cat.Add(&catalog.Table{Name: "users", File: "users.csv", Columns: []catalog.Column{{Name: "id", Type: value.TInt}, {Name: "name", Type: value.TText}, {Name: "age", Type: value.TInt}}})
	var out bytes.Buffer
	Run(cat, dir, strings.NewReader("SELECT name, COUNT(*) FROM users\n"), &out)
	if !strings.Contains(out.String(), "GROUP BY") {
		t.Fatalf("output = %q", out.String())
	}
}
