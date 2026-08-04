package plan

import (
	"fmt"

	"github.com/aybavs/sql-query-engine/internal/ast"
	"github.com/aybavs/sql-query-engine/internal/exec"
	"github.com/aybavs/sql-query-engine/internal/value"
)

func containsAggregate(e ast.Expr) bool {
	switch n := e.(type) {
	case *ast.AggregateCall:
		return true
	case *ast.UnaryExpr:
		return containsAggregate(n.Expr)
	case *ast.BinaryExpr:
		return containsAggregate(n.Left) || containsAggregate(n.Right)
	case *ast.IsNull:
		return containsAggregate(n.Expr)
	default:
		return false
	}
}

func isAggregateQuery(st *ast.SelectStmt) bool {
	if len(st.GroupBy) > 0 {
		return true
	}
	for _, projection := range st.Projections {
		if containsAggregate(projection.Expr) {
			return true
		}
	}
	if containsAggregate(st.Having) {
		return true
	}
	for _, item := range st.OrderBy {
		if containsAggregate(item.Expr) {
			return true
		}
	}
	return false
}

func buildAggregatePlan(
	st *ast.SelectStmt,
	input exec.Operator,
	inputSchema exec.Schema,
) (exec.Operator, exec.Schema, []ast.Expr, ast.Expr, []ast.OrderItem, error) {
	if containsAggregate(st.Where) {
		return nil, nil, nil, nil, nil, fmt.Errorf("WHERE cannot contain aggregates")
	}
	if st.Having != nil && !isAggregateQuery(st) {
		return nil, nil, nil, nil, nil, fmt.Errorf("HAVING requires an aggregate query")
	}
	for _, projection := range st.Projections {
		if projection.Star {
			return nil, nil, nil, nil, nil, fmt.Errorf("SELECT * is not supported in aggregate queries")
		}
	}

	groupExprs := make([]ast.Expr, 0, len(st.GroupBy))
	groupSlots := make(map[int]int, len(st.GroupBy))
	aggregateSchema := make(exec.Schema, 0, len(st.GroupBy))
	for _, expr := range st.GroupBy {
		column, ok := expr.(*ast.ColumnRef)
		if !ok {
			return nil, nil, nil, nil, nil, fmt.Errorf("GROUP BY requires columns")
		}
		inputIndex, err := inputSchema.Index(column.Table, column.Name)
		if err != nil {
			return nil, nil, nil, nil, nil, err
		}
		if _, exists := groupSlots[inputIndex]; exists {
			return nil, nil, nil, nil, nil, fmt.Errorf("duplicate GROUP BY column %s", column.Name)
		}
		groupSlots[inputIndex] = len(groupExprs)
		groupExprs = append(groupExprs, column)
		aggregateSchema = append(aggregateSchema, inputSchema[inputIndex])
	}

	collector := aggregateCollector{
		inputSchema: inputSchema,
		slots:       make(map[*ast.AggregateCall]int),
	}
	for _, projection := range st.Projections {
		if err := collector.collect(projection.Expr); err != nil {
			return nil, nil, nil, nil, nil, err
		}
	}
	if err := collector.collect(st.Having); err != nil {
		return nil, nil, nil, nil, nil, err
	}
	for _, item := range st.OrderBy {
		if err := collector.collect(item.Expr); err != nil {
			return nil, nil, nil, nil, nil, err
		}
	}

	for i, spec := range collector.specs {
		collector.slots[collector.calls[i]] = len(aggregateSchema)
		aggregateSchema = append(aggregateSchema, exec.Column{
			Name: ast.String(collector.calls[i]),
			Type: spec.OutType,
		})
	}

	projections := make([]ast.Expr, 0, len(st.Projections))
	for _, projection := range st.Projections {
		lowered, err := lowerAggregateExpr(projection.Expr, collector.slots, groupSlots, inputSchema)
		if err != nil {
			return nil, nil, nil, nil, nil, err
		}
		if _, err := inferExprType(lowered, aggregateSchema); err != nil {
			return nil, nil, nil, nil, nil, err
		}
		projections = append(projections, lowered)
	}

	var having ast.Expr
	if st.Having != nil {
		var err error
		having, err = lowerAggregateExpr(st.Having, collector.slots, groupSlots, inputSchema)
		if err != nil {
			return nil, nil, nil, nil, nil, err
		}
		if err := requireBool(having, aggregateSchema, "HAVING"); err != nil {
			return nil, nil, nil, nil, nil, err
		}
	}

	orderBy := make([]ast.OrderItem, 0, len(st.OrderBy))
	for _, item := range st.OrderBy {
		lowered, err := lowerAggregateExpr(item.Expr, collector.slots, groupSlots, inputSchema)
		if err != nil {
			return nil, nil, nil, nil, nil, err
		}
		if _, err := inferExprType(lowered, aggregateSchema); err != nil {
			return nil, nil, nil, nil, nil, err
		}
		orderBy = append(orderBy, ast.OrderItem{Expr: lowered, Desc: item.Desc})
	}

	aggregate := exec.NewAggregate(input, groupExprs, collector.specs, aggregateSchema)
	return aggregate, aggregateSchema, projections, having, orderBy, nil
}

type aggregateCollector struct {
	inputSchema exec.Schema
	calls       []*ast.AggregateCall
	specs       []exec.AggregateSpec
	slots       map[*ast.AggregateCall]int
}

