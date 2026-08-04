package ast

import (
	"testing"

	"github.com/aybavs/sql-query-engine/internal/value"
)

func TestStringRendersExpressions(t *testing.T) {
	cases := []struct {
		want string
		expr Expr
	}{
		{"age", &ColumnRef{Name: "age"}},
		{"users.age", &ColumnRef{Table: "users", Name: "age"}},
		{"18", &Literal{Val: value.Int64(18)}},
		{"'berlin'", &Literal{Val: value.Text("berlin")}},
		{"NULL", &Literal{Val: value.NullOf(value.TInt)}},
		{
			"age + 1",
			&BinaryExpr{Op: "+", Left: &ColumnRef{Name: "age"}, Right: &Literal{Val: value.Int64(1)}},
		},
		{
			"NOT active",
			&UnaryExpr{Op: "NOT", Expr: &ColumnRef{Name: "active"}},
		},
		{
			"-age",
			&UnaryExpr{Op: "-", Expr: &ColumnRef{Name: "age"}},
		},
		{"city IS NULL", &IsNull{Expr: &ColumnRef{Name: "city"}}},
		{"city IS NOT NULL", &IsNull{Expr: &ColumnRef{Name: "city"}, Negate: true}},
		{"COUNT(*)", &AggregateCall{Name: "COUNT", Star: true}},
		{"COUNT(id)", &AggregateCall{Name: "COUNT", Arg: &ColumnRef{Name: "id"}}},
		{"AVG(users.age)", &AggregateCall{Name: "AVG", Arg: &ColumnRef{Table: "users", Name: "age"}}},
	}
	for _, c := range cases {
		if got := String(c.expr); got != c.want {
			t.Errorf("String() = %q, want %q", got, c.want)
		}
	}
}

func TestStringParenthesizesNestedBinary(t *testing.T) {
	// SUM(total) / COUNT(id) where each operand is itself an expression
	e := &BinaryExpr{
		Op:    "/",
		Left:  &AggregateCall{Name: "SUM", Arg: &ColumnRef{Name: "total"}},
		Right: &AggregateCall{Name: "COUNT", Arg: &ColumnRef{Name: "id"}},
	}
	if got, want := String(e), "SUM(total) / COUNT(id)"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}

	// a nested binary operand is parenthesized so the label stays unambiguous
	nested := &BinaryExpr{
		Op:    "*",
		Left:  &BinaryExpr{Op: "+", Left: &ColumnRef{Name: "a"}, Right: &ColumnRef{Name: "b"}},
		Right: &Literal{Val: value.Int64(2)},
	}
	if got, want := String(nested), "(a + b) * 2"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}
