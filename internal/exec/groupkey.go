package exec

import (
	"encoding/binary"
	"math"

	"github.com/aybavs/sql-query-engine/internal/value"
)

// groupKey returns a collision-safe, canonical encoding of a grouping tuple.
func groupKey(values []value.Value) string {
	key := make([]byte, 0, len(values)*9)
	for _, v := range values {
		switch {
		case v.IsNull():
			key = append(key, 'n')
			key = binary.BigEndian.AppendUint64(key, uint64(v.Type))
		case v.Type == value.TInt || v.Type == value.TFloat:
			domain, bits := canonicalNumber(v)
			key = append(key, domain)
			key = binary.BigEndian.AppendUint64(key, bits)
		case v.Type == value.TText:
			key = append(key, 's')
			key = binary.BigEndian.AppendUint64(key, uint64(len(v.S)))
			key = append(key, v.S...)
		case v.Type == value.TBool:
			key = append(key, 'b')
			if v.B {
				key = append(key, 1)
			} else {
				key = append(key, 0)
			}
		}
	}
	return string(key)
}

func canonicalNumber(v value.Value) (domain byte, bits uint64) {
	if v.Type == value.TInt {
		return 'i', uint64(v.I)
	}
	f := v.F
	if f == 0 {
		return 'i', 0
	}
	if math.IsNaN(f) {
		return 'f', 0x7ff8000000000000
	}
	if f >= math.MinInt64 && f < 9223372036854775808.0 {
		i := int64(f)
		if float64(i) == f {
			return 'i', uint64(i)
		}
	}
	return 'f', math.Float64bits(f)
}
