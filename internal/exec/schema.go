// Package exec holds the volcano operators and expression evaluation.
package exec

import (
	"fmt"

	"github.com/aybavs/sql-query-engine/internal/catalog"
)

// Schema is the ordered set of columns of a row stream.
type Schema []catalog.Column

// Index resolves a (table, name) reference to a column position. An empty table
// matches by name alone; the reference must be unambiguous.
func (s Schema) Index(table, name string) (int, error) {
	found := -1
	for i, c := range s {
		if c.Name != name {
			continue
		}
		if found != -1 {
			return 0, fmt.Errorf("ambiguous column %q", name)
		}
		found = i
	}
	if found == -1 {
		return 0, fmt.Errorf("unknown column %q", name)
	}
	return found, nil
}
