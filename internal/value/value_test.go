package value

import "testing"

func TestConstructorsAndNull(t *testing.T) {
	if !NullOf(TInt).IsNull() {
		t.Fatal("NullOf(TInt) must be null")
	}
	if Int64(5).IsNull() {
		t.Fatal("Int64(5) must not be null")
	}
	if Int64(5).I != 5 || Int64(5).Type != TInt {
		t.Fatal("Int64 fields wrong")
	}
	if Text("hi").S != "hi" || Text("hi").Type != TText {
		t.Fatal("Text fields wrong")
	}
}

func TestString(t *testing.T) {
	cases := map[string]Value{
		"5":    Int64(5),
		"1.5":  Float64(1.5),
		"hi":   Text("hi"),
		"true": Bool(true),
		"NULL": NullOf(TInt),
	}
	for want, v := range cases {
		if got := v.String(); got != want {
			t.Fatalf("String() = %q, want %q", got, want)
		}
	}
}
