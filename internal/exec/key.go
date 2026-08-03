package exec

import (
	"strconv"

	"github.com/aybavs/sql-query-engine/internal/value"
)

// encodeKey renders a value as a hash-table key. It reports false for NULL and
// NaN, which never join because comparisons involving them are unknown.
//
// Ints and floats share one numeric encoding so an INT key matches a FLOAT key
// of the same magnitude, mirroring the comparison rules in package value.
func encodeKey(v value.Value) (string, bool) {
	if v.IsNull() || v.IsNaN() {
		return "", false
	}
	switch v.Type {
	case value.TInt:
		f := float64(v.I)
		if f == 0 {
			f = 0
		}
		return "n:" + strconv.FormatFloat(f, 'g', -1, 64), true
	case value.TFloat:
		f := v.F
		if f == 0 {
			f = 0
		}
		return "n:" + strconv.FormatFloat(f, 'g', -1, 64), true
	case value.TBool:
		return "b:" + strconv.FormatBool(v.B), true
	default:
		return "s:" + v.S, true
	}
}
