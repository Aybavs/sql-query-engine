package value

import (
	"math"
	"testing"
)

func TestCompareNumeric(t *testing.T) {
	if ord, known := Compare(Int64(1), Int64(2)); !known || ord != -1 {
		t.Fatalf("1 vs 2 = (%d,%v)", ord, known)
	}
	if ord, known := Compare(Int64(2), Float64(2.0)); !known || ord != 0 {
		t.Fatalf("int/float coercion failed: (%d,%v)", ord, known)
	}
}

func TestCompareNullUnknown(t *testing.T) {
	if _, known := Compare(Int64(1), NullOf(TInt)); known {
		t.Fatal("comparison with NULL must be unknown")
	}
}

func TestCompareNaNUnknown(t *testing.T) {
	nan := Float64(math.NaN())
	tests := []struct {
		name string
		a, b Value
	}{
		{name: "NaN versus finite", a: nan, b: Float64(1)},
		{name: "finite versus NaN", a: Float64(1), b: nan},
		{name: "NaN versus NaN", a: nan, b: nan},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if ord, known := Compare(tt.a, tt.b); known {
				t.Fatalf("Compare() = (%d, true), want (_, false)", ord)
			}
		})
	}
}

func TestThreeValuedAnd(t *testing.T) {
	if r := And(Bool(false), NullOf(TBool)); r.Null || r.B != false {
		t.Fatal("false AND null must be false")
	}
	if r := And(Bool(true), NullOf(TBool)); !r.Null {
		t.Fatal("true AND null must be unknown")
	}
}

func TestThreeValuedOr(t *testing.T) {
	if r := Or(Bool(true), NullOf(TBool)); r.Null || r.B != true {
		t.Fatal("true OR null must be true")
	}
	if r := Or(Bool(false), NullOf(TBool)); !r.Null {
		t.Fatal("false OR null must be unknown")
	}
}

func TestNot(t *testing.T) {
	if r := Not(NullOf(TBool)); !r.Null {
		t.Fatal("NOT null must be unknown")
	}
	if r := Not(Bool(true)); r.B != false {
		t.Fatal("NOT true must be false")
	}
}
