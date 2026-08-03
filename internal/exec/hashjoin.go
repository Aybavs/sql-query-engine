package exec

import (
	"github.com/aybavs/sql-query-engine/internal/ast"
	"github.com/aybavs/sql-query-engine/internal/value"
)

// HashJoin is an inner equi-join. It materializes the right (build) input into
// a hash table keyed by the join key, then streams the left (probe) input,
// emitting one concatenated row per match.
//
// A nested-loop join costs O(n*m); hashing the build side makes this O(n+m)
// with a single pass over each input, at the cost of holding the build side in
// memory.
type HashJoin struct {
	left, right       Operator
	leftKey, rightKey ast.Expr
	schema            Schema

	built bool
	table map[string][]value.Row

	// current probe row and its remaining matches
	probe   value.Row
	matches []value.Row
	at      int
}

func NewHashJoin(left, right Operator, leftKey, rightKey ast.Expr) *HashJoin {
	schema := make(Schema, 0, len(left.Schema())+len(right.Schema()))
	schema = append(schema, left.Schema()...)
	schema = append(schema, right.Schema()...)
	return &HashJoin{
		left: left, right: right,
		leftKey: leftKey, rightKey: rightKey,
		schema: schema,
	}
}

func (h *HashJoin) Schema() Schema { return h.schema }

// build drains the right input into the hash table. Rows whose key is NULL or
// NaN are dropped because neither can match.
func (h *HashJoin) build() {
	h.table = make(map[string][]value.Row)
	rs := h.right.Schema()
	for {
		row, ok := h.right.Next()
		if !ok {
			break
		}
		v, err := Eval(h.rightKey, row, rs)
		if err != nil {
			continue
		}
		k, ok := encodeKey(v)
		if !ok {
			continue
		}
		h.table[k] = append(h.table[k], row)
	}
	h.built = true
}

func (h *HashJoin) Next() (value.Row, bool) {
	if !h.built {
		h.build()
	}
	for {
		// Emit any pending matches for the current probe row first.
		if h.at < len(h.matches) {
			out := make(value.Row, 0, len(h.probe)+len(h.matches[h.at]))
			out = append(out, h.probe...)
			out = append(out, h.matches[h.at]...)
			h.at++
			return out, true
		}

		row, ok := h.left.Next()
		if !ok {
			return nil, false
		}
		v, err := Eval(h.leftKey, row, h.left.Schema())
		if err != nil {
			continue
		}
		k, ok := encodeKey(v)
		if !ok {
			continue // NULL and NaN probe keys never match
		}
		h.probe = row
		h.matches = h.table[k]
		h.at = 0
	}
}
