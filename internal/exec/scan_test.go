package exec

import (
	"testing"

	"github.com/aybavs/sql-query-engine/internal/value"
)

func TestScanEmitsAllRows(t *testing.T) {
	sc := NewScan(
		Schema{{Name: "id", Type: value.TInt}},
		[]value.Row{{value.Int64(1)}, {value.Int64(2)}},
	)
	var got []int64
	for {
		row, ok := sc.Next()
		if !ok {
			break
		}
		got = append(got, row[0].I)
	}
	if len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("scanned %v, want [1 2]", got)
	}
}
