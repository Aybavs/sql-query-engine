package plan

import (
	"os"
	"path/filepath"
	"testing"
)

// nullDir holds rows where age and city are NULL in different combinations, so
// each test can isolate one aspect of three-valued logic.
func nullDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "users.csv"), []byte(
		"1,alice,30,berlin\n"+
			"2,bob,,berlin\n"+ // NULL age
			"3,carol,40,\n"+ // NULL city
			"4,dan,,\n", // both NULL
	), 0o644)
	os.WriteFile(filepath.Join(dir, "orders.csv"), []byte("10,1,100\n11,,50\n"), 0o644)
	return dir
}

func runNull(t *testing.T, sql, dir string) [][]string {
	t.Helper()
	return buildAndRun(t, sql, dir, labelCatalog())
}

func TestWhereExcludesUnknownComparisons(t *testing.T) {
	dir := nullDir(t)
	// bob and dan have a NULL age, so `age > 10` is unknown for them and WHERE
	// keeps only rows that are exactly true.
	got := runNull(t, "SELECT name FROM users WHERE age > 10 ORDER BY name", dir)
	if len(got) != 2 || got[0][0] != "alice" || got[1][0] != "carol" {
		t.Fatalf("rows = %v, want [alice carol]", got)
	}
}

func TestWhereNotOfUnknownIsStillExcluded(t *testing.T) {
	dir := nullDir(t)
	// NOT unknown is unknown, so a NULL age is excluded by the negation too.
	got := runNull(t, "SELECT name FROM users WHERE NOT (age > 10)", dir)
	for _, r := range got {
		if r[0] == "bob" || r[0] == "dan" {
			t.Fatalf("NULL-age row %q must not pass NOT(unknown); rows = %v", r[0], got)
		}
	}
}

func TestIsNullFindsNullRows(t *testing.T) {
	dir := nullDir(t)
	got := runNull(t, "SELECT name FROM users WHERE age IS NULL ORDER BY name", dir)
	if len(got) != 2 || got[0][0] != "bob" || got[1][0] != "dan" {
		t.Fatalf("rows = %v, want [bob dan]", got)
	}
}

func TestIsNotNullExcludesNullRows(t *testing.T) {
	dir := nullDir(t)
	got := runNull(t, "SELECT name FROM users WHERE age IS NOT NULL ORDER BY name", dir)
	if len(got) != 2 || got[0][0] != "alice" || got[1][0] != "carol" {
		t.Fatalf("rows = %v, want [alice carol]", got)
	}
}

func TestNullEqualsNullIsNotTrue(t *testing.T) {
	dir := nullDir(t)
	// `age = age` is unknown, not true, when age is NULL.
	got := runNull(t, "SELECT name FROM users WHERE age = age", dir)
	for _, r := range got {
		if r[0] == "bob" || r[0] == "dan" {
			t.Fatalf("NULL = NULL must be unknown, but %q passed; rows = %v", r[0], got)
		}
	}
}

func TestArithmeticWithNullIsNull(t *testing.T) {
	dir := nullDir(t)
	got := runNull(t, "SELECT name, age + 1 FROM users WHERE name = 'bob'", dir)
	if len(got) != 1 || got[0][1] != "NULL" {
		t.Fatalf("rows = %v, want bob with a NULL computed age", got)
	}
}

func TestJoinDropsNullKeys(t *testing.T) {
	dir := nullDir(t)
	// Order 11 has a NULL user_id and can never match, since NULL = NULL is
	// unknown rather than true.
	got := runNull(t, "SELECT orders.id FROM users JOIN orders ON users.id = orders.user_id", dir)
	if len(got) != 1 || got[0][0] != "10" {
		t.Fatalf("rows = %v, want only order 10", got)
	}
}

func TestAggregatesIgnoreNulls(t *testing.T) {
	dir := nullDir(t)
	// COUNT(age) skips NULLs; COUNT(*) counts rows.
	got := runNull(t, "SELECT COUNT(age), COUNT(*) FROM users", dir)
	if len(got) != 1 || got[0][0] != "2" || got[0][1] != "4" {
		t.Fatalf("rows = %v, want COUNT(age)=2 and COUNT(*)=4", got)
	}
}

func TestAggregateOverAllNullValuesIsNull(t *testing.T) {
	dir := nullDir(t)
	// dan is the only row in a group whose age is NULL, so its AVG is NULL.
	got := runNull(t, "SELECT AVG(age) FROM users WHERE name = 'dan'", dir)
	if len(got) != 1 || got[0][0] != "NULL" {
		t.Fatalf("rows = %v, want a NULL average", got)
	}
}

// GROUP BY is the one place SQL treats NULL as a value: all NULL keys collapse
// into a single group, even though NULL = NULL is not true.
func TestNullsGroupTogether(t *testing.T) {
	dir := nullDir(t)
	got := runNull(t, "SELECT city, COUNT(*) FROM users GROUP BY city", dir)
	if len(got) != 2 {
		t.Fatalf("rows = %v, want two groups (berlin and the NULL city)", got)
	}
	var nullGroup []string
	for _, r := range got {
		if r[0] == "NULL" {
			nullGroup = r
		}
	}
	if nullGroup == nil || nullGroup[1] != "2" {
		t.Fatalf("rows = %v, want the NULL city group to hold carol and dan", got)
	}
}
