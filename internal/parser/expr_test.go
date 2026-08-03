package parser

import (
	"testing"

	"github.com/aybavs/sql-query-engine/internal/ast"
	"github.com/aybavs/sql-query-engine/internal/lexer"
)

func parseExpr(t *testing.T, s string) ast.Expr {
	t.Helper()
	toks, err := lexer.Lex(s)
	if err != nil {
		t.Fatalf("lex: %v", err)
	}
	e, err := New(toks).ParseExpr()
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return e
}

func TestParsePrecedence(t *testing.T) {
	// age >= 18 AND name <> 'x'  =>  AND(>=(age,18), <>(name,'x'))
	e := parseExpr(t, "age >= 18 AND name <> 'x'")
	and, ok := e.(*ast.BinaryExpr)
	if !ok || and.Op != "AND" {
		t.Fatalf("root = %#v, want AND", e)
	}
	left, ok := and.Left.(*ast.BinaryExpr)
	if !ok || left.Op != ">=" {
		t.Fatalf("left = %#v, want >=", and.Left)
	}
	if col, ok := left.Left.(*ast.ColumnRef); !ok || col.Name != "age" {
		t.Fatalf("left.left = %#v, want column age", left.Left)
	}
}

func TestParseIsNull(t *testing.T) {
	e := parseExpr(t, "city IS NOT NULL")
	n, ok := e.(*ast.IsNull)
	if !ok || !n.Negate {
		t.Fatalf("expected IS NOT NULL, got %#v", e)
	}
}
