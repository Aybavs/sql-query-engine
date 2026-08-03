package parser

import (
	"testing"

	"github.com/aybavs/sql-query-engine/internal/ast"
	"github.com/aybavs/sql-query-engine/internal/lexer"
)

func parseSelect(t *testing.T, s string) *ast.SelectStmt {
	t.Helper()
	toks, err := lexer.Lex(s)
	if err != nil {
		t.Fatalf("lex: %v", err)
	}
	st, err := New(toks).ParseSelect()
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return st
}

func TestParseSelectStar(t *testing.T) {
	st := parseSelect(t, "SELECT * FROM users")
	if len(st.Projections) != 1 || !st.Projections[0].Star {
		t.Fatalf("expected star projection, got %#v", st.Projections)
	}
	if st.From != "users" {
		t.Fatalf("from = %q", st.From)
	}
}

func TestParseSelectFull(t *testing.T) {
	st := parseSelect(t, "SELECT id, name FROM users WHERE age >= 18 ORDER BY age DESC LIMIT 5")
	if len(st.Projections) != 2 {
		t.Fatalf("projections = %d, want 2", len(st.Projections))
	}
	if st.Where == nil {
		t.Fatal("expected WHERE")
	}
	if len(st.OrderBy) != 1 || !st.OrderBy[0].Desc {
		t.Fatalf("order by = %#v", st.OrderBy)
	}
	if st.Limit == nil || *st.Limit != 5 {
		t.Fatalf("limit = %v", st.Limit)
	}
}

func TestParseTrailingTokens(t *testing.T) {
	toks, _ := lexer.Lex("SELECT * FROM t garbage")
	if _, err := New(toks).ParseSelect(); err == nil {
		t.Fatal("expected error on trailing tokens")
	}
}
