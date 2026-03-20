package astql

import (
	"fmt"

	"github.com/zoobz-io/astql/internal/types"
)

// and creates an AND condition group (internal helper for builder).
func and(conditions ...types.ConditionItem) types.ConditionGroup {
	return types.ConditionGroup{
		Logic:      types.AND,
		Conditions: conditions,
	}
}

// c creates a simple condition (internal helper for builder).
func c(f types.Field, op types.Operator, p types.Param) types.Condition {
	return types.Condition{
		Field:    f,
		Operator: op,
		Value:    p,
	}
}

// Builder provides a fluent API for constructing queries.
type Builder struct {
	ast *types.AST
	err error
}

// GetAST returns the internal AST.
func (b *Builder) GetAST() *types.AST {
	return b.ast
}

// GetError returns the internal error (for use by provider packages).
func (b *Builder) GetError() error {
	return b.err
}

// SetError sets the internal error (for use by provider packages).
func (b *Builder) SetError(err error) {
	b.err = err
}

// Select creates a new SELECT query builder.
func Select(t types.Table) *Builder {
	return &Builder{
		ast: &types.AST{
			Operation: types.OpSelect,
			Target:    t,
		},
	}
}

// Insert creates a new INSERT query builder.
func Insert(t types.Table) *Builder {
	return &Builder{
		ast: &types.AST{
			Operation: types.OpInsert,
			Target:    t,
		},
	}
}

// Update creates a new UPDATE query builder.
func Update(t types.Table) *Builder {
	return &Builder{
		ast: &types.AST{
			Operation: types.OpUpdate,
			Target:    t,
			Updates:   make(map[types.Field]types.Param),
		},
	}
}

// Delete creates a new DELETE query builder.
func Delete(t types.Table) *Builder {
	return &Builder{
		ast: &types.AST{
			Operation: types.OpDelete,
			Target:    t,
		},
	}
}

// Count creates a new COUNT query builder.
func Count(t types.Table) *Builder {
	return &Builder{
		ast: &types.AST{
			Operation: types.OpCount,
			Target:    t,
		},
	}
}

// Fields sets the fields to select.
func (b *Builder) Fields(fields ...types.Field) *Builder {
	if b.err != nil {
		return b
	}
	if b.ast.Operation != types.OpSelect {
		b.err = fmt.Errorf("Fields() can only be used with SELECT queries")
		return b
	}
	b.ast.Fields = fields
	return b
}

// Where sets or adds conditions.
func (b *Builder) Where(condition types.ConditionItem) *Builder {
	if b.err != nil {
		return b
	}

	if b.ast.WhereClause == nil {
		b.ast.WhereClause = condition
	} else {
		// If there's already a where clause, combine with AND
		b.ast.WhereClause = and(b.ast.WhereClause, condition)
	}

	return b
}

// WhereField is a convenience method for simple field conditions.
func (b *Builder) WhereField(f types.Field, op types.Operator, p types.Param) *Builder {
	return b.Where(c(f, op, p))
}

// Set adds a field update for UPDATE queries.
func (b *Builder) Set(f types.Field, p types.Param) *Builder {
	if b.err != nil {
		return b
	}
	if b.ast.Operation != types.OpUpdate {
		b.err = fmt.Errorf("Set() can only be used with UPDATE queries")
		return b
	}
	if b.ast.Updates == nil {
		b.ast.Updates = make(map[types.Field]types.Param)
	}
	b.ast.Updates[f] = p
	return b
}

// SetExpr adds a field update with an expression value for UPDATE queries.
// Use this for computed assignments like `items_completed = items_completed + :increment`.
func (b *Builder) SetExpr(f types.Field, expr types.FieldExpression) *Builder {
	if b.err != nil {
		return b
	}
	if b.ast.Operation != types.OpUpdate {
		b.err = fmt.Errorf("SetExpr() can only be used with UPDATE queries")
		return b
	}
	if expr.Alias != "" {
		b.err = fmt.Errorf("SetExpr() does not support aliased expressions")
		return b
	}
	if b.ast.UpdateExpressions == nil {
		b.ast.UpdateExpressions = make(map[types.Field]types.FieldExpression)
	}
	b.ast.UpdateExpressions[f] = expr
	return b
}

