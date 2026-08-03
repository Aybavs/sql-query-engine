// Package lexer turns SQL text into a token stream.
package lexer

import (
	"fmt"
	"strings"
)

type Kind int

const (
	Ident Kind = iota
	Int
	Float
	String
	Keyword
	Op
	LParen
	RParen
	Comma
	Dot
	Star
	EOF
)

type Token struct {
	Kind Kind
	Text string
	Pos  int
}

var keywords = map[string]bool{
	"SELECT": true, "FROM": true, "WHERE": true, "ORDER": true, "BY": true,
	"LIMIT": true, "AND": true, "OR": true, "NOT": true, "ASC": true, "DESC": true,
	"IS": true, "NULL": true, "TRUE": true, "FALSE": true,
	"JOIN": true, "INNER": true, "ON": true, "GROUP": true, "HAVING": true,
}

func Lex(input string) ([]Token, error) {
	var toks []Token
	i := 0
	for i < len(input) {
		c := input[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		case c == '(':
			toks = append(toks, Token{LParen, "(", i})
			i++
		case c == ')':
			toks = append(toks, Token{RParen, ")", i})
			i++
		case c == ',':
			toks = append(toks, Token{Comma, ",", i})
			i++
		case c == '.':
			toks = append(toks, Token{Dot, ".", i})
			i++
		case c == '*':
			toks = append(toks, Token{Star, "*", i})
			i++
		case c == '\'':
			start := i
			i++
			var sb strings.Builder
			for i < len(input) && input[i] != '\'' {
				sb.WriteByte(input[i])
				i++
			}
			if i >= len(input) {
				return nil, fmt.Errorf("unterminated string at %d", start)
			}
			i++ // closing quote
			toks = append(toks, Token{String, sb.String(), start})
		case isDigit(c):
			start := i
			isFloat := false
			for i < len(input) && (isDigit(input[i]) || input[i] == '.') {
				if input[i] == '.' {
					isFloat = true
				}
				i++
			}
			k := Int
			if isFloat {
				k = Float
			}
			toks = append(toks, Token{k, input[start:i], start})
		case isLetter(c):
			start := i
			for i < len(input) && (isLetter(input[i]) || isDigit(input[i]) || input[i] == '_') {
				i++
			}
			word := input[start:i]
			up := strings.ToUpper(word)
			if keywords[up] {
				toks = append(toks, Token{Keyword, up, start})
			} else {
				toks = append(toks, Token{Ident, word, start})
			}
		case strings.ContainsRune("=<>+-/", rune(c)):
			start := i
			if c == '<' && i+1 < len(input) && (input[i+1] == '>' || input[i+1] == '=') {
				toks = append(toks, Token{Op, input[i : i+2], start})
				i += 2
			} else if c == '>' && i+1 < len(input) && input[i+1] == '=' {
				toks = append(toks, Token{Op, ">=", start})
				i += 2
			} else {
				toks = append(toks, Token{Op, string(c), start})
				i++
			}
		default:
			return nil, fmt.Errorf("unexpected character %q at %d", c, i)
		}
	}
	toks = append(toks, Token{EOF, "", len(input)})
	return toks, nil
}

func isDigit(c byte) bool  { return c >= '0' && c <= '9' }
func isLetter(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_' }
