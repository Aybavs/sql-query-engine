package parser

import (
	"testing"

	"github.com/aybavs/sql-query-engine/internal/ast"
	"github.com/aybavs/sql-query-engine/internal/lexer"
)

func TestParseAggregateQuery(t *testing.T) {
	st := parseSelect(t, "SELECT city, COUNT(*), avg(age) FROM users WHERE age > 0 GROUP BY city HAVING COUNT(*) >= 2 ORDER BY AVG(age) DESC LIMIT 3")
	if len(st.GroupBy) != 1 || st.Having == nil || len(st.OrderBy) != 1 {
		t.Fatalf("statement = %#v", st)
	}
	count, ok := st.Projections[1].Expr.(*ast.AggregateCall)
	if !ok || count.Name != "COUNT" || !count.Star || count.Arg != nil {
		t.Fatalf("count = %#v", st.Projections[1].Expr)
	}
	avg, ok := st.Projections[2].Expr.(*ast.AggregateCall)
	if !ok || avg.Name != "AVG" || avg.Star || avg.Arg == nil {
		t.Fatalf("avg = %#v", st.Projections[2].Expr)
	}
}

func TestParseMultiColumnGroupBy(t *testing.T) {
	st := parseSelect(t, "SELECT city, age, COUNT(*) FROM users GROUP BY city, age")
	if len(st.GroupBy) != 2 {
		t.Fatalf("GroupBy length = %d, want 2", len(st.GroupBy))
	}
}

func TestParseRejectsMalformedAggregateCalls(t *testing.T) {
	for _, sql := range []string{
		"SELECT COUNT() FROM users",
		"SELECT SUM(*) FROM users",
		"SELECT AVG(age, id) FROM users",
		"SELECT COUNT(age FROM users",
	} {
		t.Run(sql, func(t *testing.T) {
			toks, err := lexer.Lex(sql)
			if err == nil {
				_, err = New(toks).ParseSelect()
			}
			if err == nil {
				t.Fatal("parse succeeded; want an error")
			}
		})
	}
}
