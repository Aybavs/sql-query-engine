package exec

import (
	"sort"

	"github.com/aybavs/sql-query-engine/internal/ast"
	"github.com/aybavs/sql-query-engine/internal/value"
)

type SortKey struct {
	Expr ast.Expr
	Desc bool
}

// Sort is a blocking operator: it drains its child on the first Next, then
// stable-sorts the materialized rows. NULLs sort before non-NULLs.
type Sort struct {
	child  Operator
	keys   []SortKey
	rows   []value.Row
	i      int
	loaded bool
}

func NewSort(child Operator, keys []SortKey) *Sort {
	return &Sort{child: child, keys: keys}
}

func (s *Sort) Schema() Schema { return s.child.Schema() }

func (s *Sort) load() {
	sch := s.child.Schema()
	for {
		r, ok := s.child.Next()
		if !ok {
			break
		}
		s.rows = append(s.rows, r)
	}
	sort.SliceStable(s.rows, func(a, b int) bool {
		for _, k := range s.keys {
			av, _ := Eval(k.Expr, s.rows[a], sch)
			bv, _ := Eval(k.Expr, s.rows[b], sch)
			c := nullAwareCompare(av, bv)
			if c == 0 {
				continue
			}
			if k.Desc {
				return c > 0
			}
			return c < 0
		}
		return false
	})
	s.loaded = true
}

// nullAwareCompare orders values with NULLs first.
func nullAwareCompare(a, b value.Value) int {
	switch {
	case a.IsNull() && b.IsNull():
		return 0
	case a.IsNull():
		return -1
	case b.IsNull():
		return 1
	}
	ord, _ := value.Compare(a, b)
	return ord
}

func (s *Sort) Next() (value.Row, bool) {
	if !s.loaded {
		s.load()
	}
	if s.i >= len(s.rows) {
		return nil, false
	}
	r := s.rows[s.i]
	s.i++
	return r, true
}
