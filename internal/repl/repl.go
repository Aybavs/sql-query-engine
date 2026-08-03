// Package repl reads SQL statements and prints result tables.
package repl

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/aybavs/sql-query-engine/internal/catalog"
	"github.com/aybavs/sql-query-engine/internal/exec"
	"github.com/aybavs/sql-query-engine/internal/lexer"
	"github.com/aybavs/sql-query-engine/internal/parser"
	"github.com/aybavs/sql-query-engine/internal/plan"
	"github.com/aybavs/sql-query-engine/internal/value"
)

// Run reads one statement per line from in and writes results (or errors) to
// out. A failing statement never stops the loop.
func Run(cat *catalog.Catalog, dataDir string, in io.Reader, out io.Writer) {
	sc := bufio.NewScanner(in)
	for sc.Scan() {
		line := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(sc.Text()), ";"))
		if line == "" {
			continue
		}
		if err := runOne(cat, dataDir, line, out); err != nil {
			fmt.Fprintf(out, "error: %v\n", err)
		}
	}
}

func runOne(cat *catalog.Catalog, dataDir, sql string, out io.Writer) error {
	toks, err := lexer.Lex(sql)
	if err != nil {
		return err
	}
	st, err := parser.New(toks).ParseSelect()
	if err != nil {
		return err
	}
	op, schema, err := plan.Build(st, cat, dataDir)
	if err != nil {
		return err
	}
	printTable(out, schema, op)
	return nil
}

// printTable renders the result set as an aligned text table.
func printTable(out io.Writer, schema exec.Schema, op exec.Operator) {
	header := make([]string, len(schema))
	widths := make([]int, len(schema))
	for i, c := range schema {
		header[i] = c.Name
		widths[i] = len(c.Name)
	}

	var rows [][]string
	for {
		row, ok := op.Next()
		if !ok {
			break
		}
		cells := make([]string, len(row))
		for i, v := range row {
			cells[i] = v.String()
			if i < len(widths) && len(cells[i]) > widths[i] {
				widths[i] = len(cells[i])
			}
		}
		rows = append(rows, cells)
	}

	fmt.Fprintln(out, joinPadded(header, widths))
	fmt.Fprintln(out, ruler(widths))
	for _, r := range rows {
		fmt.Fprintln(out, joinPadded(r, widths))
	}
	fmt.Fprintf(out, "(%d rows)\n", len(rows))
}

func joinPadded(cells []string, widths []int) string {
	parts := make([]string, len(cells))
	for i, c := range cells {
		w := 0
		if i < len(widths) {
			w = widths[i]
		}
		parts[i] = c + strings.Repeat(" ", max(0, w-len(c)))
	}
	return strings.Join(parts, " | ")
}

func ruler(widths []int) string {
	parts := make([]string, len(widths))
	for i, w := range widths {
		parts[i] = strings.Repeat("-", w)
	}
	return strings.Join(parts, "-+-")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// LoadSchema parses lines of the form `table(col TYPE, col TYPE, ...)`.
// Each table maps to `<name>.csv` in the same directory.
func LoadSchema(path string) (*catalog.Catalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cat := catalog.New()
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		open := strings.IndexByte(line, '(')
		if open < 0 || !strings.HasSuffix(line, ")") {
			return nil, fmt.Errorf("bad schema line: %q", line)
		}
		name := strings.TrimSpace(line[:open])
		body := line[open+1 : len(line)-1]
		var cols []catalog.Column
		for _, part := range strings.Split(body, ",") {
			fields := strings.Fields(strings.TrimSpace(part))
			if len(fields) != 2 {
				return nil, fmt.Errorf("bad column %q in table %q", part, name)
			}
			t, err := parseType(fields[1])
			if err != nil {
				return nil, err
			}
			cols = append(cols, catalog.Column{Name: fields[0], Type: t})
		}
		cat.Add(&catalog.Table{Name: name, File: name + ".csv", Columns: cols})
	}
	return cat, nil
}

func parseType(s string) (value.Type, error) {
	switch strings.ToUpper(s) {
	case "INT":
		return value.TInt, nil
	case "FLOAT":
		return value.TFloat, nil
	case "TEXT":
		return value.TText, nil
	case "BOOL":
		return value.TBool, nil
	default:
		return 0, fmt.Errorf("unknown type %q", s)
	}
}
