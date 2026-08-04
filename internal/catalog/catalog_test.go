package catalog

import (
	"testing"

	"github.com/aybavs/sql-query-engine/internal/value"
)

func TestCatalogLookup(t *testing.T) {
	c := New()
	c.Add(&Table{
		Name: "users",
		File: "users.csv",
		Columns: []Column{
			{Name: "id", Type: value.TInt},
			{Name: "name", Type: value.TText},
		},
	})

	tbl, ok := c.Table("users")
	if !ok {
		t.Fatal("expected users table")
	}
	if _, ok := c.Table("missing"); ok {
		t.Fatal("missing table should not resolve")
	}
	if i, ok := tbl.ColumnIndex("name"); !ok || i != 1 {
		t.Fatalf("name index = (%d,%v), want (1,true)", i, ok)
	}
	if _, ok := tbl.ColumnIndex("nope"); ok {
		t.Fatal("unknown column should not resolve")
	}
}

func TestTablesIsSortedAndComplete(t *testing.T) {
	c := New()
	c.Add(&Table{Name: "users"})
	c.Add(&Table{Name: "orders"})
	got := c.Tables()
	if len(got) != 2 || got[0].Name != "orders" || got[1].Name != "users" {
		t.Fatalf("Tables() = %v, want [orders users]", got)
	}
}
