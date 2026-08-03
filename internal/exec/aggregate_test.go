package exec

import (
	"math"
	"testing"

	"github.com/aybavs/sql-query-engine/internal/ast"
	"github.com/aybavs/sql-query-engine/internal/value"
)

func drainRows(op Operator) []value.Row {
	var rows []value.Row
	for {
		row, ok := op.Next()
		if !ok {
			return rows
		}
		rows = append(rows, row)
	}
}

func TestAggregateGlobalAllFunctions(t *testing.T) {
	in := Schema{{Name: "n", Type: value.TInt}}
	rows := []value.Row{{value.Int64(2)}, {value.NullOf(value.TInt)}, {value.Int64(4)}}
	expr := &ast.ColumnRef{Name: "n"}
	specs := []AggregateSpec{
		{Kind: AggCount, Star: true, OutType: value.TInt},
		{Kind: AggCount, Expr: expr, OutType: value.TInt},
		{Kind: AggSum, Expr: expr, OutType: value.TInt},
		{Kind: AggAvg, Expr: expr, OutType: value.TFloat},
		{Kind: AggMin, Expr: expr, OutType: value.TInt},
		{Kind: AggMax, Expr: expr, OutType: value.TInt},
	}
	out := Schema{{Name: "count_star", Type: value.TInt}, {Name: "count_n", Type: value.TInt}, {Name: "sum_n", Type: value.TInt}, {Name: "avg_n", Type: value.TFloat}, {Name: "min_n", Type: value.TInt}, {Name: "max_n", Type: value.TInt}}
	got := drainRows(NewAggregate(NewScan(in, rows), nil, specs, out))
	want := []string{"3", "2", "6", "3", "2", "4"}
	if len(got) != 1 {
		t.Fatalf("row count = %d, want 1", len(got))
	}
	for i := range want {
		if got[0][i].String() != want[i] {
			t.Fatalf("cell %d = %s, want %s", i, got[0][i].String(), want[i])
		}
	}
}

func TestAggregateGroupsInFirstSeenOrder(t *testing.T) {
	in := Schema{{Name: "city", Type: value.TText}, {Name: "n", Type: value.TInt}}
	rows := []value.Row{{value.Text("b"), value.Int64(2)}, {value.Text("a"), value.Int64(3)}, {value.Text("b"), value.Int64(4)}}
	out := Schema{{Name: "city", Type: value.TText}, {Name: "sum", Type: value.TInt}}
	op := NewAggregate(NewScan(in, rows), []ast.Expr{&ast.ColumnRef{Name: "city"}}, []AggregateSpec{{Kind: AggSum, Expr: &ast.ColumnRef{Name: "n"}, OutType: value.TInt}}, out)
	got := drainRows(op)
	if len(got) != 2 || got[0][0].String() != "b" || got[0][1].String() != "6" || got[1][0].String() != "a" {
		t.Fatalf("rows = %v", got)
	}
}

func TestAggregateEmptyInput(t *testing.T) {
	in := Schema{{Name: "n", Type: value.TInt}}
	global := drainRows(NewAggregate(NewScan(in, nil), nil, []AggregateSpec{{Kind: AggCount, Star: true, OutType: value.TInt}, {Kind: AggSum, Expr: &ast.ColumnRef{Name: "n"}, OutType: value.TInt}}, Schema{{Type: value.TInt}, {Type: value.TInt}}))
	if len(global) != 1 || global[0][0].String() != "0" || !global[0][1].IsNull() || global[0][1].Type != value.TInt {
		t.Fatalf("global = %#v", global)
	}
	grouped := drainRows(NewAggregate(NewScan(in, nil), []ast.Expr{&ast.ColumnRef{Name: "n"}}, nil, Schema{{Type: value.TInt}}))
	if len(grouped) != 0 {
		t.Fatalf("grouped rows = %d, want 0", len(grouped))
	}
}