// Values adds a row of field-value pairs for INSERT queries.
// Call Values() multiple times to insert multiple rows.
// Use instance.ValueMap() to create the map programmatically.
func (b *Builder) Values(valueMap map[types.Field]types.Param) *Builder {
	if b.err != nil {
		return b
	}
	if b.ast.Operation != types.OpInsert {
		b.err = fmt.Errorf("Values() can only be used with INSERT queries")
		return b
	}
	if len(valueMap) == 0 {
		b.err = fmt.Errorf("Values() requires at least one field-value pair")
		return b
	}
	if b.ast.Values == nil {
		b.ast.Values = []map[types.Field]types.Param{}
	}
	b.ast.Values = append(b.ast.Values, valueMap)
	return b
}

// OrderBy adds ordering.
func (b *Builder) OrderBy(f types.Field, direction types.Direction) *Builder {
	if b.err != nil {
		return b
	}
	if b.ast.Ordering == nil {
		b.ast.Ordering = []types.OrderBy{}
	}
	b.ast.Ordering = append(b.ast.Ordering, types.OrderBy{
		Field:     f,
		Direction: direction,
	})
	return b
}

// OrderByNulls adds ordering with NULLS FIRST/LAST.
func (b *Builder) OrderByNulls(f types.Field, direction types.Direction, nulls types.NullsOrdering) *Builder {
	if b.err != nil {
		return b
	}
	if b.ast.Ordering == nil {
		b.ast.Ordering = []types.OrderBy{}
	}
	b.ast.Ordering = append(b.ast.Ordering, types.OrderBy{
		Field:     f,
		Direction: direction,
		Nulls:     nulls,
	})
	return b
}

// OrderByExpr is useful for vector distance ordering: ORDER BY embedding <-> :query_vector ASC.
func (b *Builder) OrderByExpr(f types.Field, op types.Operator, p types.Param, direction types.Direction) *Builder {
	if b.err != nil {
		return b
	}
	if b.ast.Ordering == nil {
		b.ast.Ordering = []types.OrderBy{}
	}
	b.ast.Ordering = append(b.ast.Ordering, types.OrderBy{
		Field:     f,
		Operator:  op,
		Param:     p,
		Direction: direction,
	})
	return b
}

// Limit sets the limit to a static integer value.
func (b *Builder) Limit(limit int) *Builder {
	if b.err != nil {
		return b
	}
	b.ast.Limit = &types.PaginationValue{Static: &limit}
	return b
}

// LimitParam sets the limit to a parameterized value.
func (b *Builder) LimitParam(param types.Param) *Builder {
	if b.err != nil {
		return b
	}
	b.ast.Limit = &types.PaginationValue{Param: &param}
	return b
}

// Offset sets the offset to a static integer value.
func (b *Builder) Offset(offset int) *Builder {
	if b.err != nil {
		return b
	}
	b.ast.Offset = &types.PaginationValue{Static: &offset}
	return b
}

// OffsetParam sets the offset to a parameterized value.
func (b *Builder) OffsetParam(param types.Param) *Builder {
	if b.err != nil {
		return b
	}
	b.ast.Offset = &types.PaginationValue{Param: &param}
	return b
}

// Build returns the constructed AST or an error.
func (b *Builder) Build() (*types.AST, error) {
	if b.err != nil {
		return nil, b.err
	}

	// Validate the AST
	if err := b.ast.Validate(); err != nil {
		return nil, err
	}

	return b.ast, nil
}

// MustBuild returns the AST or panics on error.
func (b *Builder) MustBuild() *types.AST {
	ast, err := b.Build()
	if err != nil {
		panic(err)
	}
	return ast
}

