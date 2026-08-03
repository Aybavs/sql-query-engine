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

func testCatalog() *catalog.Catalog {
	cat := catalog.New()
	cat.Add(&catalog.Table{
		Name: "users", File: "users.csv",
		Columns: []catalog.Column{
			{Name: "id", Type: value.TInt},
			{Name: "name", Type: value.TText},
			{Name: "age", Type: value.TInt},
		},
	})
	return cat
}

func buildAndRun(t *testing.T, sql, dir string, cat *catalog.Catalog) [][]string {
	t.Helper()
	toks, err := lexer.Lex(sql)
	if err != nil {
		t.Fatal(err)
	}
	st, err := parser.New(toks).ParseSelect()
	if err != nil {
		t.Fatal(err)
	}
	op, _, err := Build(st, cat, dir)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	var out [][]string
	for {
		r, ok := op.Next()
		if !ok {
			break
		}
		var cells []string
		for _, v := range r {
			cells = append(cells, v.String())
		}
		out = append(out, cells)
	}
	return out
}

func TestBuildAndExecute(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "users.csv"), []byte("1,alice,30\n2,bob,15\n3,carol,40\n"), 0o644)

	got := buildAndRun(t, "SELECT name FROM users WHERE age >= 18 ORDER BY age DESC", dir, testCatalog())
	if len(got) != 2 || got[0][0] != "carol" || got[1][0] != "alice" {
		t.Fatalf("result = %v, want [[carol] [alice]]", got)
	}
}

func TestBuildStarAndLimit(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "users.csv"), []byte("1,alice,30\n2,bob,15\n"), 0o644)

	got := buildAndRun(t, "SELECT * FROM users LIMIT 1", dir, testCatalog())
	if len(got) != 1 || len(got[0]) != 3 || got[0][1] != "alice" {
		t.Fatalf("result = %v, want one full row for alice", got)
	}
}

func TestBuildUnknownColumn(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "users.csv"), []byte("1,alice,30\n"), 0o644)
	toks, _ := lexer.Lex("SELECT nope FROM users")
	st, _ := parser.New(toks).ParseSelect()
	if _, _, err := Build(st, testCatalog(), dir); err == nil {
		t.Fatal("expected unknown-column error")
	}
}

func TestBuildUnknownTable(t *testing.T) {
	toks, _ := lexer.Lex("SELECT * FROM missing")
	st, _ := parser.New(toks).ParseSelect()
	if _, _, err := Build(st, testCatalog(), t.TempDir()); err == nil {
		t.Fatal("expected unknown-table error")
	}
}
