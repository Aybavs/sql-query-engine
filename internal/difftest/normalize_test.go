package difftest

import (
	"testing"

	"github.com/aybavs/sql-query-engine/internal/value"
)

func TestNormalizeMatchesIntAndFloat(t *testing.T) {
	eng := normalizeEngineRow(value.Row{value.Int64(35)})
	orc := normalizeOracleRow(Row{float64(35)})
	if eng[0] != orc[0] {
		t.Fatalf("int 35 = %q but float 35.0 = %q; they must normalize alike", eng[0], orc[0])
	}
}

func TestNormalizeNull(t *testing.T) {
	eng := normalizeEngineRow(value.Row{value.NullOf(value.TInt)})
	orc := normalizeOracleRow(Row{nil})
	if eng[0] != orc[0] {
		t.Fatalf("NULL normalized differently: %q vs %q", eng[0], orc[0])
	}
}

func TestNormalizeTextIsNotConfusedWithNull(t *testing.T) {
	eng := normalizeEngineRow(value.Row{value.Text("NULL")})
	orc := normalizeOracleRow(Row{nil})
	if eng[0] == orc[0] {
		t.Fatal("the text 'NULL' must not normalize to the NULL sentinel")
	}
}

func TestNormalizeTextIsNotConfusedWithNumber(t *testing.T) {
	eng := normalizeEngineRow(value.Row{value.Text("35")})
	orc := normalizeOracleRow(Row{int64(35)})
	if eng[0] == orc[0] {
		t.Fatal("the text '35' must not normalize to the number 35")
	}
}

func TestNormalizeAbsorbsFloatNoise(t *testing.T) {
	// An average computed two different ways can differ in the last bits.
	a := normalizeEngineRow(value.Row{value.Float64(35.0)})
	b := normalizeOracleRow(Row{35.000000000000004})
	if a[0] != b[0] {
		t.Fatalf("float noise must not read as a difference: %q vs %q", a[0], b[0])
	}
}

func TestNormalizeKeepsRealNumericDifference(t *testing.T) {
	a := normalizeEngineRow(value.Row{value.Float64(35.0)})
	b := normalizeOracleRow(Row{35.1})
	if a[0] == b[0] {
		t.Fatal("35.0 and 35.1 are genuinely different and must not normalize alike")
	}
}

func TestCompareIgnoresRowOrder(t *testing.T) {
	a := [][]string{{"1"}, {"2"}}
	b := [][]string{{"2"}, {"1"}}
	if err := Compare(a, b); err != nil {
		t.Fatalf("row order must not matter: %v", err)
	}
}

func TestCompareDetectsDifference(t *testing.T) {
	a := [][]string{{"1"}, {"2"}}
	b := [][]string{{"1"}, {"3"}}
	if err := Compare(a, b); err == nil {
		t.Fatal("differing multisets must be reported")
	}
}

func TestCompareDetectsDuplicateCount(t *testing.T) {
	a := [][]string{{"1"}, {"1"}}
	b := [][]string{{"1"}}
	if err := Compare(a, b); err == nil {
		t.Fatal("duplicate counts must matter")
	}
}

func TestCompareEmptyResultsMatch(t *testing.T) {
	if err := Compare(nil, nil); err != nil {
		t.Fatalf("two empty result sets must match: %v", err)
	}
}