// Render builds the AST and renders it using the provided renderer.
func (b *Builder) Render(renderer Renderer) (*QueryResult, error) {
	ast, err := b.Build()
	if err != nil {
		return nil, err
	}
	return renderer.Render(ast)
}

// MustRender builds and renders the AST with the provided renderer, or panics on error.
func (b *Builder) MustRender(renderer Renderer) *QueryResult {
	result, err := b.Render(renderer)
	if err != nil {
		panic(err)
	}
	return result
}

// Distinct sets the DISTINCT flag for SELECT queries.
func (b *Builder) Distinct() *Builder {
	if b.err != nil {
		return b
	}
	if b.ast.Operation != types.OpSelect {
		b.err = fmt.Errorf("DISTINCT can only be used with SELECT queries")
		return b
	}
	if len(b.ast.DistinctOn) > 0 {
		b.err = fmt.Errorf("cannot use DISTINCT with DISTINCT ON")
		return b
	}
	b.ast.Distinct = true
	return b
}

// DistinctOn sets DISTINCT ON fields for SELECT queries (PostgreSQL).
// The query will return only the first row for each unique combination of the specified fields.
func (b *Builder) DistinctOn(fields ...types.Field) *Builder {
	if b.err != nil {
		return b
	}
	if b.ast.Operation != types.OpSelect {
		b.err = fmt.Errorf("DISTINCT ON can only be used with SELECT queries")
		return b
	}
	if b.ast.Distinct {
		b.err = fmt.Errorf("cannot use DISTINCT ON with DISTINCT")
		return b
	}
	b.ast.DistinctOn = fields
	return b
}

// ForUpdate adds FOR UPDATE row locking.
func (b *Builder) ForUpdate() *Builder {
	if b.err != nil {
		return b
	}
	if b.ast.Operation != types.OpSelect {
		b.err = fmt.Errorf("FOR UPDATE can only be used with SELECT queries")
		return b
	}
	lock := types.LockForUpdate
	b.ast.Lock = &lock
	return b
}

// ForNoKeyUpdate adds FOR NO KEY UPDATE row locking.
func (b *Builder) ForNoKeyUpdate() *Builder {
	if b.err != nil {
		return b
	}
	if b.ast.Operation != types.OpSelect {
		b.err = fmt.Errorf("FOR NO KEY UPDATE can only be used with SELECT queries")
		return b
	}
	lock := types.LockForNoKeyUpdate
	b.ast.Lock = &lock
	return b
}

// ForShare adds FOR SHARE row locking.
func (b *Builder) ForShare() *Builder {
	if b.err != nil {
		return b
	}
	if b.ast.Operation != types.OpSelect {
		b.err = fmt.Errorf("FOR SHARE can only be used with SELECT queries")
		return b
	}
	lock := types.LockForShare
	b.ast.Lock = &lock
	return b
}

// ForKeyShare adds FOR KEY SHARE row locking.
func (b *Builder) ForKeyShare() *Builder {
	if b.err != nil {
		return b
	}
	if b.ast.Operation != types.OpSelect {
		b.err = fmt.Errorf("FOR KEY SHARE can only be used with SELECT queries")
		return b
	}
	lock := types.LockForKeyShare
	b.ast.Lock = &lock
	return b
}

// Join adds an INNER JOIN.
func (b *Builder) Join(table types.Table, on types.ConditionItem) *Builder {
	return b.addJoin(types.InnerJoin, table, on)
}

// InnerJoin adds an INNER JOIN.
func (b *Builder) InnerJoin(table types.Table, on types.ConditionItem) *Builder {
	return b.addJoin(types.InnerJoin, table, on)
}

// LeftJoin adds a LEFT JOIN.
func (b *Builder) LeftJoin(table types.Table, on types.ConditionItem) *Builder {
	return b.addJoin(types.LeftJoin, table, on)
}

