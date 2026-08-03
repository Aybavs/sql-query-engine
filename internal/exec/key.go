package exec

import (
	"strconv"

	"github.com/aybavs/sql-query-engine/internal/value"
)

// encodeKey renders a value as a hash-table key. It reports false for NULL,
// which never joins: SQL treats NULL = NULL as unknown, not true.
//
// Ints and floats share one numeric encoding so an INT key matches a FLOAT key
// of the same magnitude, mirroring the comparison rules in package value.
func encodeKey(v value.Value) (string, bool) {
	if v.IsNull() {
		return "", false
	}
	switch v.Type {
	case value.TInt:
		return "n:" + strconv.FormatFloat(float64(v.I), 'g', -1, 64), true
	case value.TFloat:
		return "n:" + strconv.FormatFloat(v.F, 'g', -1, 64), true
	case value.TBool:
		return "b:" + strconv.FormatBool(v.B), true
	default:
		return "s:" + v.S, true
	}
}
