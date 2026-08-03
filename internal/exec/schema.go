// Package exec holds the volcano operators and expression evaluation.
package exec

import (
	"fmt"

	"github.com/aybavs/sql-query-engine/internal/catalog"
	"github.com/aybavs/sql-query-engine/internal/value"
)

// Column is one column of a row stream. Table is the qualifier a query may use
// to disambiguate columns with the same name across joined inputs; it is empty
// for computed projections.
type Column struct {
	Table string
	Name  string
	Type  value.Type
}

// Schema is the ordered set of columns of a row stream.
type Schema []Column

// FromTable builds a schema whose columns are qualified by the table name.
func FromTable(t *catalog.Table) Schema {
	s := make(Schema, len(t.Columns))
	for i, c := range t.Columns {
		s[i] = Column{Table: t.Name, Name: c.Name, Type: c.Type}
	}
	return s
}

// Index resolves a (table, name) reference to a column position. A non-empty
// table must match the column's qualifier; an empty table matches by name alone
// and is an error when several columns share that name.
func (s Schema) Index(table, name string) (int, error) {
	found := -1
	for i, c := range s {
		if c.Name != name {
			continue
		}
		if table != "" && c.Table != table {
			continue
		}
		if found != -1 {
			return 0, fmt.Errorf("ambiguous column %q", name)
		}
		found = i
	}
	if found == -1 {
		if table != "" {
			return 0, fmt.Errorf("unknown column %q.%q", table, name)
		}
		return 0, fmt.Errorf("unknown column %q", name)
	}
	return found, nil
}
