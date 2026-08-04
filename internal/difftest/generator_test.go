package difftest

import (
	"strings"
	"testing"

	"github.com/aybavs/sql-query-engine/internal/lexer"
	"github.com/aybavs/sql-query-engine/internal/parser"
)

func TestGeneratedQueriesParse(t *testing.T) {
	cat, _ := fixture(t)
	g := NewGenerator(1, cat)
	for i := 0; i < 200; i++ {
		q := g.Query()
		toks, err := lexer.Lex(q)
		if err != nil {
			t.Fatalf("query %d does not lex: %q: %v", i, q, err)
		}
		if _, err := parser.New(toks).ParseSelect(); err != nil {
			t.Fatalf("query %d does not parse: %q: %v", i, q, err)
		}
	}
}

func TestGeneratorIsDeterministicPerSeed(t *testing.T) {
	cat, _ := fixture(t)
	a := NewGenerator(42, cat)
	b := NewGenerator(42, cat)
	for i := 0; i < 50; i++ {
		if qa, qb := a.Query(), b.Query(); qa != qb {
			t.Fatalf("same seed diverged at %d:\n  %q\n  %q", i, qa, qb)
		}
	}
}

func TestGeneratorVariesAcrossSeeds(t *testing.T) {
	cat, _ := fixture(t)
	a, b := NewGenerator(1, cat), NewGenerator(2, cat)
	same := 0
	for i := 0; i < 50; i++ {
		if a.Query() == b.Query() {
			same++
		}
	}
	if same > 40 {
		t.Fatalf("different seeds produced %d/50 identical queries", same)
	}
}

// Integer division is a documented divergence: SQLite floors it, this engine
// returns a float. The generator must never emit it.
func TestGeneratorAvoidsKnownDivergences(t *testing.T) {
	cat, _ := fixture(t)
	g := NewGenerator(7, cat)
	for i := 0; i < 300; i++ {
		q := g.Query()
		if strings.Contains(q, "/") {
			t.Fatalf("integer division diverges from SQLite and must not be generated: %q", q)
		}
	}
}

// The grammar should actually exercise the interesting features, not just emit
// trivial SELECTs.
func TestGeneratorCoversTheGrammar(t *testing.T) {
	cat, _ := fixture(t)
	g := NewGenerator(3, cat)
	seen := map[string]bool{}
	for i := 0; i < 400; i++ {
		q := g.Query()
		for _, kw := range []string{"WHERE", "ORDER BY", "LIMIT", "GROUP BY", "HAVING", "IS NULL", "IS NOT NULL", "AND", "OR", "NOT ("} {
			if strings.Contains(q, kw) {
				seen[kw] = true
			}
		}
	}
	for _, kw := range []string{"WHERE", "ORDER BY", "LIMIT", "GROUP BY", "HAVING", "IS NULL", "AND", "OR"} {
		if !seen[kw] {
			t.Errorf("400 queries never produced %q", kw)
		}
	}
}
