package plan

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aybavs/sql-query-engine/internal/catalog"
	"github.com/aybavs/sql-query-engine/internal/lexer"
	"github.com/aybavs/sql-query-engine/internal/parser"
	"github.com/aybavs/sql-query-engine/internal/value"
)

// labelCatalog has users(id, name, age, city) and orders(id, user_id, total).
func labelCatalog() *catalog.Catalog {
	cat := catalog.New()
	cat.Add(&catalog.Table{
		Name: "users", File: "users.csv",
		Columns: []catalog.Column{
			{Name: "id", Type: value.TInt},
			{Name: "name", Type: value.TText},
			{Name: "age", Type: value.TInt},
			{Name: "city", Type: value.TText},
		},
	})
	cat.Add(&catalog.Table{
		Name: "orders", File: "orders.csv",
		Columns: []catalog.Column{
			{Name: "id", Type: value.TInt},
			{Name: "user_id", Type: value.TInt},
			{Name: "total", Type: value.TInt},
		},
	})
	return cat
}

// schemaNames plans a query and returns its result-column labels.
func schemaNames(t *testing.T, sql, dir string) []string {
	t.Helper()
	toks, err := lexer.Lex(sql)
	if err != nil {
		t.Fatalf("lex: %v", err)
	}
	st, err := parser.New(toks).ParseSelect()
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	_, schema, err := Build(st, labelCatalog(), dir)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	names := make([]string, len(schema))
	for i, c := range schema {
		names[i] = c.Name
	}
	return names
}

func labelDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "users.csv"), []byte("1,alice,30,berlin\n2,bob,15,paris\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "orders.csv"), []byte("10,1,100\n"), 0o644)
	return dir
}

func TestAggregateProjectionsAreLabelled(t *testing.T) {
	dir := labelDir(t)
	got := schemaNames(t, "SELECT city, COUNT(id), AVG(age) FROM users GROUP BY city", dir)
	want := []string{"city", "COUNT(id)", "AVG(age)"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("labels = %v, want %v", got, want)
		}
	}
}

func TestComputedProjectionIsLabelled(t *testing.T) {
	dir := labelDir(t)
	got := schemaNames(t, "SELECT age + 1 FROM users", dir)
	if got[0] != "age + 1" {
		t.Fatalf("label = %q, want %q", got[0], "age + 1")
	}
}

func TestCountStarIsLabelled(t *testing.T) {
	dir := labelDir(t)
	got := schemaNames(t, "SELECT COUNT(*) FROM users", dir)
	if got[0] != "COUNT(*)" {
		t.Fatalf("label = %q, want %q", got[0], "COUNT(*)")
	}
}

// A bare column keeps its plain name even when qualified, matching SQLite.
func TestBareColumnKeepsPlainName(t *testing.T) {
	dir := labelDir(t)
	got := schemaNames(t, "SELECT users.name FROM users JOIN orders ON users.id = orders.user_id", dir)
	if got[0] != "name" {
		t.Fatalf("label = %q, want %q", got[0], "name")
	}
}
