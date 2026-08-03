// Package ast defines the query syntax tree.
package ast

import "github.com/aybavs/sql-query-engine/internal/value"

type Expr interface{ isExpr() }

type ColumnRef struct{ Table, Name string }
type Literal struct{ Val value.Value }
type BinaryExpr struct {
	Op          string
	Left, Right Expr
}
type UnaryExpr struct {
	Op   string
	Expr Expr
}
type IsNull struct {
	Expr   Expr
	Negate bool
}

func (*ColumnRef) isExpr()  {}
func (*Literal) isExpr()    {}
func (*BinaryExpr) isExpr() {}
func (*UnaryExpr) isExpr()  {}
func (*IsNull) isExpr()     {}

// SelectStmt and friends are used by the statement parser.
type SelectStmt struct {
	Projections []Projection
	From        string
	Where       Expr
	OrderBy     []OrderItem
	Limit       *int
}
type Projection struct {
	Star bool
	Expr Expr
}
type OrderItem struct {
	Expr Expr
	Desc bool
}
