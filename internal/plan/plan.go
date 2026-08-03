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

// Build validates st against the catalog, loads the tables, and returns the
// operator tree together with its output schema. Column references are resolved
// here, before execution, so operators never fail on unknown names.
func Build(st *ast.SelectStmt, cat *catalog.Catalog, dataDir string) (exec.Operator, exec.Schema, error) {
	op, schema, err := scanTable(st.From, cat, dataDir)
	if err != nil {
		return nil, nil, err
	}

	for _, j := range st.Joins {
		right, rightSchema, err := scanTable(j.Table, cat, dataDir)
		if err != nil {
			return nil, nil, err
		}
		leftKey, rightKey, err := splitJoinKeys(j.On, schema, rightSchema)
		if err != nil {
			return nil, nil, err
		}
		op = exec.NewHashJoin(op, right, leftKey, rightKey)
		schema = op.Schema()
	}

	if st.Where != nil {
		if err := validate(st.Where, schema); err != nil {
			return nil, nil, err
		}
		op = exec.NewFilter(op, st.Where)
	}

	if len(st.OrderBy) > 0 {
		keys := make([]exec.SortKey, 0, len(st.OrderBy))
		for _, o := range st.OrderBy {
			if err := validate(o.Expr, schema); err != nil {
				return nil, nil, err
			}
			keys = append(keys, exec.SortKey{Expr: o.Expr, Desc: o.Desc})
		}
		op = exec.NewSort(op, keys)
	}

	exprs, out, err := projections(st, schema)
	if err != nil {
		return nil, nil, err
	}
	op = exec.NewProject(op, exprs, out)

	if st.Limit != nil {
		op = exec.NewLimit(op, *st.Limit)
	}
	return op, out, nil
}

// scanTable loads one table from the catalog into a Scan operator.
func scanTable(name string, cat *catalog.Catalog, dataDir string) (exec.Operator, exec.Schema, error) {
	tbl, ok := cat.Table(name)
	if !ok {
		return nil, nil, fmt.Errorf("unknown table %q", name)
	}
	rows, err := csv.Read(filepath.Join(dataDir, tbl.File), tbl.Columns)
	if err != nil {
		return nil, nil, err
	}
	schema := exec.FromTable(tbl)
	return exec.NewScan(schema, rows), schema, nil
}

// splitJoinKeys turns an ON equality into the key expression evaluated against
// the left input and the one evaluated against the right input. The operand
// order in the query does not matter: each side is assigned by which schema its
// columns resolve against.
func splitJoinKeys(on ast.Expr, left, right exec.Schema) (leftKey, rightKey ast.Expr, err error) {
	eq, ok := on.(*ast.BinaryExpr)
	if !ok || eq.Op != "=" {
		return nil, nil, fmt.Errorf("only equality join conditions are supported")
	}

	leftOwner := joinOperandOwner(eq.Left, left, right)
	rightOwner := joinOperandOwner(eq.Right, left, right)
	switch {
	case leftOwner == leftJoinOwner && rightOwner == rightJoinOwner:
		return eq.Left, eq.Right, nil
	case leftOwner == rightJoinOwner && rightOwner == leftJoinOwner:
		return eq.Right, eq.Left, nil
	default:
		return nil, nil, fmt.Errorf("join condition must compare a column from each joined table")
	}
}

const invalidJoinOwner = -1

const (
	noJoinOwner = iota
	leftJoinOwner
	rightJoinOwner
)

// joinOperandOwner reports whether an expression belongs unambiguously to the
// left or right input. Every column is resolved independently so a compound
// expression cannot hide an ambiguous reference. An operand without a column,
// one that spans both inputs, or one with nested equality has no valid owner.
func joinOperandOwner(e ast.Expr, left, right exec.Schema) int {
	switch n := e.(type) {
	case *ast.Literal:
		return noJoinOwner
	case *ast.ColumnRef:
		validLeft := validate(n, left) == nil
		validRight := validate(n, right) == nil
		if validLeft == validRight {
			return invalidJoinOwner
		}
		if validLeft {
			return leftJoinOwner
		}
		return rightJoinOwner
	case *ast.UnaryExpr:
		return joinOperandOwner(n.Expr, left, right)
	case *ast.IsNull:
		return joinOperandOwner(n.Expr, left, right)
	case *ast.BinaryExpr:
		if n.Op == "=" {
			return invalidJoinOwner
		}
		return combineJoinOwners(
			joinOperandOwner(n.Left, left, right),
			joinOperandOwner(n.Right, left, right),
		)
	default:
		return invalidJoinOwner
	}
}

func combineJoinOwners(left, right int) int {
	switch {
	case left == invalidJoinOwner || right == invalidJoinOwner:
		return invalidJoinOwner
	case left == noJoinOwner:
		return right
	case right == noJoinOwner || left == right:
		return left
	default:
		return invalidJoinOwner
	}
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
