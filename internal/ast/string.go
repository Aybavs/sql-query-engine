package ast

import "github.com/aybavs/sql-query-engine/internal/value"

// String renders an expression back to SQL-like text. It is used to label
// result columns for computed projections, the way `SELECT COUNT(id)` reports
// a column named "COUNT(id)".
func String(e Expr) string {
	switch n := e.(type) {
	case *ColumnRef:
		if n.Table != "" {
			return n.Table + "." + n.Name
		}
		return n.Name

	case *Literal:
		if !n.Val.IsNull() && n.Val.Type == value.TText {
			return "'" + n.Val.S + "'"
		}
		return n.Val.String()

	case *UnaryExpr:
		if n.Op == "NOT" {
			return "NOT " + operand(n.Expr)
		}
		return n.Op + operand(n.Expr)

	case *IsNull:
		if n.Negate {
			return operand(n.Expr) + " IS NOT NULL"
		}
		return operand(n.Expr) + " IS NULL"

	case *BinaryExpr:
		return operand(n.Left) + " " + n.Op + " " + operand(n.Right)

	case *AggregateCall:
		if n.Star {
			return n.Name + "(*)"
		}
		return n.Name + "(" + String(n.Arg) + ")"

	case *SlotRef:
		// Slots only exist after aggregate lowering and are never user-visible;
		// labels are taken from the original expressions.
		return "expr"

	default:
		return "expr"
	}
}

// operand renders a sub-expression, parenthesizing nested binary and unary
// operators so a rendered label cannot be read with the wrong grouping.
func operand(e Expr) string {
	switch e.(type) {
	case *BinaryExpr, *UnaryExpr:
		return "(" + String(e) + ")"
	default:
		return String(e)
	}
}
