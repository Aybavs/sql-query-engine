package plan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aybavs/sql-query-engine/internal/ast"
	"github.com/aybavs/sql-query-engine/internal/exec"
	"github.com/aybavs/sql-query-engine/internal/lexer"
	"github.com/aybavs/sql-query-engine/internal/parser"
	"github.com/aybavs/sql-query-engine/internal/value"
)

func TestInferExprType(t *testing.T) {
	s := exec.Schema{{Table: "users", Name: "age", Type: value.TInt}, {Table: "users", Name: "name", Type: value.TText}}
	tests := []struct {
		name string
		expr ast.Expr
		want value.Type
	}{
		{"integer arithmetic", &ast.BinaryExpr{Op: "+", Left: &ast.ColumnRef{Name: "age"}, Right: &ast.Literal{Val: value.Int64(1)}}, value.TInt},
		{"division", &ast.BinaryExpr{Op: "/", Left: &ast.ColumnRef{Name: "age"}, Right: &ast.Literal{Val: value.Int64(2)}}, value.TFloat},
		{"comparison", &ast.BinaryExpr{Op: ">", Left: &ast.ColumnRef{Name: "age"}, Right: &ast.Literal{Val: value.Int64(18)}}, value.TBool},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := inferExprType(tt.expr, s)
			if err != nil || got != tt.want {
				t.Fatalf("inferExprType() = %v, %v; want %v, nil", got, err, tt.want)
			}
		})
	}
}

func TestBuildRejectsIllTypedExpressions(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "users.csv"), []byte("1,alice,30\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, sql := range []string{
		"SELECT name FROM users WHERE age + 1",
		"SELECT name + 1 FROM users",
		"SELECT name FROM users WHERE name > age",
		"SELECT name FROM users ORDER BY name AND TRUE",
	} {
		t.Run(sql, func(t *testing.T) {
			toks, _ := lexer.Lex(sql)
			st, _ := parser.New(toks).ParseSelect()
			if _, _, err := Build(st, testCatalog(), dir); err == nil {
				t.Fatal("Build succeeded; want a type error")
			}
		})
	}
}

func TestBuildUsesPreciseComputedProjectionTypes(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "users.csv"), []byte("1,alice,30\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	toks, _ := lexer.Lex("SELECT age + 1, age / 2, age > 18 FROM users")
	st, _ := parser.New(toks).ParseSelect()
	_, schema, err := Build(st, testCatalog(), dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []value.Type{value.TInt, value.TFloat, value.TBool}
	for i := range want {
		if schema[i].Type != want[i] {
			t.Fatalf("schema[%d].Type = %v, want %v", i, schema[i].Type, want[i])
		}
	}
}

func TestRequireBoolNamesClause(t *testing.T) {
	err := requireBool(&ast.ColumnRef{Name: "age"}, exec.Schema{{Name: "age", Type: value.TInt}}, "WHERE")
	if err == nil || !strings.Contains(err.Error(), "WHERE requires BOOL") {
		t.Fatalf("error = %v", err)
	}
}
