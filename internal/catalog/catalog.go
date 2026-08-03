// Package catalog holds table schemas and resolves table/column references.
package catalog

import "github.com/aybavs/sql-query-engine/internal/value"

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
