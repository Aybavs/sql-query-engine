package exec

import "github.com/aybavs/sql-query-engine/internal/value"

// Operator is a pull-based (volcano) node: Next returns the next row and false
// when exhausted. Operators compose into a tree; the root is pulled until it
// reports exhaustion.
type Operator interface {
	Schema() Schema
	Next() (value.Row, bool)
}

// Scan yields pre-loaded rows.
type Scan struct {
	schema Schema
	rows   []value.Row
	i      int
}

func NewScan(schema Schema, rows []value.Row) *Scan {
	return &Scan{schema: schema, rows: rows}
}

func (s *Scan) Schema() Schema { return s.schema }

func (s *Scan) Next() (value.Row, bool) {
	if s.i >= len(s.rows) {
		return nil, false
	}
	row := s.rows[s.i]
	s.i++
	return row, true
}