// RightJoin adds a RIGHT JOIN.
func (b *Builder) RightJoin(table types.Table, on types.ConditionItem) *Builder {
	return b.addJoin(types.RightJoin, table, on)
}

// FullOuterJoin adds a FULL OUTER JOIN.
func (b *Builder) FullOuterJoin(table types.Table, on types.ConditionItem) *Builder {
	return b.addJoin(types.FullOuterJoin, table, on)
}

// CrossJoin adds a CROSS JOIN (no ON clause needed).
func (b *Builder) CrossJoin(table types.Table) *Builder {
	return b.addJoin(types.CrossJoin, table, nil)
}

// addJoin is a helper to add joins.
func (b *Builder) addJoin(joinType types.JoinType, table types.Table, on types.ConditionItem) *Builder {
	if b.err != nil {
		return b
	}
	if b.ast.Operation != types.OpSelect && b.ast.Operation != types.OpCount {
		b.err = fmt.Errorf("JOIN can only be used with SELECT or COUNT queries")
		return b
	}
	if joinType == types.CrossJoin && on != nil {
		b.err = fmt.Errorf("CROSS JOIN cannot have ON clause")
		return b
	}
	if joinType != types.CrossJoin && on == nil {
		b.err = fmt.Errorf("%s requires ON clause", joinType)
		return b
	}

	join := types.Join{
		Type:  joinType,
		Table: table,
		On:    on,
	}

	b.ast.Joins = append(b.ast.Joins, join)
	return b
}

// GroupBy adds GROUP BY fields.
func (b *Builder) GroupBy(fields ...types.Field) *Builder {
	if b.err != nil {
		return b
	}
	if b.ast.Operation != types.OpSelect {
		b.err = fmt.Errorf("GROUP BY can only be used with SELECT queries")
		return b
	}
	b.ast.GroupBy = append(b.ast.GroupBy, fields...)
	return b
}

// Having adds HAVING conditions (simple field-based conditions).
// For aggregate conditions like COUNT(*) > 10, use HavingAgg instead.
func (b *Builder) Having(conditions ...types.Condition) *Builder {
	if b.err != nil {
		return b
	}
	if b.ast.Operation != types.OpSelect {
		b.err = fmt.Errorf("HAVING can only be used with SELECT queries")
		return b
	}
	if len(b.ast.GroupBy) == 0 {
		b.err = fmt.Errorf("HAVING requires GROUP BY")
		return b
	}
	for _, c := range conditions {
		b.ast.Having = append(b.ast.Having, c)
	}
	return b
}

// HavingAgg adds aggregate HAVING conditions like COUNT(*) > :min_count.
// Use this for conditions on aggregate functions (COUNT, SUM, AVG, MIN, MAX).
func (b *Builder) HavingAgg(conditions ...types.AggregateCondition) *Builder {
	if b.err != nil {
		return b
	}
	if b.ast.Operation != types.OpSelect {
		b.err = fmt.Errorf("HAVING can only be used with SELECT queries")
		return b
	}
	if len(b.ast.GroupBy) == 0 {
		b.err = fmt.Errorf("HAVING requires GROUP BY")
		return b
	}
	for _, c := range conditions {
		b.ast.Having = append(b.ast.Having, c)
	}
	return b
}

// Returning adds RETURNING fields for INSERT/UPDATE/DELETE.
func (b *Builder) Returning(fields ...types.Field) *Builder {
	if b.err != nil {
		return b
	}
	switch b.ast.Operation {
	case types.OpInsert, types.OpUpdate, types.OpDelete:
		b.ast.Returning = append(b.ast.Returning, fields...)
	default:
		b.err = fmt.Errorf("RETURNING can only be used with INSERT, UPDATE, or DELETE")
	}
	return b
}

