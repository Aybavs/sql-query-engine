package exec

import "github.com/aybavs/sql-query-engine/internal/value"

// Limit passes through at most n rows, then reports exhaustion. Because the
// pipeline is pull-based, the child stops being read once the cap is reached.
type Limit struct {
	child Operator
	n     int
	seen  int
}

func NewLimit(child Operator, n int) *Limit {
	return &Limit{child: child, n: n}
}

func (l *Limit) Schema() Schema { return l.child.Schema() }

func (l *Limit) Next() (value.Row, bool) {
	if l.seen >= l.n {
		return nil, false
	}
	row, ok := l.child.Next()
	if !ok {
		return nil, false
	}
	l.seen++
	return row, true
}
