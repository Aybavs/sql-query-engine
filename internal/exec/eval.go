package exec

import (
	"fmt"

	"github.com/aybavs/sql-query-engine/internal/ast"
	"github.com/aybavs/sql-query-engine/internal/value"
)

// Eval evaluates an expression against a row using the given schema.
func Eval(e ast.Expr, row value.Row, s Schema) (value.Value, error) {
	switch n := e.(type) {
	case *ast.Literal:
		return n.Val, nil
	case *ast.ColumnRef:
		i, err := s.Index(n.Table, n.Name)
		if err != nil {
			return value.Value{}, err
		}
		return row[i], nil
	case *ast.IsNull:
		v, err := Eval(n.Expr, row, s)
		if err != nil {
			return value.Value{}, err
		}
		isNull := v.IsNull()
		if n.Negate {
			return value.Bool(!isNull), nil
		}
		return value.Bool(isNull), nil
	case *ast.UnaryExpr:
		v, err := Eval(n.Expr, row, s)
		if err != nil {
			return value.Value{}, err
		}
		switch n.Op {
		case "NOT":
			return value.Not(v), nil
		case "-":
			if v.IsNull() {
				return v, nil
			}
			if v.Type == value.TInt {
				return value.Int64(-v.I), nil
			}
			return value.Float64(-v.F), nil
		}
		return value.Value{}, fmt.Errorf("unknown unary op %q", n.Op)
	case *ast.BinaryExpr:
		return evalBinary(n, row, s)
	default:
		return value.Value{}, fmt.Errorf("cannot evaluate %T", e)
	}
}

func evalBinary(n *ast.BinaryExpr, row value.Row, s Schema) (value.Value, error) {
	l, err := Eval(n.Left, row, s)
	if err != nil {
		return value.Value{}, err
	}
	r, err := Eval(n.Right, row, s)
	if err != nil {
		return value.Value{}, err
	}
	switch n.Op {
	case "AND":
		return value.And(l, r), nil
	case "OR":
		return value.Or(l, r), nil
	case "=", "<>", "<", "<=", ">", ">=":
		ord, known := value.Compare(l, r)
		if !known {
			return value.NullOf(value.TBool), nil
		}
		return value.Bool(compareResult(n.Op, ord)), nil
	case "+", "-", "*", "/":
		return arithmetic(n.Op, l, r)
	default:
		return value.Value{}, fmt.Errorf("unknown operator %q", n.Op)
	}
}

func compareResult(op string, ord int) bool {
	switch op {
	case "=":
		return ord == 0
	case "<>":
		return ord != 0
	case "<":
		return ord < 0
	case "<=":
		return ord <= 0
	case ">":
		return ord > 0
	case ">=":
		return ord >= 0
	}
	return false
}

func arithmetic(op string, l, r value.Value) (value.Value, error) {
	if l.IsNull() || r.IsNull() {
		return value.NullOf(value.TFloat), nil
	}
	lf, rf := toF(l), toF(r)
	var out float64
	switch op {
	case "+":
		out = lf + rf
	case "-":
		out = lf - rf
	case "*":
		out = lf * rf
	case "/":
		if rf == 0 {
			return value.NullOf(value.TFloat), nil
		}
		out = lf / rf
	}
	if l.Type == value.TInt && r.Type == value.TInt && op != "/" {
		return value.Int64(int64(out)), nil
	}
	return value.Float64(out), nil
}

func toF(v value.Value) float64 {
	if v.Type == value.TInt {
		return float64(v.I)
	}
	return v.F
}
