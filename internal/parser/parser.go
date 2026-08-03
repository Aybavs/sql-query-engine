// Package parser turns tokens into an AST.
package parser

import (
	"fmt"
	"strconv"

	"github.com/aybavs/sql-query-engine/internal/ast"
	"github.com/aybavs/sql-query-engine/internal/lexer"
	"github.com/aybavs/sql-query-engine/internal/value"
)

type Parser struct {
	toks []lexer.Token
	pos  int
}

func New(toks []lexer.Token) *Parser { return &Parser{toks: toks} }

func (p *Parser) peek() lexer.Token { return p.toks[p.pos] }
func (p *Parser) next() lexer.Token { t := p.toks[p.pos]; p.pos++; return t }
func (p *Parser) atEOF() bool       { return p.peek().Kind == lexer.EOF }

func (p *Parser) expectKeyword(kw string) error {
	t := p.peek()
	if t.Kind != lexer.Keyword || t.Text != kw {
		return fmt.Errorf("expected %s at pos %d, got %q", kw, t.Pos, t.Text)
	}
	p.next()
	return nil
}

// binaryPrec is the precedence of operators (higher binds tighter).
var binaryPrec = map[string]int{
	"OR": 1, "AND": 2,
	"=": 3, "<>": 3, "<": 3, "<=": 3, ">": 3, ">=": 3,
	"+": 4, "-": 4, "*": 5, "/": 5,
}

// ParseExpr parses an expression using precedence climbing.
func (p *Parser) ParseExpr() (ast.Expr, error) { return p.parseBinary(1) }

func (p *Parser) parseBinary(minPrec int) (ast.Expr, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for {
		op, prec, ok := p.peekBinaryOp()
		if !ok || prec < minPrec {
			return left, nil
		}
		p.next()
		right, err := p.parseBinary(prec + 1)
		if err != nil {
			return nil, err
		}
		left = &ast.BinaryExpr{Op: op, Left: left, Right: right}
	}
}

func (p *Parser) peekBinaryOp() (string, int, bool) {
	t := p.peek()
	if t.Kind == lexer.Op || t.Kind == lexer.Star {
		if prec, ok := binaryPrec[t.Text]; ok {
			return t.Text, prec, true
		}
	}
	if t.Kind == lexer.Keyword && (t.Text == "AND" || t.Text == "OR") {
		return t.Text, binaryPrec[t.Text], true
	}
	return "", 0, false
}

func (p *Parser) parseUnary() (ast.Expr, error) {
	t := p.peek()
	if t.Kind == lexer.Keyword && t.Text == "NOT" {
		p.next()
		e, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &ast.UnaryExpr{Op: "NOT", Expr: e}, nil
	}
	if t.Kind == lexer.Op && t.Text == "-" {
		p.next()
		e, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &ast.UnaryExpr{Op: "-", Expr: e}, nil
	}
	return p.parsePostfix()
}

// parsePostfix handles the "IS [NOT] NULL" suffix on a primary.
func (p *Parser) parsePostfix() (ast.Expr, error) {
	e, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	if p.peek().Kind == lexer.Keyword && p.peek().Text == "IS" {
		p.next()
		negate := false
		if p.peek().Kind == lexer.Keyword && p.peek().Text == "NOT" {
			negate = true
			p.next()
		}
		if err := p.expectKeyword("NULL"); err != nil {
			return nil, err
		}
		return &ast.IsNull{Expr: e, Negate: negate}, nil
	}
	return e, nil
}

func (p *Parser) parsePrimary() (ast.Expr, error) {
	t := p.next()
	switch t.Kind {
	case lexer.LParen:
		e, err := p.ParseExpr()
		if err != nil {
			return nil, err
		}
		if p.peek().Kind != lexer.RParen {
			return nil, fmt.Errorf("expected ) at pos %d", p.peek().Pos)
		}
		p.next()
		return e, nil
	case lexer.Int:
		n, _ := strconv.ParseInt(t.Text, 10, 64)
		return &ast.Literal{Val: value.Int64(n)}, nil
	case lexer.Float:
		f, _ := strconv.ParseFloat(t.Text, 64)
		return &ast.Literal{Val: value.Float64(f)}, nil
	case lexer.String:
		return &ast.Literal{Val: value.Text(t.Text)}, nil
	case lexer.Keyword:
		switch t.Text {
		case "TRUE":
			return &ast.Literal{Val: value.Bool(true)}, nil
		case "FALSE":
			return &ast.Literal{Val: value.Bool(false)}, nil
		case "NULL":
			return &ast.Literal{Val: value.NullOf(value.TInt)}, nil
		}
		return nil, fmt.Errorf("unexpected keyword %q at pos %d", t.Text, t.Pos)
	case lexer.Ident:
		if p.peek().Kind == lexer.Dot {
			p.next()
			col := p.next()
			if col.Kind != lexer.Ident {
				return nil, fmt.Errorf("expected column name after '.' at pos %d", col.Pos)
			}
			return &ast.ColumnRef{Table: t.Text, Name: col.Text}, nil
		}
		return &ast.ColumnRef{Name: t.Text}, nil
	default:
		return nil, fmt.Errorf("unexpected token %q at pos %d", t.Text, t.Pos)
	}
}
