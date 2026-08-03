package exec

import (
	"github.com/aybavs/sql-query-engine/internal/ast"
	"github.com/aybavs/sql-query-engine/internal/value"
)

// Filter passes through rows whose predicate evaluates to exactly true.
// Rows where the predicate is false or unknown (NULL) are dropped, which is
// what SQL's WHERE requires.
type Filter struct {
	child Operator
	pred  ast.Expr
}

func NewFilter(child Operator, pred ast.Expr) *Filter {
	return &Filter{child: child, pred: pred}
}

func (f *Filter) Schema() Schema { return f.child.Schema() }

func (f *Filter) Next() (value.Row, bool) {
	for {
		row, ok := f.child.Next()
		if !ok {
			return nil, false
		}
		v, err := Eval(f.pred, row, f.child.Schema())
		if err != nil {
			// The planner validates predicates, so an evaluation error here is
			// unexpected; skip the row rather than aborting the scan.
			continue
		}
		if !v.IsNull() && v.Type == value.TBool && v.B {
			return row, true
		}
	}
}
