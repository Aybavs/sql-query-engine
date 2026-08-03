package exec

import (
	"fmt"
	"math"
	"sort"
	"testing"

	"github.com/aybavs/sql-query-engine/internal/ast"
	"github.com/aybavs/sql-query-engine/internal/value"
)

func usersOp(rows ...value.Row) *Scan {
	return NewScan(Schema{
		{Table: "users", Name: "id", Type: value.TInt},
		{Table: "users", Name: "name", Type: value.TText},
	}, rows)
}

func ordersOp(rows ...value.Row) *Scan {
	return NewScan(Schema{
		{Table: "orders", Name: "user_id", Type: value.TInt},
		{Table: "orders", Name: "total", Type: value.TInt},
	}, rows)
}

func drain(op Operator) []string {
	var out []string
	for {
		r, ok := op.Next()
		if !ok {
			sort.Strings(out)
			return out
		}
		var s string
		for i, v := range r {
			if i > 0 {
				s += ","
			}
			s += v.String()
		}
		out = append(out, s)
	}
}

func joinOps(l, r Operator) *HashJoin {
	return NewHashJoin(l, r,
		&ast.ColumnRef{Table: "users", Name: "id"},
		&ast.ColumnRef{Table: "orders", Name: "user_id"},
	)
}

func TestHashJoinMatches(t *testing.T) {
	l := usersOp(
		value.Row{value.Int64(1), value.Text("alice")},
		value.Row{value.Int64(2), value.Text("bob")},
	)
	r := ordersOp(
		value.Row{value.Int64(1), value.Int64(100)},
		value.Row{value.Int64(2), value.Int64(200)},
	)
	got := drain(joinOps(l, r))
	want := []string{"1,alice,1,100", "2,bob,2,200"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("joined = %v, want %v", got, want)
	}
}

func TestHashJoinDuplicateKeys(t *testing.T) {
	// user 1 has two orders: both must be emitted.
	l := usersOp(value.Row{value.Int64(1), value.Text("alice")})
	r := ordersOp(
		value.Row{value.Int64(1), value.Int64(100)},
		value.Row{value.Int64(1), value.Int64(300)},
	)
	got := drain(joinOps(l, r))
	if len(got) != 2 {
		t.Fatalf("joined = %v, want 2 rows for duplicate keys", got)
	}
}

func TestHashJoinNullKeysNeverMatch(t *testing.T) {
	l := usersOp(value.Row{value.NullOf(value.TInt), value.Text("ghost")})
	r := ordersOp(value.Row{value.NullOf(value.TInt), value.Int64(100)})
	if got := drain(joinOps(l, r)); len(got) != 0 {
		t.Fatalf("joined = %v, want no rows (NULL = NULL is unknown)", got)
	}
}

func TestHashJoinNonMatchingDropped(t *testing.T) {
	l := usersOp(value.Row{value.Int64(9), value.Text("nobody")})
	r := ordersOp(value.Row{value.Int64(1), value.Int64(100)})
	if got := drain(joinOps(l, r)); len(got) != 0 {
		t.Fatalf("joined = %v, want no rows", got)
	}
}

func TestHashJoinMatchesEquivalentIntAndFloatKeys(t *testing.T) {
	l := usersOp(value.Row{value.Int64(7), value.Text("alice")})
	r := ordersOp(value.Row{value.Float64(7), value.Int64(100)})
	got := drain(joinOps(l, r))
	want := []string{"7,alice,7,100"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("joined = %v, want %v", got, want)
	}
}

func TestHashJoinMatchesNegativeFloatZeroWithIntegerZero(t *testing.T) {
	l := usersOp(value.Row{value.Int64(0), value.Text("zero")})
	r := ordersOp(value.Row{value.Float64(math.Copysign(0, -1)), value.Int64(100)})
	got := drain(joinOps(l, r))
	want := []string{"0,zero,-0,100"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("joined = %v, want %v", got, want)
	}
}

func TestHashJoinNaNKeysNeverMatch(t *testing.T) {
	nan := value.Float64(math.NaN())
	tests := []struct {
		name        string
		left, right value.Value
	}{
		{name: "NaN probe versus finite build", left: nan, right: value.Float64(1)},
		{name: "finite probe versus NaN build", left: value.Float64(1), right: nan},
		{name: "NaN probe versus NaN build", left: nan, right: nan},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			left := NewScan(
				Schema{{Table: "left", Name: "key", Type: value.TFloat}},
				[]value.Row{{tt.left}},
			)
			right := NewScan(
				Schema{{Table: "right", Name: "key", Type: value.TFloat}},
				[]value.Row{{tt.right}},
			)
			join := NewHashJoin(left, right,
				&ast.ColumnRef{Table: "left", Name: "key"},
				&ast.ColumnRef{Table: "right", Name: "key"},
			)
			if got := drain(join); len(got) != 0 {
				t.Fatalf("joined = %v, want no rows", got)
			}
		})
	}
}

func TestEncodeKeyRejectsNaN(t *testing.T) {
	if key, ok := encodeKey(value.Float64(math.NaN())); ok {
		t.Fatalf("encodeKey(NaN) = (%q, true), want (_, false)", key)
	}
}

func TestHashJoinSchemaIsConcatenated(t *testing.T) {
	j := joinOps(usersOp(), ordersOp())
	s := j.Schema()
	if len(s) != 4 || s[0].Table != "users" || s[2].Table != "orders" {
		t.Fatalf("schema = %#v, want users columns then orders columns", s)
	}
}
