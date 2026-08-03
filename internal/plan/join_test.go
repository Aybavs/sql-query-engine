package plan

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aybavs/sql-query-engine/internal/ast"
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

func TestJoinCompoundExpressionEndToEnd(t *testing.T) {
	dir := joinDir(t)
	got := buildAndRun(t,
		"SELECT users.name, orders.total FROM users JOIN orders ON users.id + 0 = orders.user_id ORDER BY orders.total",
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

func TestJoinRejectsIncompatibleKeyTypes(t *testing.T) {
	dir := joinDir(t)
	tests := []struct {
		name string
		sql  string
	}{
		{
			name: "TEXT versus INT",
			sql:  "SELECT users.name FROM users JOIN orders ON users.name = orders.user_id",
		},
		{
			name: "INT versus BOOL",
			sql:  "SELECT users.name FROM users JOIN orders ON users.id = (orders.user_id IS NULL)",
		},
		{
			name: "invalid arithmetic operand",
			sql:  "SELECT users.name FROM users JOIN orders ON users.name + 0 = orders.user_id",
		},
		{
			name: "invalid comparison operand",
			sql:  "SELECT users.name FROM users JOIN orders ON (users.name > 0) = (orders.user_id > 0)",
		},
		{
			name: "invalid logical operand",
			sql:  "SELECT users.name FROM users JOIN orders ON (users.id AND TRUE) = (orders.user_id > 0)",
		},
		{
			name: "invalid NOT operand",
			sql:  "SELECT users.name FROM users JOIN orders ON (NOT users.id) = (orders.user_id IS NULL)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			toks, err := lexer.Lex(tt.sql)
			if err != nil {
				t.Fatal(err)
			}
			st, err := parser.New(toks).ParseSelect()
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err := Build(st, joinCatalog(), dir); err == nil {
				t.Fatal("expected incompatible join key types to be rejected")
			}
		})
	}
}

func TestJoinSupportsTypedCompoundOperands(t *testing.T) {
	dir := joinDir(t)
	tests := []struct {
		name string
		on   string
	}{
		{name: "unary minus and arithmetic", on: "-users.id + 0 = -orders.user_id + 0"},
		{name: "comparison", on: "(users.id > 0) = (orders.user_id > 0)"},
		{name: "NOT and IS NULL", on: "(NOT (users.id IS NULL)) = (NOT (orders.user_id IS NULL))"},
		{name: "AND and OR", on: "((users.id > 0) AND (users.age > 0)) = ((orders.user_id > 0) OR (orders.total < 0))"},
		{name: "numeric coercion", on: "users.id + 0 = orders.user_id / 1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			toks, err := lexer.Lex("SELECT users.name FROM users JOIN orders ON " + tt.on)
			if err != nil {
				t.Fatal(err)
			}
			st, err := parser.New(toks).ParseSelect()
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err := Build(st, joinCatalog(), dir); err != nil {
				t.Fatalf("Build: %v", err)
			}
		})
	}
}

func TestBuildRejectsMultipleJoinsInManualAST(t *testing.T) {
	dir := joinDir(t)
	if err := os.WriteFile(filepath.Join(dir, "shipments.csv"), []byte("100,10\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cat := joinCatalog()
	cat.Add(&catalog.Table{
		Name: "shipments", File: "shipments.csv",
		Columns: []catalog.Column{
			{Name: "id", Type: value.TInt},
			{Name: "order_id", Type: value.TInt},
		},
	})
	st := &ast.SelectStmt{
		Projections: []ast.Projection{{Star: true}},
		From:        "users",
		Joins: []ast.Join{
			{
				Table: "orders",
				On: &ast.BinaryExpr{
					Op:    "=",
					Left:  &ast.ColumnRef{Table: "users", Name: "id"},
					Right: &ast.ColumnRef{Table: "orders", Name: "user_id"},
				},
			},
			{
				Table: "shipments",
				On: &ast.BinaryExpr{
					Op:    "=",
					Left:  &ast.ColumnRef{Table: "orders", Name: "id"},
					Right: &ast.ColumnRef{Table: "shipments", Name: "order_id"},
				},
			},
		},
	}
	if _, _, err := Build(st, cat, dir); err == nil {
		t.Fatal("expected manually constructed statement with two joins to be rejected")
	}
}
