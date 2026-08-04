package exec

import (
	"testing"

	"github.com/aybavs/sql-query-engine/internal/ast"
	"github.com/aybavs/sql-query-engine/internal/value"
)

func testSchema() Schema {
	return Schema{
		{Name: "age", Type: value.TInt},
		{Name: "city", Type: value.TText},
	}
}

func TestEvalComparison(t *testing.T) {
	e := &ast.BinaryExpr{Op: ">=", Left: &ast.ColumnRef{Name: "age"}, Right: &ast.Literal{Val: value.Int64(18)}}
	got, err := Eval(e, value.Row{value.Int64(20), value.Text("x")}, testSchema())
	if err != nil {
		t.Fatal(err)
	}
	if got.Null || got.B != true {
		t.Fatalf("got %v, want true", got)
	}
}

func TestEvalNullComparisonIsUnknown(t *testing.T) {
	e := &ast.BinaryExpr{Op: ">=", Left: &ast.ColumnRef{Name: "age"}, Right: &ast.Literal{Val: value.Int64(18)}}
	got, _ := Eval(e, value.Row{value.NullOf(value.TInt), value.Text("x")}, testSchema())
	if !got.Null {
		t.Fatal("comparison with NULL must be unknown")
	}
}

func TestEvalIsNull(t *testing.T) {
	e := &ast.IsNull{Expr: &ast.ColumnRef{Name: "city"}}
	got, _ := Eval(e, value.Row{value.Int64(1), value.NullOf(value.TText)}, testSchema())
	if got.B != true {
		t.Fatal("city IS NULL should be true when city is NULL")
	}
}

func TestEvalUnknownColumn(t *testing.T) {
	e := &ast.ColumnRef{Name: "nope"}
	if _, err := Eval(e, value.Row{}, testSchema()); err == nil {
		t.Fatal("expected error for unknown column")
	}
}

func TestEvalArithmeticNullKeepsInferredType(t *testing.T) {
	tests := []struct {
		op   string
		want value.Type
	}{{"+", value.TInt}, {"/", value.TFloat}}
	for _, tt := range tests {
		e := &ast.BinaryExpr{Op: tt.op, Left: &ast.ColumnRef{Name: "age"}, Right: &ast.Literal{Val: value.Int64(2)}}
		got, err := Eval(e, value.Row{value.NullOf(value.TInt), value.Text("x")}, testSchema())
		if err != nil {
			t.Fatal(err)
		}
		if !got.IsNull() || got.Type != tt.want {
			t.Fatalf("%s result = %#v, want NULL type %v", tt.op, got, tt.want)
		}
	}
}
