package exec

import (
	"testing"

	"github.com/aybavs/sql-query-engine/internal/ast"
	"github.com/aybavs/sql-query-engine/internal/value"
)

func drainAges(op Operator) []int64 {
	var out []int64
	for {
		r, ok := op.Next()
		if !ok {
			return out
		}
		if r[0].IsNull() {
			out = append(out, -1)
		} else {
			out = append(out, r[0].I)
		}
	}
}

func TestSortDesc(t *testing.T) {
	sc := NewScan(Schema{{Name: "age", Type: value.TInt}},
		[]value.Row{{value.Int64(10)}, {value.Int64(30)}, {value.Int64(20)}})
	s := NewSort(sc, []SortKey{{Expr: &ast.ColumnRef{Name: "age"}, Desc: true}})
	if got := drainAges(s); len(got) != 3 || got[0] != 30 || got[2] != 10 {
		t.Fatalf("sorted = %v, want [30 20 10]", got)
	}
}

func TestSortNullsFirstAscending(t *testing.T) {
	sc := NewScan(Schema{{Name: "age", Type: value.TInt}},
		[]value.Row{{value.Int64(10)}, {value.NullOf(value.TInt)}})
	s := NewSort(sc, []SortKey{{Expr: &ast.ColumnRef{Name: "age"}}})
	got := drainAges(s)
	if len(got) != 2 || got[0] != -1 {
		t.Fatalf("sorted = %v, want NULL first", got)
	}
}

func TestLimit(t *testing.T) {
	sc := NewScan(Schema{{Name: "age", Type: value.TInt}},
		[]value.Row{{value.Int64(1)}, {value.Int64(2)}, {value.Int64(3)}})
	l := NewLimit(sc, 2)
	if got := drainAges(l); len(got) != 2 {
		t.Fatalf("limited to %v, want 2 rows", got)
	}
}