func (c *aggregateCollector) collect(e ast.Expr) error {
	switch n := e.(type) {
	case nil, *ast.Literal, *ast.ColumnRef, *ast.SlotRef:
		return nil
	case *ast.AggregateCall:
		if containsAggregate(n.Arg) {
			return fmt.Errorf("nested aggregate")
		}
		spec, err := aggregateSpec(n, c.inputSchema)
		if err != nil {
			return err
		}
		c.calls = append(c.calls, n)
		c.specs = append(c.specs, spec)
		return nil
	case *ast.UnaryExpr:
		return c.collect(n.Expr)
	case *ast.BinaryExpr:
		if err := c.collect(n.Left); err != nil {
			return err
		}
		return c.collect(n.Right)
	case *ast.IsNull:
		return c.collect(n.Expr)
	default:
		return fmt.Errorf("unsupported expression %T", e)
	}
}

func aggregateSpec(call *ast.AggregateCall, inputSchema exec.Schema) (exec.AggregateSpec, error) {
	switch call.Name {
	case "COUNT":
		if call.Star {
			return exec.AggregateSpec{Kind: exec.AggCount, Star: true, OutType: value.TInt}, nil
		}
		argType, err := inferExprType(call.Arg, inputSchema)
		if err != nil {
			return exec.AggregateSpec{}, err
		}
		_ = argType
		return exec.AggregateSpec{Kind: exec.AggCount, Expr: call.Arg, OutType: value.TInt}, nil
	case "SUM":
		return numericAggregateSpec(exec.AggSum, call, inputSchema, false)
	case "AVG":
		return numericAggregateSpec(exec.AggAvg, call, inputSchema, true)
	case "MIN":
		return orderedAggregateSpec(exec.AggMin, call, inputSchema)
	case "MAX":
		return orderedAggregateSpec(exec.AggMax, call, inputSchema)
	default:
		return exec.AggregateSpec{}, fmt.Errorf("unknown aggregate %s", call.Name)
	}
}

func numericAggregateSpec(
	kind exec.AggregateKind,
	call *ast.AggregateCall,
	inputSchema exec.Schema,
	forceFloat bool,
) (exec.AggregateSpec, error) {
	if call.Star {
		return exec.AggregateSpec{}, fmt.Errorf("%s does not accept *", call.Name)
	}
	argType, err := inferExprType(call.Arg, inputSchema)
	if err != nil {
		return exec.AggregateSpec{}, err
	}
	if !numericType(argType) {
		return exec.AggregateSpec{}, fmt.Errorf("%s requires numeric argument", call.Name)
	}
	outType := argType
	if forceFloat {
		outType = value.TFloat
	}
	return exec.AggregateSpec{Kind: kind, Expr: call.Arg, OutType: outType}, nil
}

func orderedAggregateSpec(
	kind exec.AggregateKind,
	call *ast.AggregateCall,
	inputSchema exec.Schema,
) (exec.AggregateSpec, error) {
	if call.Star {
		return exec.AggregateSpec{}, fmt.Errorf("%s does not accept *", call.Name)
	}
	argType, err := inferExprType(call.Arg, inputSchema)
	if err != nil {
		return exec.AggregateSpec{}, err
	}
	switch argType {
	case value.TInt, value.TFloat, value.TText, value.TBool:
		return exec.AggregateSpec{Kind: kind, Expr: call.Arg, OutType: argType}, nil
	default:
		return exec.AggregateSpec{}, fmt.Errorf("%s requires an ordered argument", call.Name)
	}
}

func lowerAggregateExpr(
	e ast.Expr,
	aggregateSlots map[*ast.AggregateCall]int,
	groupSlots map[int]int,
	inputSchema exec.Schema,
) (ast.Expr, error) {
	switch n := e.(type) {
	case *ast.Literal:
		return n, nil
	case *ast.SlotRef:
		return n, nil
	case *ast.AggregateCall:
		slot, ok := aggregateSlots[n]
		if !ok {
			return nil, fmt.Errorf("aggregate %s has no output slot", n.Name)
		}
		return &ast.SlotRef{Index: slot}, nil
	case *ast.ColumnRef:
		inputIndex, err := inputSchema.Index(n.Table, n.Name)
		if err != nil {
			return nil, err
		}
		slot, ok := groupSlots[inputIndex]
		if !ok {
			return nil, fmt.Errorf("column %s must appear in GROUP BY", n.Name)
		}
		return &ast.SlotRef{Index: slot}, nil
	case *ast.UnaryExpr:
		expr, err := lowerAggregateExpr(n.Expr, aggregateSlots, groupSlots, inputSchema)
		if err != nil {
			return nil, err
		}
		return &ast.UnaryExpr{Op: n.Op, Expr: expr}, nil
	case *ast.BinaryExpr:
		left, err := lowerAggregateExpr(n.Left, aggregateSlots, groupSlots, inputSchema)
		if err != nil {
			return nil, err
		}
		right, err := lowerAggregateExpr(n.Right, aggregateSlots, groupSlots, inputSchema)
		if err != nil {
			return nil, err
		}
		return &ast.BinaryExpr{Op: n.Op, Left: left, Right: right}, nil
	case *ast.IsNull:
		expr, err := lowerAggregateExpr(n.Expr, aggregateSlots, groupSlots, inputSchema)
		if err != nil {
			return nil, err
		}
		return &ast.IsNull{Expr: expr, Negate: n.Negate}, nil
	default:
		return nil, fmt.Errorf("unsupported expression %T", e)
	}
}
