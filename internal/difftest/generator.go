package difftest

import (
	"fmt"
	"math/rand"
	"strings"

	"github.com/aybavs/sql-query-engine/internal/catalog"
	"github.com/aybavs/sql-query-engine/internal/value"
)

// Generator emits queries from a bounded SQL grammar. It is seeded so a failing
// run can be replayed exactly, and it deliberately avoids constructs where this
// engine and SQLite differ by design (see the README's divergence table):
// integer division, cross-type comparison, and booleans.
type Generator struct {
	rnd    *rand.Rand
	tables []*catalog.Table
}

func NewGenerator(seed int64, cat *catalog.Catalog) *Generator {
	return &Generator{
		rnd:    rand.New(rand.NewSource(seed)),
		tables: cat.Tables(),
	}
}

func (g *Generator) Query() string {
	t := g.tables[g.rnd.Intn(len(g.tables))]
	if g.chance(0.35) {
		return g.aggregateQuery(t)
	}
	return g.selectQuery(t)
}

func (g *Generator) selectQuery(t *catalog.Table) string {
	var b strings.Builder
	b.WriteString("SELECT ")
	b.WriteString(strings.Join(g.projections(t), ", "))
	fmt.Fprintf(&b, " FROM %s", t.Name)

	if g.chance(0.7) {
		fmt.Fprintf(&b, " WHERE %s", g.predicate(t))
	}
	if g.chance(0.5) {
		c := g.column(t)
		fmt.Fprintf(&b, " ORDER BY %s.%s", t.Name, c.Name)
		if g.chance(0.5) {
			b.WriteString(" DESC")
		}
	}
	if g.chance(0.3) {
		fmt.Fprintf(&b, " LIMIT %d", 1+g.rnd.Intn(3))
	}
	return b.String()
}

// aggregateQuery always groups by the column it projects, so the result is
// well-defined rather than relying on SQLite's bare-column extension.
func (g *Generator) aggregateQuery(t *catalog.Table) string {
	group := g.column(t)
	agg := g.aggregate(t)

	var b strings.Builder
	fmt.Fprintf(&b, "SELECT %s.%s, %s FROM %s GROUP BY %s.%s",
		t.Name, group.Name, agg, t.Name, t.Name, group.Name)
	if g.chance(0.3) {
		fmt.Fprintf(&b, " HAVING COUNT(*) > %d", g.rnd.Intn(2))
	}
	return b.String()
}

func (g *Generator) aggregate(t *catalog.Table) string {
	if g.chance(0.25) {
		return "COUNT(*)"
	}
	numeric := g.numericColumns(t)
	if len(numeric) == 0 {
		return "COUNT(*)"
	}
	c := numeric[g.rnd.Intn(len(numeric))]
	fn := []string{"COUNT", "SUM", "AVG", "MIN", "MAX"}[g.rnd.Intn(5)]
	return fmt.Sprintf("%s(%s.%s)", fn, t.Name, c.Name)
}

func (g *Generator) projections(t *catalog.Table) []string {
	if g.chance(0.2) {
		return []string{"*"}
	}
	n := 1 + g.rnd.Intn(2)
	out := make([]string, n)
	for i := range out {
		c := g.column(t)
		out[i] = fmt.Sprintf("%s.%s", t.Name, c.Name)
	}
	return out
}

// predicate builds a boolean expression whose operands always share a type.
func (g *Generator) predicate(t *catalog.Table) string {
	p := g.comparison(t)
	for g.chance(0.35) {
		op := "AND"
		if g.chance(0.5) {
			op = "OR"
		}
		p = fmt.Sprintf("%s %s %s", p, op, g.comparison(t))
	}
	if g.chance(0.15) {
		p = "NOT (" + p + ")"
	}
	return p
}

func (g *Generator) comparison(t *catalog.Table) string {
	c := g.column(t)
	ref := fmt.Sprintf("%s.%s", t.Name, c.Name)

	if g.chance(0.2) {
		if g.chance(0.5) {
			return ref + " IS NULL"
		}
		return ref + " IS NOT NULL"
	}

	op := []string{"=", "<>", "<", "<=", ">", ">="}[g.rnd.Intn(6)]
	return fmt.Sprintf("%s %s %s", ref, op, g.literal(c))
}

// literal produces a value of the column's own type: comparing across types is
// rejected by this engine's type checker but silently coerced by SQLite.
func (g *Generator) literal(c catalog.Column) string {
	switch c.Type {
	case value.TInt:
		return fmt.Sprint(g.rnd.Intn(60))
	case value.TFloat:
		return fmt.Sprintf("%.1f", g.rnd.Float64()*100)
	default:
		words := []string{"alice", "bob", "carol", "dan", "erin", "berlin", "paris", "london", "zzz"}
		return "'" + words[g.rnd.Intn(len(words))] + "'"
	}
}

// column returns a column of a type the generator supports. Booleans are
// excluded because SQLite stores them as integers.
func (g *Generator) column(t *catalog.Table) catalog.Column {
	usable := make([]catalog.Column, 0, len(t.Columns))
	for _, c := range t.Columns {
		if c.Type != value.TBool {
			usable = append(usable, c)
		}
	}
	return usable[g.rnd.Intn(len(usable))]
}

func (g *Generator) numericColumns(t *catalog.Table) []catalog.Column {
	var out []catalog.Column
	for _, c := range t.Columns {
		if c.Type == value.TInt || c.Type == value.TFloat {
			out = append(out, c)
		}
	}
	return out
}

func (g *Generator) chance(p float64) bool { return g.rnd.Float64() < p }
