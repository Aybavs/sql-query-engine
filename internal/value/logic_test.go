package value

import (
	"math"
	"testing"
)

func TestCompareNumeric(t *testing.T) {
	tests := []struct {
		name string
		a, b Value
		want int
	}{
		{name: "ordinary ints", a: Int64(1), b: Int64(2), want: -1},
		{name: "positive 2^53 adjacent ints", a: Int64(9007199254740993), b: Int64(9007199254740992), want: 1},
		{name: "negative 2^53 adjacent ints", a: Int64(-9007199254740993), b: Int64(-9007199254740992), want: -1},
		{name: "int64 max and predecessor", a: Int64(math.MaxInt64), b: Int64(math.MaxInt64 - 1), want: 1},
		{name: "int64 min and successor", a: Int64(math.MinInt64), b: Int64(math.MinInt64 + 1), want: -1},
		{name: "equal int and float", a: Int64(2), b: Float64(2), want: 0},
		{name: "equal at positive 2^53", a: Int64(9007199254740992), b: Float64(9007199254740992), want: 0},
		{name: "int above rounded positive float", a: Int64(9007199254740993), b: Float64(9007199254740992), want: 1},
		{name: "int below rounded negative float", a: Int64(-9007199254740993), b: Float64(-9007199254740992), want: -1},
		{name: "int64 max below 2^63 float", a: Int64(math.MaxInt64), b: Float64(1 << 63), want: -1},
		{name: "int64 min equals negative 2^63 float", a: Int64(math.MinInt64), b: Float64(-1 << 63), want: 0},
		{name: "int64 min successor above negative 2^63 float", a: Int64(math.MinInt64 + 1), b: Float64(-1 << 63), want: 1},
		{name: "positive fraction", a: Int64(1), b: Float64(1.5), want: -1},
		{name: "negative fraction", a: Int64(-1), b: Float64(-1.5), want: 1},
		{name: "float before int reverses order", a: Float64(1 << 63), b: Int64(math.MaxInt64), want: 1},
		{name: "positive infinity", a: Int64(math.MaxInt64), b: Float64(math.Inf(1)), want: -1},
		{name: "negative infinity", a: Int64(math.MinInt64), b: Float64(math.Inf(-1)), want: 1},
		{name: "int zero equals negative zero", a: Int64(0), b: Float64(math.Copysign(0, -1)), want: 0},
		{name: "negative zero equals int zero", a: Float64(math.Copysign(0, -1)), b: Int64(0), want: 0},
		{name: "float signed zeros compare equal", a: Float64(math.Copysign(0, -1)), b: Float64(0), want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if ord, known := Compare(tt.a, tt.b); !known || ord != tt.want {
				t.Fatalf("Compare(%v, %v) = (%d, %v), want (%d, true)", tt.a, tt.b, ord, known, tt.want)
			}
		})
	}
}

func TestCompareNullUnknown(t *testing.T) {
	tests := []struct {
		name string
		a, b Value
	}{
		{name: "NULL on right", a: Int64(1), b: NullOf(TInt)},
		{name: "NULL on left", a: NullOf(TFloat), b: Int64(1)},
		{name: "both NULL", a: NullOf(TInt), b: NullOf(TFloat)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if ord, known := Compare(tt.a, tt.b); known {
				t.Fatalf("Compare() = (%d, true), want (_, false)", ord)
			}
		})
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