// OnConflict adds ON CONFLICT clause for INSERT.
func (b *Builder) OnConflict(columns ...types.Field) *ConflictBuilder {
	if b.err != nil {
		return &ConflictBuilder{builder: b, err: b.err}
	}
	if b.ast.Operation != types.OpInsert {
		err := fmt.Errorf("ON CONFLICT can only be used with INSERT")
		b.err = err
		return &ConflictBuilder{builder: b, err: err}
	}

	b.ast.OnConflict = &types.ConflictClause{
		Columns: columns,
	}

	return &ConflictBuilder{builder: b}
}

// ConflictBuilder handles ON CONFLICT actions.
type ConflictBuilder struct {
	builder *Builder
	err     error
}

// DoNothing sets the conflict action to DO NOTHING.
func (cb *ConflictBuilder) DoNothing() *Builder {
	if cb.err != nil {
		return cb.builder
	}
	cb.builder.ast.OnConflict.Action = types.DoNothing
	return cb.builder
}

// DoUpdate sets the conflict action to DO UPDATE.
func (cb *ConflictBuilder) DoUpdate() *UpdateBuilder {
	if cb.err != nil {
		return &UpdateBuilder{builder: cb.builder, err: cb.err}
	}
	cb.builder.ast.OnConflict.Action = types.DoUpdate
	cb.builder.ast.OnConflict.Updates = make(map[types.Field]types.Param)
	return &UpdateBuilder{
		builder: cb.builder,
		updates: cb.builder.ast.OnConflict.Updates,
	}
}

// UpdateBuilder handles DO UPDATE SET clause construction.
type UpdateBuilder struct {
	builder *Builder
	updates map[types.Field]types.Param
	err     error
}

// Set adds a field to update on conflict.
func (ub *UpdateBuilder) Set(field types.Field, param types.Param) *UpdateBuilder {
	if ub.err != nil {
		return ub
	}
	ub.updates[field] = param
	return ub
}

// Build finalizes the update and returns the builder.
func (ub *UpdateBuilder) Build() *Builder {
	return ub.builder
}

// SelectExpr adds a field expression (aggregate, case, etc) to SELECT.
func (b *Builder) SelectExpr(expr types.FieldExpression) *Builder {
	if b.err != nil {
		return b
	}
	if b.ast.Operation != types.OpSelect {
		b.err = fmt.Errorf("SelectExpr can only be used with SELECT queries")
		return b
	}
	b.ast.FieldExpressions = append(b.ast.FieldExpressions, expr)
	return b
}

// SelectBinaryExpr adds a binary expression (field <op> param) AS alias to SELECT.
// Useful for vector distance calculations with pgvector.
//
// Example:
//
//	builder.SelectBinaryExpr(f, VectorCosineDistance, p, "score")
//	// Renders: "field" <=> :param AS "score"
func (b *Builder) SelectBinaryExpr(f types.Field, op types.Operator, p types.Param, alias string) *Builder {
	if b.err != nil {
		return b
	}
	if b.ast.Operation != types.OpSelect {
		b.err = fmt.Errorf("SelectBinaryExpr can only be used with SELECT queries")
		return b
	}
	if !isValidSQLIdentifier(alias) {
		b.err = fmt.Errorf("invalid alias '%s': must be alphanumeric/underscore, start with letter/underscore, and contain no SQL keywords", alias)
		return b
	}
	b.ast.FieldExpressions = append(b.ast.FieldExpressions, types.FieldExpression{
		Binary: &types.BinaryExpression{
			Field:    f,
			Operator: op,
			Param:    p,
		},
		Alias: alias,
	})
	return b
}

// Set operations (UNION, INTERSECT, EXCEPT)

// Union creates a UNION between two queries (standalone function).
func Union(first, second *Builder) *CompoundBuilder {
	return first.Union(second)
}

// UnionAll creates a UNION ALL between two queries (standalone function).
func UnionAll(first, second *Builder) *CompoundBuilder {
	return first.UnionAll(second)
}

// Intersect creates an INTERSECT between two queries (standalone function).
func Intersect(first, second *Builder) *CompoundBuilder {
	return first.Intersect(second)
}

