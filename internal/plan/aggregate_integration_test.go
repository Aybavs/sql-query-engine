package plan

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/aybavs/sql-query-engine/internal/ast"
	"github.com/aybavs/sql-query-engine/internal/catalog"
	"github.com/aybavs/sql-query-engine/internal/exec"
	"github.com/aybavs/sql-query-engine/internal/lexer"
	"github.com/aybavs/sql-query-engine/internal/parser"
	"github.com/aybavs/sql-query-engine/internal/value"
)

func aggregateFixture(t *testing.T) (string, *catalog.Catalog) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "users.csv"), []byte("1,alice,30,ankara\n2,bob,15,izmir\n3,carol,40,ankara\n4,dave,,bursa\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "orders.csv"), []byte("1,1,10.5\n2,1,20\n3,2,7\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cat := catalog.New()
	cat.Add(&catalog.Table{Name: "users", File: "users.csv", Columns: []catalog.Column{{Name: "id", Type: value.TInt}, {Name: "name", Type: value.TText}, {Name: "age", Type: value.TInt}, {Name: "city", Type: value.TText}}})
	cat.Add(&catalog.Table{Name: "orders", File: "orders.csv", Columns: []catalog.Column{{Name: "id", Type: value.TInt}, {Name: "user_id", Type: value.TInt}, {Name: "total", Type: value.TFloat}}})
	return dir, cat
}

func TestAggregateSlotRefEvaluationRejectsOutOfRangeIndexes(t *testing.T) {
	row := value.Row{value.Int64(7)}
	for _, index := range []int{-1, len(row)} {
		_, err := exec.Eval(&ast.SlotRef{Index: index}, row, nil)
		want := fmt.Sprintf("slot %d out of range", index)
		if err == nil || err.Error() != want {
			t.Fatalf("Eval slot %d error = %v, want %q", index, err, want)
		}
	}
}

func TestAggregateQueriesEndToEnd(t *testing.T) {
	dir, cat := aggregateFixture(t)
	tests := []struct {
		name, sql string
		want      [][]string
	}{
		{"global", "SELECT COUNT(*), COUNT(age), SUM(age), AVG(age), MIN(age), MAX(age) FROM users", [][]string{{"4", "3", "85", "28.333333333333332", "15", "40"}}},
		{"aggregate argument expression", "SELECT SUM(age + 1) FROM users", [][]string{{"88"}}},
		{"group having order limit", "SELECT city, COUNT(*), AVG(age) FROM users GROUP BY city HAVING COUNT(*) >= 2 ORDER BY AVG(age) DESC LIMIT 1", [][]string{{"ankara", "2", "35"}}},
		{"multi column group and result expression", "SELECT city, age, COUNT(*) + 1 FROM users GROUP BY city, age ORDER BY city, age", [][]string{{"ankara", "30", "2"}, {"ankara", "40", "2"}, {"bursa", "NULL", "2"}, {"izmir", "15", "2"}}},
		{"hidden having aggregate", "SELECT city FROM users GROUP BY city HAVING COUNT(*) >= 2 ORDER BY city", [][]string{{"ankara"}}},
		{"hidden order aggregate", "SELECT city, SUM(age) FROM users GROUP BY city ORDER BY COUNT(*) DESC, city", [][]string{{"ankara", "70"}, {"bursa", "NULL"}, {"izmir", "15"}}},
		{"join aggregate", "SELECT users.name, SUM(orders.total) FROM users JOIN orders ON users.id = orders.user_id GROUP BY users.name ORDER BY SUM(orders.total) DESC", [][]string{{"alice", "30.5"}, {"bob", "7"}}},
		{"grouped empty", "SELECT city, COUNT(*) FROM users WHERE age > 100 GROUP BY city", nil},
		{"global empty", "SELECT COUNT(*), SUM(age), AVG(age), MIN(age), MAX(age) FROM users WHERE age > 100", [][]string{{"0", "NULL", "NULL", "NULL", "NULL"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildAndRun(t, tt.sql, dir, cat)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("result = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAggregateQueriesRejectInvalidSQL(t *testing.T) {
	dir, cat := aggregateFixture(t)
	tests := []struct{ sql, want string }{
		{"SELECT age FROM users WHERE COUNT(*) > 0", "WHERE cannot contain aggregates"},
		{"SELECT COUNT(*) FROM users HAVING COUNT(*)", "HAVING requires BOOL"},
		{"SELECT name, COUNT(*) FROM users", "must appear in GROUP BY"},
		{"SELECT SUM(*) FROM users", "only COUNT accepts"},
		{"SELECT SUM(COUNT(*)) FROM users", "nested aggregate"},
		{"SELECT MIDDLE(age) FROM users", "unknown aggregate"},
		{"SELECT age FROM users HAVING age > 0", "HAVING requires an aggregate query"},
	}
	for _, tt := range tests {
		t.Run(tt.sql, func(t *testing.T) {
			tokens, err := lexer.Lex(tt.sql)
			if err == nil {
				var statement *ast.SelectStmt
				statement, err = parser.New(tokens).ParseSelect()
				if err == nil {
					_, _, err = Build(statement, cat, dir)
				}
			}
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}
