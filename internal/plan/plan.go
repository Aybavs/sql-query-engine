// Package plan turns a parsed statement into a validated operator tree.
package plan

import (
	"fmt"
	"path/filepath"

	"github.com/aybavs/sql-query-engine/internal/ast"
	"github.com/aybavs/sql-query-engine/internal/catalog"
	"github.com/aybavs/sql-query-engine/internal/csv"
	"github.com/aybavs/sql-query-engine/internal/exec"
	"github.com/aybavs/sql-query-engine/internal/value"
)

// Build validates st against the catalog, loads the table, and returns the
// operator tree together with its output schema. Column references are resolved
// here, before execution, so operators never fail on unknown names.
func Build(st *ast.SelectStmt, cat *catalog.Catalog, dataDir string) (exec.Operator, exec.Schema, error) {
	tbl, ok := cat.Table(st.From)
	if !ok {
		return nil, nil, fmt.Errorf("unknown table %q", st.From)
	}
	tableSchema := exec.FromTable(tbl)

	rows, err := csv.Read(filepath.Join(dataDir, tbl.File), tbl.Columns)
	if err != nil {
		return nil, nil, err
	}

	var op exec.Operator = exec.NewScan(tableSchema, rows)

	if st.Where != nil {
		if err := validate(st.Where, tableSchema); err != nil {
			return nil, nil, err
		}
		op = exec.NewFilter(op, st.Where)
	}

	if len(st.OrderBy) > 0 {
		keys := make([]exec.SortKey, 0, len(st.OrderBy))
		for _, o := range st.OrderBy {
			if err := validate(o.Expr, tableSchema); err != nil {
				return nil, nil, err
			}
			keys = append(keys, exec.SortKey{Expr: o.Expr, Desc: o.Desc})
		}
		op = exec.NewSort(op, keys)
	}

	exprs, out, err := projections(st, tableSchema)
	if err != nil {
		return nil, nil, err
	}
	op = exec.NewProject(op, exprs, out)

	if st.Limit != nil {
		op = exec.NewLimit(op, *st.Limit)
	}
	return op, out, nil
}

// projections expands `*` and validates explicit projection expressions.
func projections(st *ast.SelectStmt, s exec.Schema) ([]ast.Expr, exec.Schema, error) {
	var exprs []ast.Expr
	var out exec.Schema
	for _, pr := range st.Projections {
		if pr.Star {
			for _, c := range s {
				exprs = append(exprs, &ast.ColumnRef{Table: c.Table, Name: c.Name})
				out = append(out, exec.Column{Table: c.Table, Name: c.Name, Type: c.Type})
			}
			continue
		}
		if err := validate(pr.Expr, s); err != nil {
			return nil, nil, err
		}
		exprs = append(exprs, pr.Expr)
		out = append(out, exec.Column{Name: exprName(pr.Expr), Type: exprType(pr.Expr, s)})
	}
	return exprs, out, nil
}

// validate walks an expression and checks that every column reference resolves.
func validate(e ast.Expr, s exec.Schema) error {
	switch n := e.(type) {
	case *ast.Literal:
		return nil
	case *ast.ColumnRef:
		_, err := s.Index(n.Table, n.Name)
		return err
	case *ast.UnaryExpr:
		return validate(n.Expr, s)
	case *ast.IsNull:
		return validate(n.Expr, s)
	case *ast.BinaryExpr:
		if err := validate(n.Left, s); err != nil {
			return err
		}
		return validate(n.Right, s)
	default:
		return fmt.Errorf("unsupported expression %T", e)
	}
}

func exprName(e ast.Expr) string {
	if c, ok := e.(*ast.ColumnRef); ok {
		return c.Name
	}
	return "expr"
}

// exprType reports the declared type of a projected column. Column references
// carry their catalog type; computed expressions are labelled by their operand
// kind and carry their precise type at runtime on each Value.
func exprType(e ast.Expr, s exec.Schema) value.Type {
	switch n := e.(type) {
	case *ast.ColumnRef:
		if i, err := s.Index(n.Table, n.Name); err == nil {
			return s[i].Type
		}
	case *ast.Literal:
		return n.Val.Type
	case *ast.IsNull:
		return value.TBool
	}
	return value.TFloat
}