// IntersectAll creates an INTERSECT ALL between two queries (standalone function).
func IntersectAll(first, second *Builder) *CompoundBuilder {
	return first.IntersectAll(second)
}

// Except creates an EXCEPT between two queries (standalone function).
func Except(first, second *Builder) *CompoundBuilder {
	return first.Except(second)
}

// ExceptAll creates an EXCEPT ALL between two queries (standalone function).
func ExceptAll(first, second *Builder) *CompoundBuilder {
	return first.ExceptAll(second)
}

// CompoundBuilder handles building compound queries with set operations.
type CompoundBuilder struct {
	query *types.CompoundQuery
	err   error
}

// Union creates a UNION between two queries.
func (b *Builder) Union(other *Builder) *CompoundBuilder {
	return b.setOperation(types.SetUnion, other)
}

// UnionAll creates a UNION ALL between two queries.
func (b *Builder) UnionAll(other *Builder) *CompoundBuilder {
	return b.setOperation(types.SetUnionAll, other)
}

// Intersect creates an INTERSECT between two queries.
func (b *Builder) Intersect(other *Builder) *CompoundBuilder {
	return b.setOperation(types.SetIntersect, other)
}

// IntersectAll creates an INTERSECT ALL between two queries.
func (b *Builder) IntersectAll(other *Builder) *CompoundBuilder {
	return b.setOperation(types.SetIntersectAll, other)
}

// Except creates an EXCEPT between two queries.
func (b *Builder) Except(other *Builder) *CompoundBuilder {
	return b.setOperation(types.SetExcept, other)
}

// ExceptAll creates an EXCEPT ALL between two queries.
func (b *Builder) ExceptAll(other *Builder) *CompoundBuilder {
	return b.setOperation(types.SetExceptAll, other)
}

func (b *Builder) setOperation(op types.SetOperation, other *Builder) *CompoundBuilder {
	if b.err != nil {
		return &CompoundBuilder{err: b.err}
	}
	if other.err != nil {
		return &CompoundBuilder{err: other.err}
	}
	if b.ast.Operation != types.OpSelect {
		return &CompoundBuilder{err: fmt.Errorf("set operations can only be used with SELECT queries")}
	}
	if other.ast.Operation != types.OpSelect {
		return &CompoundBuilder{err: fmt.Errorf("set operations can only be used with SELECT queries")}
	}

	baseAST, err := b.Build()
	if err != nil {
		return &CompoundBuilder{err: err}
	}

	otherAST, err := other.Build()
	if err != nil {
		return &CompoundBuilder{err: err}
	}

	return &CompoundBuilder{
		query: &types.CompoundQuery{
			Base: baseAST,
			Operands: []types.SetOperand{{
				AST:       otherAST,
				Operation: op,
			}},
		},
	}
}

// Union adds a UNION to the compound query.
func (cb *CompoundBuilder) Union(other *Builder) *CompoundBuilder {
	return cb.addOperation(types.SetUnion, other)
}

// UnionAll adds a UNION ALL to the compound query.
func (cb *CompoundBuilder) UnionAll(other *Builder) *CompoundBuilder {
	return cb.addOperation(types.SetUnionAll, other)
}

// Intersect adds an INTERSECT to the compound query.
func (cb *CompoundBuilder) Intersect(other *Builder) *CompoundBuilder {
	return cb.addOperation(types.SetIntersect, other)
}

// IntersectAll adds an INTERSECT ALL to the compound query.
func (cb *CompoundBuilder) IntersectAll(other *Builder) *CompoundBuilder {
	return cb.addOperation(types.SetIntersectAll, other)
}

// Except adds an EXCEPT to the compound query.
func (cb *CompoundBuilder) Except(other *Builder) *CompoundBuilder {
	return cb.addOperation(types.SetExcept, other)
}

