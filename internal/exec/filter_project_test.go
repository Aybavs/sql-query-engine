package exec

import (
	"testing"

	"github.com/aybavs/sql-query-engine/internal/ast"
	"github.com/aybavs/sql-query-engine/internal/catalog"
	"github.com/aybavs/sql-query-engine/internal/value"
)

func ageScan() *Scan {
	return NewScan(
		Schema{catalog.Column{Name: "age", Type: value.TInt}},
		[]value.Row{{value.Int64(10)}, {value.Int64(20)}, {value.NullOf(value.TInt)}},
	)
}

func TestFilterKeepsOnlyTrue(t *testing.T) {
	// age >= 18 keeps only 20; the NULL row is excluded (unknown, not true).
	pred := &ast.BinaryExpr{Op: ">=", Left: &ast.ColumnRef{Name: "age"}, Right: &ast.Literal{Val: value.Int64(18)}}
	f := NewFilter(ageScan(), pred)
	var got []value.Row
	for {
		r, ok := f.Next()
		if !ok {
			break
		}
		got = append(got, r)
	}
	if len(got) != 1 || got[0][0].I != 20 {
		t.Fatalf("filtered = %v, want [[20]]", got)
	}
}

func TestProject(t *testing.T) {
	expr := &ast.BinaryExpr{Op: "+", Left: &ast.ColumnRef{Name: "age"}, Right: &ast.Literal{Val: value.Int64(1)}}
	out := Schema{catalog.Column{Name: "age_plus", Type: value.TInt}}
	child := NewScan(Schema{catalog.Column{Name: "age", Type: value.TInt}}, []value.Row{{value.Int64(5)}})
	p := NewProject(child, []ast.Expr{expr}, out)
	r, ok := p.Next()
	if !ok || r[0].I != 6 {
		t.Fatalf("projected = %v, want [6]", r)
	}
}
