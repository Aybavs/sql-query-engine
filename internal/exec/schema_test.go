package exec

import (
	"testing"

	"github.com/aybavs/sql-query-engine/internal/value"
)

func joined() Schema {
	return Schema{
		{Table: "users", Name: "id", Type: value.TInt},
		{Table: "users", Name: "name", Type: value.TText},
		{Table: "orders", Name: "id", Type: value.TInt},
		{Table: "orders", Name: "user_id", Type: value.TInt},
	}
}

func TestIndexQualified(t *testing.T) {
	s := joined()
	if i, err := s.Index("orders", "id"); err != nil || i != 2 {
		t.Fatalf("orders.id = (%d,%v), want (2,nil)", i, err)
	}
	if i, err := s.Index("users", "id"); err != nil || i != 0 {
		t.Fatalf("users.id = (%d,%v), want (0,nil)", i, err)
	}
}

func TestIndexAmbiguousBareName(t *testing.T) {
	if _, err := joined().Index("", "id"); err == nil {
		t.Fatal("bare `id` is ambiguous across both tables and must error")
	}
}

func TestIndexUnambiguousBareName(t *testing.T) {
	if i, err := joined().Index("", "user_id"); err != nil || i != 3 {
		t.Fatalf("user_id = (%d,%v), want (3,nil)", i, err)
	}
}

func TestIndexUnknownQualifier(t *testing.T) {
	if _, err := joined().Index("nope", "id"); err == nil {
		t.Fatal("unknown table qualifier must error")
	}
}
