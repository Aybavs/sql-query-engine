package plan

import (
	"strings"
	"testing"

	"github.com/aybavs/sql-query-engine/internal/ast"
	"github.com/aybavs/sql-query-engine/internal/exec"
	"github.com/aybavs/sql-query-engine/internal/lexer"
	"github.com/aybavs/sql-query-engine/internal/parser"
	"github.com/aybavs/sql-query-engine/internal/value"
)

func parsePlanSelect(t *testing.T, sql string) *ast.SelectStmt {
	t.Helper()
	tokens, err := lexer.Lex(sql)
	if err != nil {
		t.Fatal(err)
	}
	statement, err := parser.New(tokens).ParseSelect()
	if err != nil {
		t.Fatal(err)
	}
	return statement
}

func TestIsAggregateQuery(t *testing.T) {
	for _, sql := range []string{
		"SELECT COUNT(*) FROM users",
		"SELECT city FROM users GROUP BY city",
		"SELECT city FROM users ORDER BY MAX(age)",
	} {
		if !isAggregateQuery(parsePlanSelect(t, sql)) {
			t.Fatalf("not classified as aggregate: %s", sql)
		}
	}
	if isAggregateQuery(parsePlanSelect(t, "SELECT age FROM users ORDER BY age")) {
		t.Fatal("ordinary query classified as aggregate")
	}
}

func TestBuildAggregatePlanLowersSlots(t *testing.T) {
	st := parsePlanSelect(t, "SELECT city, COUNT(*) + 1 FROM users GROUP BY city HAVING COUNT(*) > 1 ORDER BY AVG(age) DESC")
	inSchema := exec.Schema{{Table: "users", Name: "city", Type: value.TText}, {Table: "users", Name: "age", Type: value.TInt}}
	op, aggSchema, projections, having, orderBy, err := buildAggregatePlan(st, exec.NewScan(inSchema, nil), inSchema)
	if err != nil {
		t.Fatal(err)
	}
	if op == nil || len(aggSchema) != 4 {
		t.Fatalf("schema = %#v", aggSchema)
	}
	if _, ok := projections[0].(*ast.SlotRef); !ok {
		t.Fatalf("group projection = %T", projections[0])
	}
	if containsAggregate(projections[1]) || containsAggregate(having) || containsAggregate(orderBy[0].Expr) {
		t.Fatal("post-aggregate expression was not fully lowered")
	}
	wantTypes := []value.Type{value.TText, value.TInt, value.TInt, value.TFloat}
	for i, want := range wantTypes {
		if aggSchema[i].Type != want {
			t.Fatalf("schema[%d].Type = %v, want %v", i, aggSchema[i].Type, want)
		}
	}
	assertInternalAggregateExpr(t, projections[0])
	assertInternalAggregateExpr(t, projections[1])
	assertInternalAggregateExpr(t, having)
	assertInternalAggregateExpr(t, orderBy[0].Expr)
	if projections[0].(*ast.SlotRef).Index != 0 || slotInBinary(t, projections[1], true) != 1 ||
		slotInBinary(t, having, true) != 2 || orderBy[0].Expr.(*ast.SlotRef).Index != 3 {
		t.Fatalf("aggregate slots were not assigned in GROUP, SELECT, HAVING, ORDER BY order")
	}
}

func TestBuildAggregatePlanUsesResolvedGroupIdentity(t *testing.T) {
	st := parsePlanSelect(t, "SELECT users.age, COUNT(*) FROM users GROUP BY age")
	s := exec.Schema{{Table: "users", Name: "age", Type: value.TInt}}
	_, _, projections, _, _, err := buildAggregatePlan(st, exec.NewScan(s, nil), s)
	if err != nil {
		t.Fatal(err)
	}
	if slot := projections[0].(*ast.SlotRef); slot.Index != 0 {
		t.Fatalf("group slot = %d, want 0", slot.Index)
	}

	st = parsePlanSelect(t, "SELECT COUNT(*) FROM users GROUP BY age, users.age")
	_, _, _, _, _, err = buildAggregatePlan(st, exec.NewScan(s, nil), s)
	if err == nil || !strings.Contains(err.Error(), "duplicate GROUP BY") {
		t.Fatalf("duplicate error = %v", err)
	}
}

