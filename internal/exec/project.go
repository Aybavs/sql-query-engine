package exec

import (
	"github.com/aybavs/sql-query-engine/internal/ast"
	"github.com/aybavs/sql-query-engine/internal/value"
)

// Project computes a new row from a list of expressions.
type Project struct {
	child Operator
	exprs []ast.Expr
	out   Schema
}

func NewProject(child Operator, exprs []ast.Expr, out Schema) *Project {
	return &Project{child: child, exprs: exprs, out: out}
}

func (p *Project) Schema() Schema { return p.out }

func (p *Project) Next() (value.Row, bool) {
	row, ok := p.child.Next()
	if !ok {
		return nil, false
	}
	out := make(value.Row, len(p.exprs))
	for i, e := range p.exprs {
		v, err := Eval(e, row, p.child.Schema())
		if err != nil {
			return nil, false
		}
		out[i] = v
	}
	return out, true
}
