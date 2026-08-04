package difftest

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aybavs/sql-query-engine/internal/catalog"
	"github.com/aybavs/sql-query-engine/internal/value"
)

var (
	seed    = flag.Int64("seed", 20260804, "seed for the query generator")
	queries = flag.Int("queries", 300, "number of generated queries to compare")
)

// diffFixture is deliberately small but full of NULLs, duplicate values, and
// ties, since those are where two engines are most likely to disagree.
func diffFixture(t *testing.T) (*catalog.Catalog, string) {
	t.Helper()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "users.csv"), []byte(
		"1,alice,30,berlin\n"+
			"2,bob,,berlin\n"+
			"3,carol,40,\n"+
			"4,dan,25,london\n"+
			"5,erin,25,paris\n"+
			"6,frank,,\n",
	), 0o644)
	os.WriteFile(filepath.Join(dir, "orders.csv"), []byte(
		"10,1,100\n11,1,300\n12,2,50\n13,,75\n14,5,50\n",
	), 0o644)

	cat := catalog.New()
	cat.Add(&catalog.Table{Name: "users", File: "users.csv", Columns: []catalog.Column{
		{Name: "id", Type: value.TInt},
		{Name: "name", Type: value.TText},
		{Name: "age", Type: value.TInt},
		{Name: "city", Type: value.TText},
	}})
	cat.Add(&catalog.Table{Name: "orders", File: "orders.csv", Columns: []catalog.Column{
		{Name: "id", Type: value.TInt},
		{Name: "user_id", Type: value.TInt},
		{Name: "total", Type: value.TInt},
	}})
	return cat, dir
}

// TestDifferentialAgainstSQLite runs generated queries through this engine and
// through SQLite over identical data, and requires the results to agree.
func TestDifferentialAgainstSQLite(t *testing.T) {
	cat, dir := diffFixture(t)

	oracle, err := NewOracle(cat, dir)
	if err != nil {
		t.Fatalf("oracle: %v", err)
	}
	defer oracle.Close()

	g := NewGenerator(*seed, cat)
	compared, joined := 0, 0
	for i := 0; i < *queries; i++ {
		q := g.Query()
		if strings.Contains(q, " JOIN ") {
			joined++
		}

		engineRows, engErr := runEngine(cat, dir, q)
		oracleRows, orcErr := oracle.Query(q)

		// Both rejecting a query is uninteresting; disagreeing about whether a
		// query is even valid is a finding worth reporting.
		switch {
		case engErr != nil && orcErr != nil:
			continue
		case engErr != nil:
			t.Fatalf("seed %d query %d: engine rejected a query SQLite accepted\n  %s\n  %v", *seed, i, q, engErr)
		case orcErr != nil:
			t.Fatalf("seed %d query %d: SQLite rejected a query the engine accepted\n  %s\n  %v", *seed, i, q, orcErr)
		}

		normalized := make([][]string, len(oracleRows))
		for j, r := range oracleRows {
			normalized[j] = normalizeOracleRow(r)
		}
		if err := Compare(engineRows, normalized); err != nil {
			t.Fatalf("seed %d query %d disagrees:\n  %s\n%v", *seed, i, q, err)
		}
		compared++
	}

	if compared < *queries/2 {
		t.Fatalf("only %d/%d generated queries were actually compared; the generator is producing too many invalid queries", compared, *queries)
	}
	// The hash join is the engine's most intricate operator, so it has to be a
	// real share of what the oracle checks, not an occasional accident.
	if joined < *queries/20 {
		t.Fatalf("only %d/%d generated queries joined; the hash join is barely covered", joined, *queries)
	}
	t.Logf("compared %d generated queries against SQLite (%d of them joins, seed %d)", compared, joined, *seed)
}