func TestBuildAggregatePlanPreservesAggregateOutputTypes(t *testing.T) {
	st := parsePlanSelect(t, "SELECT COUNT(label), SUM(n), SUM(ratio), AVG(n), MIN(label), MAX(active) FROM users")
	s := exec.Schema{
		{Table: "users", Name: "n", Type: value.TInt},
		{Table: "users", Name: "ratio", Type: value.TFloat},
		{Table: "users", Name: "label", Type: value.TText},
		{Table: "users", Name: "active", Type: value.TBool},
	}
	_, aggregateSchema, _, _, _, err := buildAggregatePlan(st, exec.NewScan(s, nil), s)
	if err != nil {
		t.Fatal(err)
	}
	want := []value.Type{value.TInt, value.TInt, value.TFloat, value.TFloat, value.TText, value.TBool}
	for i, wantType := range want {
		if aggregateSchema[i].Type != wantType {
			t.Fatalf("schema[%d].Type = %v, want %v", i, aggregateSchema[i].Type, wantType)
		}
	}
}

func TestBuildAggregatePlanRejectsInvalidSQL(t *testing.T) {
	tests := []struct{ sql, want string }{
		{"SELECT name, COUNT(*) FROM users", "must appear in GROUP BY"},
		{"SELECT * FROM users GROUP BY age", "SELECT *"},
		{"SELECT SUM(name) FROM users", "SUM requires numeric"},
		{"SELECT MIDDLE(age) FROM users", "unknown aggregate"},
		{"SELECT SUM(COUNT(*)) FROM users", "nested aggregate"},
		{"SELECT SUM(COUNT(missing)) FROM users", "nested aggregate"},
		{"SELECT age FROM users GROUP BY age + 1", "GROUP BY requires columns"},
		{"SELECT COUNT(*) FROM users GROUP BY COUNT(*)", "GROUP BY requires columns"},
		{"SELECT age FROM users WHERE COUNT(*) > 0", "WHERE cannot contain aggregates"},
		{"SELECT age FROM users HAVING age > 0", "HAVING requires an aggregate query"},
	}
	for _, tt := range tests {
		t.Run(tt.sql, func(t *testing.T) {
			st := parsePlanSelect(t, tt.sql)
			s := exec.Schema{{Table: "users", Name: "age", Type: value.TInt}, {Table: "users", Name: "name", Type: value.TText}}
			_, _, _, _, _, err := buildAggregatePlan(st, exec.NewScan(s, nil), s)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func assertInternalAggregateExpr(t *testing.T, expr ast.Expr) {
	t.Helper()
	switch n := expr.(type) {
	case *ast.Literal, *ast.SlotRef:
		return
	case *ast.UnaryExpr:
		assertInternalAggregateExpr(t, n.Expr)
	case *ast.BinaryExpr:
		assertInternalAggregateExpr(t, n.Left)
		assertInternalAggregateExpr(t, n.Right)
	case *ast.IsNull:
		assertInternalAggregateExpr(t, n.Expr)
	default:
		t.Fatalf("post-aggregate expression contains %T", expr)
	}
}

func slotInBinary(t *testing.T, expr ast.Expr, left bool) int {
	t.Helper()
	binary, ok := expr.(*ast.BinaryExpr)
	if !ok {
		t.Fatalf("expression = %T, want *ast.BinaryExpr", expr)
	}
	child := binary.Right
	if left {
		child = binary.Left
	}
	slot, ok := child.(*ast.SlotRef)
	if !ok {
		t.Fatalf("binary child = %T, want *ast.SlotRef", child)
	}
	return slot.Index
}
