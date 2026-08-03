package parser

import (
	"testing"

	"github.com/aybavs/sql-query-engine/internal/ast"
	"github.com/aybavs/sql-query-engine/internal/lexer"
)

func TestParseInnerJoin(t *testing.T) {
	st := parseSelect(t, "SELECT users.name FROM users INNER JOIN orders ON users.id = orders.user_id WHERE orders.total > 10")
	if len(st.Joins) != 1 {
		t.Fatalf("joins = %d, want 1", len(st.Joins))
	}
	j := st.Joins[0]
	if j.Table != "orders" {
		t.Fatalf("join table = %q, want orders", j.Table)
	}
	on, ok := j.On.(*ast.BinaryExpr)
	if !ok || on.Op != "=" {
		t.Fatalf("ON = %#v, want an equality", j.On)
	}
	left, ok := on.Left.(*ast.ColumnRef)
	if !ok || left.Table != "users" || left.Name != "id" {
		t.Fatalf("ON left = %#v, want users.id", on.Left)
	}
	right, ok := on.Right.(*ast.ColumnRef)
	if !ok || right.Table != "orders" || right.Name != "user_id" {
		t.Fatalf("ON right = %#v, want orders.user_id", on.Right)
	}
	if st.Where == nil {
		t.Fatal("WHERE should still parse after the join")
	}
}

func TestParseJoinWithoutInnerKeyword(t *testing.T) {
	st := parseSelect(t, "SELECT * FROM users JOIN orders ON users.id = orders.user_id")
	if len(st.Joins) != 1 || st.Joins[0].Table != "orders" {
		t.Fatalf("joins = %#v", st.Joins)
	}
}

func TestParseJoinRequiresOn(t *testing.T) {
	toks, _ := lexer.Lex("SELECT * FROM users JOIN orders")
	if _, err := New(toks).ParseSelect(); err == nil {
		t.Fatal("expected error when ON is missing")
	}
}

func TestParseRejectsSecondJoin(t *testing.T) {
	toks, _ := lexer.Lex("SELECT * FROM a JOIN b ON a.x = b.x JOIN c ON a.x = c.x")
	if _, err := New(toks).ParseSelect(); err == nil {
		t.Fatal("expected error for a second join (out of scope in M2)")
	}
}
