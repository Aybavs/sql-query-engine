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

// qualified is a column together with the table it came from, so a query over
// several tables can always write an unambiguous reference.
type qualified struct {
	table  *catalog.Table
	column catalog.Column
}

func (q qualified) ref() string { return q.table.Name + "." + q.column.Name }

func (g *Generator) Query() string {
	// A join exercises the hash join, which is the most intricate operator in
	// the engine and therefore the one most worth checking against an oracle.
	if len(g.tables) >= 2 && g.chance(0.3) {
		if q, ok := g.joinQuery(); ok {
			return q
		}
	}

	scope := []*catalog.Table{g.tables[g.rnd.Intn(len(g.tables))]}
	if g.chance(0.35) {
		return g.aggregateQuery(scope, scope[0].Name)
	}
	return g.selectQuery(scope, scope[0].Name)
}

// joinQuery joins two tables on a pair of same-type columns. The pairing need
// not be semantically meaningful — an arbitrary equi-join still exercises
// build/probe, duplicate keys, and NULL keys, which is what is being checked.
func (g *Generator) joinQuery() (string, bool) {
	i := g.rnd.Intn(len(g.tables))
	j := g.rnd.Intn(len(g.tables) - 1)
	if j >= i {
		j++
	}
	left, right := g.tables[i], g.tables[j]

	lc, rc, ok := g.joinKeys(left, right)
	if !ok {
		return "", false
	}

	scope := []*catalog.Table{left, right}
	from := fmt.Sprintf("%s JOIN %s ON %s = %s", left.Name, right.Name, lc.ref(), rc.ref())

	if g.chance(0.3) {
		return g.aggregateQuery(scope, from), true
	}
	return g.selectQuery(scope, from), true
}

// joinKeys picks one column from each table that share a type.
func (g *Generator) joinKeys(left, right *catalog.Table) (qualified, qualified, bool) {
	var pairs [][2]qualified
	for _, lc := range g.usableColumns(left) {
		for _, rc := range g.usableColumns(right) {
			if lc.Type == rc.Type {
				pairs = append(pairs, [2]qualified{{left, lc}, {right, rc}})
			}
		}
	}
	if len(pairs) == 0 {
		return qualified{}, qualified{}, false
	}
	p := pairs[g.rnd.Intn(len(pairs))]
	return p[0], p[1], true
}

func (g *Generator) selectQuery(scope []*catalog.Table, from string) string {
	var b strings.Builder
	b.WriteString("SELECT ")
	b.WriteString(strings.Join(g.projections(scope), ", "))
	fmt.Fprintf(&b, " FROM %s", from)

	if g.chance(0.7) {
		fmt.Fprintf(&b, " WHERE %s", g.predicate(scope))
	}
	if g.chance(0.5) {
		fmt.Fprintf(&b, " ORDER BY %s", g.column(scope).ref())
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
func (g *Generator) aggregateQuery(scope []*catalog.Table, from string) string {
	group := g.column(scope)

	var b strings.Builder
	fmt.Fprintf(&b, "SELECT %s, %s FROM %s GROUP BY %s",
		group.ref(), g.aggregate(scope), from, group.ref())
	if g.chance(0.3) {
		fmt.Fprintf(&b, " HAVING COUNT(*) > %d", g.rnd.Intn(2))
	}
	return b.String()
}

func (g *Generator) aggregate(scope []*catalog.Table) string {
	if g.chance(0.25) {
		return "COUNT(*)"
	}
	numeric := g.numericColumns(scope)
	if len(numeric) == 0 {
		return "COUNT(*)"
	}
	c := numeric[g.rnd.Intn(len(numeric))]
	fn := []string{"COUNT", "SUM", "AVG", "MIN", "MAX"}[g.rnd.Intn(5)]
	return fmt.Sprintf("%s(%s)", fn, c.ref())
}

func (g *Generator) projections(scope []*catalog.Table) []string {
	if g.chance(0.2) {
		return []string{"*"}
	}
	n := 1 + g.rnd.Intn(2)
	out := make([]string, n)
	for i := range out {
		out[i] = g.column(scope).ref()
	}
	return out
}

// predicate builds a boolean expression whose operands always share a type.
func (g *Generator) predicate(scope []*catalog.Table) string {
	p := g.comparison(scope)
	for g.chance(0.35) {
		op := "AND"
		if g.chance(0.5) {
			op = "OR"
		}
		p = fmt.Sprintf("%s %s %s", p, op, g.comparison(scope))
	}
	if g.chance(0.15) {
		p = "NOT (" + p + ")"
	}
	return p
}

func (g *Generator) comparison(scope []*catalog.Table) string {
	c := g.column(scope)

	if g.chance(0.2) {
		if g.chance(0.5) {
			return c.ref() + " IS NULL"
		}
		return c.ref() + " IS NOT NULL"
	}

	op := []string{"=", "<>", "<", "<=", ">", ">="}[g.rnd.Intn(6)]
	return fmt.Sprintf("%s %s %s", c.ref(), op, g.literal(c.column))
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

// column picks a usable column from anywhere in scope.
func (g *Generator) column(scope []*catalog.Table) qualified {
	var all []qualified
	for _, t := range scope {
		for _, c := range g.usableColumns(t) {
			all = append(all, qualified{t, c})
		}
	}
	return all[g.rnd.Intn(len(all))]
}

// usableColumns excludes booleans, which SQLite stores as integers.
func (g *Generator) usableColumns(t *catalog.Table) []catalog.Column {
	out := make([]catalog.Column, 0, len(t.Columns))
	for _, c := range t.Columns {
		if c.Type != value.TBool {
			out = append(out, c)
		}
	}
	return out
}

func (g *Generator) numericColumns(scope []*catalog.Table) []qualified {
	var out []qualified
	for _, t := range scope {
		for _, c := range t.Columns {
			if c.Type == value.TInt || c.Type == value.TFloat {
				out = append(out, qualified{t, c})
			}
		}
	}
	return out
}

func (g *Generator) chance(p float64) bool { return g.rnd.Float64() < p }
