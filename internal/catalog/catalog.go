// Package catalog holds table schemas and resolves table/column references.
package catalog

import (
	"sort"

	"github.com/aybavs/sql-query-engine/internal/value"
)

type Column struct {
	Name string
	Type value.Type
}

type Table struct {
	Name    string
	Columns []Column
	File    string
}

func (t *Table) ColumnIndex(name string) (int, bool) {
	for i, c := range t.Columns {
		if c.Name == name {
			return i, true
		}
	}
	return 0, false
}

type Catalog struct {
	tables map[string]*Table
}

func New() *Catalog { return &Catalog{tables: make(map[string]*Table)} }

func (c *Catalog) Add(t *Table) { c.tables[t.Name] = t }

func (c *Catalog) Table(name string) (*Table, bool) {
	t, ok := c.tables[name]
	return t, ok
}

// Tables returns every registered table, ordered by name so callers that
// iterate the catalog behave deterministically.
func (c *Catalog) Tables() []*Table {
	names := make([]string, 0, len(c.tables))
	for n := range c.tables {
		names = append(names, n)
	}
	sort.Strings(names)

	out := make([]*Table, 0, len(names))
	for _, n := range names {
		out = append(out, c.tables[n])
	}
	return out
}
