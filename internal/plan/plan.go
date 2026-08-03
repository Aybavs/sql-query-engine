// Package plan turns a parsed statement into a validated operator tree.
package plan

import (
	"fmt"
	"path/filepath"

	"github.com/aybavs/sql-query-engine/internal/ast"
	"github.com/aybavs/sql-query-engine/internal/catalog"
	"github.com/aybavs/sql-query-engine/internal/csv"
	"github.com/aybavs/sql-query-engine/internal/exec"
)

// Build validates st against the catalog, loads the tables, and returns the
// operator tree together with its output schema. Column references are resolved
// here, before execution, so operators never fail on unknown names.
func Build(st *ast.SelectStmt, cat *catalog.Catalog, dataDir string) (exec.Operator, exec.Schema, error) {
	if len(st.Joins) > 1 {
		return nil, nil, fmt.Errorf("only a single JOIN is supported")
	}

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

	if containsAggregate(st.Where) {
		return nil, nil, fmt.Errorf("WHERE cannot contain aggregates")
	}
	aggregateQuery := isAggregateQuery(st)
	if st.Having != nil && !aggregateQuery {
		return nil, nil, fmt.Errorf("HAVING requires an aggregate query")
	}

	if st.Where != nil {
		if err := requireBool(st.Where, schema, "WHERE"); err != nil {
			return nil, nil, err
		}
		op = exec.NewFilter(op, st.Where)
	}

	if aggregateQuery {
		aggOp, aggSchema, loweredProjections, loweredHaving, loweredOrderBy, err := buildAggregatePlan(st, op, schema)
		if err != nil {
			return nil, nil, err
		}
		op, schema = aggOp, aggSchema
		if loweredHaving != nil {
			op = exec.NewFilter(op, loweredHaving)
		}
		if len(loweredOrderBy) > 0 {
			keys := make([]exec.SortKey, 0, len(loweredOrderBy))
			for _, item := range loweredOrderBy {
				keys = append(keys, exec.SortKey{Expr: item.Expr, Desc: item.Desc})
			}
			op = exec.NewSort(op, keys)
		}
		out := make(exec.Schema, len(loweredProjections))
		for i, e := range loweredProjections {
			t, err := inferExprType(e, schema)
			if err != nil {
				return nil, nil, err
			}
			out[i] = exec.Column{Name: exprName(st.Projections[i].Expr), Type: t}
		}
		op = exec.NewProject(op, loweredProjections, out)
		if st.Limit != nil {
			op = exec.NewLimit(op, *st.Limit)
		}
		return op, out, nil
	}

	if len(st.OrderBy) > 0 {
		keys := make([]exec.SortKey, 0, len(st.OrderBy))
		for _, o := range st.OrderBy {
			if _, err := inferExprType(o.Expr, schema); err != nil {
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
		leftKey, rightKey = eq.Left, eq.Right
	case leftOwner == rightJoinOwner && rightOwner == leftJoinOwner:
		leftKey, rightKey = eq.Right, eq.Left
	default:
		return nil, nil, fmt.Errorf("join condition must compare a column from each joined table")
	}

	leftType, err := inferExprType(leftKey, left)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid left join key: %w", err)
	}
	rightType, err := inferExprType(rightKey, right)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid right join key: %w", err)
	}
	if !comparableTypes(leftType, rightType) {
		return nil, nil, fmt.Errorf("incompatible join key types")
	}
	return leftKey, rightKey, nil
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
		t, err := inferExprType(pr.Expr, s)
		if err != nil {
			return nil, nil, err
		}
		exprs = append(exprs, pr.Expr)
		out = append(out, exec.Column{Name: exprName(pr.Expr), Type: t})
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
