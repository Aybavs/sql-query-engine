package plan

import (
	"fmt"

	"github.com/aybavs/sql-query-engine/internal/ast"
	"github.com/aybavs/sql-query-engine/internal/exec"
	"github.com/aybavs/sql-query-engine/internal/value"
)

// inferExprType validates an expression and reports its runtime type.
// It mirrors the expression forms supported by exec.Eval without adding SQL
// syntax: numeric arithmetic, boolean logic, comparisons, and IS NULL.
func inferExprType(e ast.Expr, s exec.Schema) (value.Type, error) {
	switch n := e.(type) {
	case *ast.Literal:
		return n.Val.Type, nil
	case *ast.ColumnRef:
		i, err := s.Index(n.Table, n.Name)
		if err != nil {
			return 0, err
		}
		return s[i].Type, nil
	case *ast.SlotRef:
		if n.Index < 0 || n.Index >= len(s) {
			return 0, fmt.Errorf("slot %d out of range", n.Index)
		}
		return s[n.Index].Type, nil
	case *ast.UnaryExpr:
		t, err := inferExprType(n.Expr, s)
		if err != nil {
			return 0, err
		}
		switch n.Op {
		case "-":
			if !numericType(t) {
				return 0, fmt.Errorf("operator - requires a numeric operand")
			}
			return t, nil
		case "NOT":
			if t != value.TBool {
				return 0, fmt.Errorf("operator NOT requires a BOOL operand")
			}
			return value.TBool, nil
		default:
			return 0, fmt.Errorf("unsupported unary operator %q", n.Op)
		}
	case *ast.IsNull:
		if _, err := inferExprType(n.Expr, s); err != nil {
			return 0, err
		}
		return value.TBool, nil
	case *ast.BinaryExpr:
		leftType, err := inferExprType(n.Left, s)
		if err != nil {
			return 0, err
		}
		rightType, err := inferExprType(n.Right, s)
		if err != nil {
			return 0, err
		}
		switch n.Op {
		case "+", "-", "*", "/":
			if !numericType(leftType) || !numericType(rightType) {
				return 0, fmt.Errorf("operator %s requires numeric operands", n.Op)
			}
			if n.Op == "/" || leftType == value.TFloat || rightType == value.TFloat {
				return value.TFloat, nil
			}
			return value.TInt, nil
		case "=", "<>", "<", "<=", ">", ">=":
			if !comparableTypes(leftType, rightType) {
				return 0, fmt.Errorf("operator %s requires compatible operands", n.Op)
			}
			return value.TBool, nil
		case "AND", "OR":
			if leftType != value.TBool || rightType != value.TBool {
				return 0, fmt.Errorf("operator %s requires BOOL operands", n.Op)
			}
			return value.TBool, nil
		default:
			return 0, fmt.Errorf("unsupported binary operator %q", n.Op)
		}
	default:
		return 0, fmt.Errorf("unsupported expression %T", e)
	}
}

func requireBool(e ast.Expr, schema exec.Schema, clause string) error {
	t, err := inferExprType(e, schema)
	if err != nil {
		return err
	}
	if t != value.TBool {
		return fmt.Errorf("%s requires BOOL, got %v", clause, t)
	}
	return nil
}

func comparableTypes(left, right value.Type) bool {
	return numericType(left) && numericType(right) || left == right
}

func numericType(t value.Type) bool {
	return t == value.TInt || t == value.TFloat
}
