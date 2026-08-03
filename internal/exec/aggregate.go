package exec

import (
	"github.com/aybavs/sql-query-engine/internal/ast"
	"github.com/aybavs/sql-query-engine/internal/value"
)

type AggregateKind int

const (
	AggCount AggregateKind = iota
	AggSum
	AggAvg
	AggMin
	AggMax
)

type AggregateSpec struct {
	Kind    AggregateKind
	Expr    ast.Expr
	Star    bool
	OutType value.Type
}

type aggregateState struct {
	count    int64
	intSum   int64
	floatSum float64
	seen     bool
	best     value.Value
}

type aggregateGroup struct {
	values value.Row
	states []aggregateState
}

// Aggregate is a blocking operator that groups all child rows before emitting
// finalized aggregate values.
type Aggregate struct {
	child       Operator
	groupExprs  []ast.Expr
	specs       []AggregateSpec
	schema      Schema
	groups      []*aggregateGroup
	initialized bool
	failed      bool
	index       int
}

func NewAggregate(child Operator, groupExprs []ast.Expr, specs []AggregateSpec, schema Schema) *Aggregate {
	return &Aggregate{
		child:      child,
		groupExprs: groupExprs,
		specs:      specs,
		schema:     schema,
	}
}

func (a *Aggregate) Schema() Schema { return a.schema }

func (a *Aggregate) build() {
	a.initialized = true
	byKey := make(map[string]*aggregateGroup)
	if len(a.groupExprs) == 0 {
		group := a.newGroup(nil)
		a.groups = append(a.groups, group)
	}

	for {
		row, ok := a.child.Next()
		if !ok {
			return
		}

		var group *aggregateGroup
		if len(a.groupExprs) == 0 {
			group = a.groups[0]
		} else {
			values := make(value.Row, len(a.groupExprs))
			for i, expr := range a.groupExprs {
				v, err := Eval(expr, row, a.child.Schema())
				if err != nil {
					a.failed = true
					return
				}
				values[i] = v
			}
			key := groupKey(values)
			group = byKey[key]
			if group == nil {
				group = a.newGroup(values)
				byKey[key] = group
				a.groups = append(a.groups, group)
			}
		}

		if err := a.step(group, row); err != nil {
			a.failed = true
			return
		}
	}
}

func (a *Aggregate) newGroup(values value.Row) *aggregateGroup {
	return &aggregateGroup{
		values: values,
		states: make([]aggregateState, len(a.specs)),
	}
}

func (a *Aggregate) step(group *aggregateGroup, row value.Row) error {
	for i, spec := range a.specs {
		state := &group.states[i]
		if spec.Kind == AggCount && spec.Star {
			state.count++
			continue
		}
		v, err := Eval(spec.Expr, row, a.child.Schema())
		if err != nil {
			return err
		}
		if v.IsNull() {
			continue
		}
		if spec.Kind == AggCount {
			state.count++
			continue
		}
		if v.IsNaN() {
			continue
		}

		switch spec.Kind {
		case AggSum:
			state.seen = true
			if v.Type == value.TInt {
				state.intSum += v.I
			} else {
				state.floatSum += v.F
			}
		case AggAvg:
			state.count++
			if v.Type == value.TInt {
				state.floatSum += float64(v.I)
			} else {
				state.floatSum += v.F
			}
		case AggMin:
			if !state.seen {
				state.seen = true
				state.best = v
				continue
			}
			if ord, known := value.Compare(v, state.best); known && ord < 0 {
				state.best = v
			}
		case AggMax:
			if !state.seen {
				state.seen = true
				state.best = v
				continue
			}
			if ord, known := value.Compare(v, state.best); known && ord > 0 {
				state.best = v
			}
		}
	}
	return nil
}

func (a *Aggregate) Next() (value.Row, bool) {
	if !a.initialized {
		a.build()
	}
	if a.failed || a.index >= len(a.groups) {
		return nil, false
	}

	group := a.groups[a.index]
	a.index++
	row := make(value.Row, 0, len(group.values)+len(a.specs))
	row = append(row, group.values...)
	for i, spec := range a.specs {
		row = append(row, finalizeAggregate(spec, group.states[i]))
	}
	return row, true
}

func finalizeAggregate(spec AggregateSpec, state aggregateState) value.Value {
	switch spec.Kind {
	case AggCount:
		return value.Int64(state.count)
	case AggSum:
		if !state.seen {
			return value.NullOf(spec.OutType)
		}
		if spec.OutType == value.TInt {
			return value.Int64(state.intSum)
		}
		return value.Float64(state.floatSum)
	case AggAvg:
		if state.count == 0 {
			return value.NullOf(value.TFloat)
		}
		return value.Float64(state.floatSum / float64(state.count))
	case AggMin, AggMax:
		if !state.seen {
			return value.NullOf(spec.OutType)
		}
		return state.best
	}
	panic("unknown aggregate kind")
}
