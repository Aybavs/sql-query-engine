package difftest

import (
	"fmt"
	"sort"
	"strings"

	"github.com/aybavs/sql-query-engine/internal/value"
)

// nullCell is a sentinel no literal in the generated grammar can produce, so a
// text value never masquerades as NULL.
const nullCell = "∅"

// normalizeEngineRow renders one engine row as comparable cells. Cells carry a
// type tag so the text "35" cannot compare equal to the number 35.
func normalizeEngineRow(r value.Row) []string {
	out := make([]string, len(r))
	for i, v := range r {
		switch {
		case v.IsNull():
			out[i] = nullCell
		case v.Type == value.TInt:
			out[i] = num(float64(v.I))
		case v.Type == value.TFloat:
			out[i] = num(v.F)
		case v.Type == value.TBool:
			out[i] = fmt.Sprintf("b:%t", v.B)
		default:
			out[i] = "s:" + v.S
		}
	}
	return out
}

// normalizeOracleRow renders one SQLite driver row the same way.
func normalizeOracleRow(r Row) []string {
	out := make([]string, len(r))
	for i, cell := range r {
		switch v := cell.(type) {
		case nil:
			out[i] = nullCell
		case int64:
			out[i] = num(float64(v))
		case float64:
			out[i] = num(v)
		case bool:
			out[i] = fmt.Sprintf("b:%t", v)
		case []byte:
			out[i] = "s:" + string(v)
		case string:
			out[i] = "s:" + v
		default:
			out[i] = fmt.Sprintf("?:%v", v)
		}
	}
	return out
}

// num renders a number so an integer and an equal float compare the same.
// SQLite returns an average as a float where this engine may return an int, and
// the two can disagree in the last bits; six significant digits absorb that
// noise while still separating genuinely different values.
func num(f float64) string { return fmt.Sprintf("n:%.6g", f) }

// canonical sorts rows so two result sets can be compared as multisets. Row
// order is unspecified in SQL without ORDER BY, so the comparison must not
// depend on it — but duplicates still have to match.
func canonical(rows [][]string) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = strings.Join(r, "\x1f")
	}
	sort.Strings(out)
	return out
}

// Compare reports whether two result sets hold the same rows, ignoring order
// but respecting duplicates.
func Compare(engine, oracle [][]string) error {
	a, b := canonical(engine), canonical(oracle)
	if len(a) != len(b) {
		return fmt.Errorf("row count: engine %d, sqlite %d\n  engine: %v\n  sqlite: %v", len(a), len(b), a, b)
	}
	for i := range a {
		if a[i] != b[i] {
			return fmt.Errorf("row %d differs:\n  engine: %s\n  sqlite: %s", i, a[i], b[i])
		}
	}
	return nil
}
