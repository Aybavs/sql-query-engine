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
		switch {
		case a.Type == TInt && b.Type == TInt:
			return cmpInt(a.I, b.I), true
		case a.Type == TFloat && b.Type == TFloat:
			return cmpFloat(a.F, b.F), true
		case a.Type == TInt:
			return cmpIntFloat(a.I, b.F), true
		default:
			return -cmpIntFloat(b.I, a.F), true
		}
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

func cmpInt(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func cmpIntFloat(i int64, f float64) int {
	const (
		minInt64Float       = -1 << 63
		maxInt64FloatCutoff = 1 << 63
	)

	// Screen values outside int64's range before converting f. These checks
	// also order negative and positive infinity without a special case.
	switch {
	case f >= maxInt64FloatCutoff:
		return -1
	case f < minInt64Float:
		return 1
	}

	truncated := int64(f)
	switch {
	case i < truncated:
		return -1
	case i > truncated:
		return 1
	case f == float64(truncated):
		return 0
	case f > 0:
		return -1
	default:
		return 1
	}
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
