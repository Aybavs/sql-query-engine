package difftest

import (
	"github.com/aybavs/sql-query-engine/internal/catalog"
	"github.com/aybavs/sql-query-engine/internal/lexer"
	"github.com/aybavs/sql-query-engine/internal/parser"
	"github.com/aybavs/sql-query-engine/internal/plan"
)

// runEngine executes a query through this engine — lexer, parser, planner, then
// the operator tree — and returns rows normalized for comparison.
func runEngine(cat *catalog.Catalog, dataDir, query string) ([][]string, error) {
	toks, err := lexer.Lex(query)
	if err != nil {
		return nil, err
	}
	st, err := parser.New(toks).ParseSelect()
	if err != nil {
		return nil, err
	}
	op, _, err := plan.Build(st, cat, dataDir)
	if err != nil {
		return nil, err
	}

	var out [][]string
	for {
		row, ok := op.Next()
		if !ok {
			return out, nil
		}
		out = append(out, normalizeEngineRow(row))
	}
}
