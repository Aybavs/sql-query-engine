package value

// Compare orders two values. If either is NULL or NaN, known is false
// (unknown). NaN is unordered and never equal to any value, including itself.
// Numeric values (TInt/TFloat) are compared numerically; TText and TBool
// compare within their own type. Type compatibility is guaranteed by plan-time
// checks.
func Compare(a, b Value) (ord int, known bool) {
	if a.Null || b.Null || a.IsNaN() || b.IsNaN() {
		return 0, false
	}
	if isNumeric(a.Type) && isNumeric(b.Type) {
		return cmpFloat(toFloat(a), toFloat(b)), true
	}
	switch a.Type {
	case TText:
		return cmpString(a.S, b.S), true
	case TBool:
		return cmpBool(a.B, b.B), true
	}
	return 0, true
}

func isNumeric(t Type) bool { return t == TInt || t == TFloat }

func toFloat(v Value) float64 {
	if v.Type == TInt {
		return float64(v.I)
	}
	return v.F
}

func cmpFloat(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func cmpString(a, b string) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func cmpBool(a, b bool) int {
	switch {
	case a == b:
		return 0
	case !a:
		return -1
	default:
		return 1
	}
}

// And/Or/Not implement SQL three-valued logic over Bool values. A NULL Bool
// represents "unknown".
func And(a, b Value) Value {
	if isFalse(a) || isFalse(b) {
		return Bool(false)
	}
	if a.Null || b.Null {
		return NullOf(TBool)
	}
	return Bool(true)
}

func Or(a, b Value) Value {
	if isTrue(a) || isTrue(b) {
		return Bool(true)
	}
	if a.Null || b.Null {
		return NullOf(TBool)
	}
	return Bool(false)
}

func Not(a Value) Value {
	if a.Null {
		return NullOf(TBool)
	}
	return Bool(!a.B)
}

func isTrue(v Value) bool  { return !v.Null && v.Type == TBool && v.B }
func isFalse(v Value) bool { return !v.Null && v.Type == TBool && !v.B }