// ExceptAll adds an EXCEPT ALL to the compound query.
func (cb *CompoundBuilder) ExceptAll(other *Builder) *CompoundBuilder {
	return cb.addOperation(types.SetExceptAll, other)
}

func (cb *CompoundBuilder) addOperation(op types.SetOperation, other *Builder) *CompoundBuilder {
	if cb.err != nil {
		return cb
	}
	if len(cb.query.Operands) >= types.MaxSetOperations {
		cb.err = fmt.Errorf("too many set operations: max %d", types.MaxSetOperations)
		return cb
	}
	if other.err != nil {
		cb.err = other.err
		return cb
	}
	if other.ast.Operation != types.OpSelect {
		cb.err = fmt.Errorf("set operations can only be used with SELECT queries")
		return cb
	}

	otherAST, err := other.Build()
	if err != nil {
		cb.err = err
		return cb
	}

	cb.query.Operands = append(cb.query.Operands, types.SetOperand{
		AST:       otherAST,
		Operation: op,
	})
	return cb
}

// OrderBy adds final ordering to the compound query.
func (cb *CompoundBuilder) OrderBy(f types.Field, direction types.Direction) *CompoundBuilder {
	if cb.err != nil {
		return cb
	}
	cb.query.Ordering = append(cb.query.Ordering, types.OrderBy{
		Field:     f,
		Direction: direction,
	})
	return cb
}

// OrderByNulls adds final ordering with NULLS FIRST/LAST to the compound query.
func (cb *CompoundBuilder) OrderByNulls(f types.Field, direction types.Direction, nulls types.NullsOrdering) *CompoundBuilder {
	if cb.err != nil {
		return cb
	}
	cb.query.Ordering = append(cb.query.Ordering, types.OrderBy{
		Field:     f,
		Direction: direction,
		Nulls:     nulls,
	})
	return cb
}

// Limit sets the limit for the compound query to a static integer value.
func (cb *CompoundBuilder) Limit(limit int) *CompoundBuilder {
	if cb.err != nil {
		return cb
	}
	cb.query.Limit = &types.PaginationValue{Static: &limit}
	return cb
}

// LimitParam sets the limit for the compound query to a parameterized value.
func (cb *CompoundBuilder) LimitParam(param types.Param) *CompoundBuilder {
	if cb.err != nil {
		return cb
	}
	cb.query.Limit = &types.PaginationValue{Param: &param}
	return cb
}

// Offset sets the offset for the compound query to a static integer value.
func (cb *CompoundBuilder) Offset(offset int) *CompoundBuilder {
	if cb.err != nil {
		return cb
	}
	cb.query.Offset = &types.PaginationValue{Static: &offset}
	return cb
}

// OffsetParam sets the offset for the compound query to a parameterized value.
func (cb *CompoundBuilder) OffsetParam(param types.Param) *CompoundBuilder {
	if cb.err != nil {
		return cb
	}
	cb.query.Offset = &types.PaginationValue{Param: &param}
	return cb
}

// Build returns the CompoundQuery or an error.
func (cb *CompoundBuilder) Build() (*types.CompoundQuery, error) {
	if cb.err != nil {
		return nil, cb.err
	}
	return cb.query, nil
}

// MustBuild returns the CompoundQuery or panics on error.
func (cb *CompoundBuilder) MustBuild() *types.CompoundQuery {
	query, err := cb.Build()
	if err != nil {
		panic(err)
	}
	return query
}

// Render builds and renders the compound query using the provided renderer.
func (cb *CompoundBuilder) Render(renderer Renderer) (*QueryResult, error) {
	query, err := cb.Build()
	if err != nil {
		return nil, err
	}
	return renderer.RenderCompound(query)
}

// MustRender builds and renders the compound query with the provided renderer, or panics on error.
func (cb *CompoundBuilder) MustRender(renderer Renderer) *QueryResult {
	result, err := cb.Render(renderer)
	if err != nil {
		panic(err)
	}
	return result
}
