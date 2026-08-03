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

func joinCatalog() *catalog.Catalog {
	cat := testCatalog() // users(id, name, age)
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

func joinDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "users.csv"), []byte("1,alice,30\n2,bob,25\n3,carol,40\n"), 0o644)
	// user 1 has two orders; user 3 has none; one order has a NULL user_id
	os.WriteFile(filepath.Join(dir, "orders.csv"), []byte("10,1,100\n11,1,300\n12,2,200\n13,,50\n"), 0o644)
	return dir
}

func TestJoinEndToEnd(t *testing.T) {
	dir := joinDir(t)
	got := buildAndRun(t,
		"SELECT users.name, orders.total FROM users JOIN orders ON users.id = orders.user_id ORDER BY orders.total",
		dir, joinCatalog())
	want := [][]string{{"alice", "100"}, {"bob", "200"}, {"alice", "300"}}
	if len(got) != len(want) {
		t.Fatalf("rows = %v, want %v", got, want)
	}
	for i := range want {
		if got[i][0] != want[i][0] || got[i][1] != want[i][1] {
			t.Fatalf("row %d = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestJoinReversedOnOrder(t *testing.T) {
	dir := joinDir(t)
	got := buildAndRun(t,
		"SELECT users.name FROM users JOIN orders ON orders.user_id = users.id",
		dir, joinCatalog())
	if len(got) != 3 {
		t.Fatalf("rows = %v, want 3 (ON order must not matter)", got)
	}
}

func TestJoinWithWhere(t *testing.T) {
	dir := joinDir(t)
	got := buildAndRun(t,
		"SELECT users.name FROM users JOIN orders ON users.id = orders.user_id WHERE orders.total > 150",
		dir, joinCatalog())
	if len(got) != 2 {
		t.Fatalf("rows = %v, want 2 (totals 200 and 300)", got)
	}
}

func TestJoinAmbiguousColumnRejected(t *testing.T) {
	dir := joinDir(t)
	toks, _ := lexer.Lex("SELECT id FROM users JOIN orders ON users.id = orders.user_id")
	st, _ := parser.New(toks).ParseSelect()
	if _, _, err := Build(st, joinCatalog(), dir); err == nil {
		t.Fatal("bare `id` exists in both tables and must be rejected as ambiguous")
	}
}

func TestJoinRejectsNonEqualityOn(t *testing.T) {
	dir := joinDir(t)
	toks, _ := lexer.Lex("SELECT users.name FROM users JOIN orders ON users.id > orders.user_id")
	st, _ := parser.New(toks).ParseSelect()
	if _, _, err := Build(st, joinCatalog(), dir); err == nil {
		t.Fatal("only equality joins are supported; expected an error")
	}
}

func TestJoinRejectsColumnToLiteralOn(t *testing.T) {
	dir := joinDir(t)
	toks, _ := lexer.Lex("SELECT users.name FROM users JOIN orders ON users.id = 1")
	st, _ := parser.New(toks).ParseSelect()
	if _, _, err := Build(st, joinCatalog(), dir); err == nil {
		t.Fatal("join key operands must contain columns from opposite inputs")
	}
}

func TestJoinRejectsSameInputOn(t *testing.T) {
	dir := joinDir(t)
	toks, _ := lexer.Lex("SELECT users.name FROM users JOIN orders ON users.id = users.age")
	st, _ := parser.New(toks).ParseSelect()
	if _, _, err := Build(st, joinCatalog(), dir); err == nil {
		t.Fatal("join key operands must belong to opposite inputs")
	}
}

func TestJoinRejectsAmbiguousOnOwnership(t *testing.T) {
	dir := joinDir(t)
	toks, _ := lexer.Lex("SELECT users.name FROM users JOIN orders ON id = orders.user_id")
	st, _ := parser.New(toks).ParseSelect()
	if _, _, err := Build(st, joinCatalog(), dir); err == nil {
		t.Fatal("an unqualified join key that resolves in both inputs must be rejected")
	}
}

func TestJoinRejectsLiteralOnlyOn(t *testing.T) {
	dir := joinDir(t)
	toks, _ := lexer.Lex("SELECT users.name FROM users JOIN orders ON 1 = 2")
	st, _ := parser.New(toks).ParseSelect()
	if _, _, err := Build(st, joinCatalog(), dir); err == nil {
		t.Fatal("literal-only join keys must be rejected")
	}
}

func TestJoinRejectsMixedInputOperandOn(t *testing.T) {
	dir := joinDir(t)
	toks, _ := lexer.Lex("SELECT users.name FROM users JOIN orders ON (users.id + orders.id) = orders.total")
	st, _ := parser.New(toks).ParseSelect()
	if _, _, err := Build(st, joinCatalog(), dir); err == nil {
		t.Fatal("a join key operand spanning both inputs must be rejected")
	}
}

func TestJoinRejectsCompoundAmbiguousOnOwnership(t *testing.T) {
	dir := joinDir(t)
	toks, _ := lexer.Lex("SELECT users.name FROM users JOIN orders ON (id + users.age) = orders.total")
	st, _ := parser.New(toks).ParseSelect()
	if _, _, err := Build(st, joinCatalog(), dir); err == nil {
		t.Fatal("an ambiguous column hidden inside a compound join key must be rejected")
	}
}

func TestJoinRejectsNestedEqualityOn(t *testing.T) {
	dir := joinDir(t)
	toks, _ := lexer.Lex("SELECT users.name FROM users JOIN orders ON (users.id = users.age) = orders.total")
	st, _ := parser.New(toks).ParseSelect()
	if _, _, err := Build(st, joinCatalog(), dir); err == nil {
		t.Fatal("ON must contain a single equality")
	}
}
