package value

import "testing"

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