func TestAggregateNumericGroupingAndNaN(t *testing.T) {
	in := Schema{{Name: "k", Type: value.TFloat}, {Name: "n", Type: value.TFloat}}
	rows := []value.Row{{value.Float64(0), value.Float64(math.NaN())}, {value.Float64(math.Copysign(0, -1)), value.Float64(2)}, {value.Float64(math.NaN()), value.Float64(math.NaN())}, {value.Float64(math.NaN()), value.Float64(4)}}
	specs := []AggregateSpec{{Kind: AggCount, Expr: &ast.ColumnRef{Name: "n"}, OutType: value.TInt}, {Kind: AggAvg, Expr: &ast.ColumnRef{Name: "n"}, OutType: value.TFloat}, {Kind: AggMin, Expr: &ast.ColumnRef{Name: "n"}, OutType: value.TFloat}, {Kind: AggMax, Expr: &ast.ColumnRef{Name: "n"}, OutType: value.TFloat}}
	got := drainRows(NewAggregate(NewScan(in, rows), []ast.Expr{&ast.ColumnRef{Name: "k"}}, specs, Schema{{Type: value.TFloat}, {Type: value.TInt}, {Type: value.TFloat}, {Type: value.TFloat}, {Type: value.TFloat}}))
	if len(got) != 2 || got[0][1].String() != "2" || got[0][2].String() != "2" || got[0][3].String() != "2" || got[0][4].String() != "2" || got[1][1].String() != "2" || got[1][2].String() != "4" || got[1][3].String() != "4" || got[1][4].String() != "4" {
		t.Fatalf("rows = %#v", got)
	}
}

func TestGroupKeyNumericEquivalenceWithoutIntegerCollisions(t *testing.T) {
	if groupKey([]value.Value{value.Int64(2)}) != groupKey([]value.Value{value.Float64(2)}) {
		t.Fatal("INT 2 and FLOAT 2 must share a key")
	}
	if groupKey([]value.Value{value.Float64(0)}) != groupKey([]value.Value{value.Float64(math.Copysign(0, -1))}) {
		t.Fatal("signed zero must share a key")
	}
	if groupKey([]value.Value{value.Float64(math.NaN())}) != groupKey([]value.Value{value.Float64(math.NaN())}) {
		t.Fatal("NaN values must share a key")
	}
	if groupKey([]value.Value{value.Int64(9007199254740992)}) == groupKey([]value.Value{value.Int64(9007199254740993)}) {
		t.Fatal("distinct large integers collided")
	}
	if groupKey([]value.Value{value.NullOf(value.TInt)}) != groupKey([]value.Value{value.NullOf(value.TInt)}) {
		t.Fatal("NULL values must share a key")
	}
	if groupKey([]value.Value{value.Text("ab"), value.Text("c")}) == groupKey([]value.Value{value.Text("a"), value.Text("bc")}) {
		t.Fatal("composite text boundaries collided")
	}
}

func TestAggregateMinMaxTextAndBool(t *testing.T) {
	in := Schema{{Name: "text", Type: value.TText}, {Name: "flag", Type: value.TBool}}
	rows := []value.Row{{value.Text("z"), value.Bool(true)}, {value.Text("a"), value.Bool(false)}}
	specs := []AggregateSpec{{Kind: AggMin, Expr: &ast.ColumnRef{Name: "text"}, OutType: value.TText}, {Kind: AggMax, Expr: &ast.ColumnRef{Name: "text"}, OutType: value.TText}, {Kind: AggMin, Expr: &ast.ColumnRef{Name: "flag"}, OutType: value.TBool}, {Kind: AggMax, Expr: &ast.ColumnRef{Name: "flag"}, OutType: value.TBool}}
	got := drainRows(NewAggregate(NewScan(in, rows), nil, specs, Schema{{Type: value.TText}, {Type: value.TText}, {Type: value.TBool}, {Type: value.TBool}}))
	want := []string{"a", "z", "false", "true"}
	for i := range want {
		if got[0][i].String() != want[i] {
			t.Fatalf("cell %d = %s, want %s", i, got[0][i].String(), want[i])
		}
	}
}
