// Package mssql provides the SQL Server dialect renderer for astql.
package mssql

import (
	"fmt"
	"sort"
	"strings"

	"github.com/zoobz-io/astql/internal/render"
	"github.com/zoobz-io/astql/internal/types"
)

// countStarSQL is the SQL for COUNT(*) aggregate.
const countStarSQL = "COUNT(*)"

// renderContext tracks rendering state for parameter namespacing and depth limiting.
type renderContext struct {
	usedParams    map[string]bool
	paramCallback func(types.Param) string
	paramPrefix   string
	depth         int
}

// newRenderContext creates a new render context.
func newRenderContext(paramCallback func(types.Param) string) *renderContext {
	return &renderContext{
		depth:         0,
		paramPrefix:   "",
		usedParams:    make(map[string]bool),
		paramCallback: paramCallback,
	}
}

// withSubquery creates a child context for rendering a subquery.
func (ctx *renderContext) withSubquery() (*renderContext, error) {
	if ctx.depth >= types.MaxSubqueryDepth {
		return nil, fmt.Errorf("maximum subquery depth (%d) exceeded", types.MaxSubqueryDepth)
	}

	return &renderContext{
		depth:         ctx.depth + 1,
		paramPrefix:   fmt.Sprintf("sq%d_", ctx.depth+1),
		usedParams:    ctx.usedParams,
		paramCallback: ctx.paramCallback,
	}, nil
}

// addParam adds a parameter with proper namespacing.
func (ctx *renderContext) addParam(param types.Param) string {
	if ctx.paramPrefix != "" {
		param = types.Param{Name: ctx.paramPrefix + param.Name}
	}
	return ctx.paramCallback(param)
}

// Renderer implements the SQL Server dialect renderer.
type Renderer struct{}

// New creates a new SQL Server renderer.
func New() *Renderer {
	return &Renderer{}
}

// Render converts an AST to a QueryResult with SQL Server SQL.
func (r *Renderer) Render(ast *types.AST) (*types.QueryResult, error) {
	// Validate unsupported features
	if err := r.validateAST(ast); err != nil {
		return nil, err
	}

	if err := ast.Validate(); err != nil {
		return nil, fmt.Errorf("invalid AST: %w", err)
	}

	var sql strings.Builder
	var params []string
	usedParams := make(map[string]bool)

	addParam := func(param types.Param) string {
		placeholder := ":" + param.Name
		if !usedParams[param.Name] {
			params = append(params, param.Name)
			usedParams[param.Name] = true
		}
		return placeholder
	}

	ctx := newRenderContext(addParam)

	switch ast.Operation {
	case types.OpSelect:
		if err := r.renderSelect(ast, &sql, ctx); err != nil {
			return nil, err
		}
	case types.OpInsert:
		if err := r.renderInsert(ast, &sql, addParam); err != nil {
			return nil, err
		}
	case types.OpUpdate:
		if err := r.renderUpdate(ast, &sql, addParam); err != nil {
			return nil, err
		}
	case types.OpDelete:
		if err := r.renderDelete(ast, &sql, addParam); err != nil {
			return nil, err
		}
	case types.OpCount:
		if err := r.renderCount(ast, &sql, addParam); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported operation: %s", ast.Operation)
	}

	return &types.QueryResult{
		SQL:            sql.String(),
		RequiredParams: params,
	}, nil
}

// RenderCompound converts a CompoundQuery to a QueryResult with SQL and parameters.
func (r *Renderer) RenderCompound(query *types.CompoundQuery) (*types.QueryResult, error) {
	// Validate each AST in the compound query
	if err := r.validateAST(query.Base); err != nil {
		return nil, err
	}
	for _, operand := range query.Operands {
		if err := r.validateAST(operand.AST); err != nil {
			return nil, err
		}
	}

	var sql strings.Builder
	var params []string
	usedParams := make(map[string]bool)

	queryIndex := 0

	makeParamCallback := func(prefix string) func(types.Param) string {
		return func(param types.Param) string {
			name := prefix + param.Name
			placeholder := ":" + name
			if !usedParams[name] {
				params = append(params, name)
				usedParams[name] = true
			}
			return placeholder
		}
	}

	// SQL Server supports parentheses around SELECT in compound queries
	sql.WriteString("(")
	baseCtx := newRenderContext(makeParamCallback(fmt.Sprintf("q%d_", queryIndex)))
	if err := r.renderSelect(query.Base, &sql, baseCtx); err != nil {
		return nil, err
	}
	sql.WriteString(")")
	queryIndex++

	for _, operand := range query.Operands {
		sql.WriteString(" ")
		sql.WriteString(string(operand.Operation))
		sql.WriteString(" (")

		opCtx := newRenderContext(makeParamCallback(fmt.Sprintf("q%d_", queryIndex)))
		if err := r.renderSelect(operand.AST, &sql, opCtx); err != nil {
			return nil, err
		}
		sql.WriteString(")")
		queryIndex++
	}

	// Create context for final ORDER BY/OFFSET/FETCH params
	finalCtx := newRenderContext(makeParamCallback(""))

	if len(query.Ordering) > 0 {
		sql.WriteString(" ORDER BY ")
		var orderParts []string
		for i := range query.Ordering {
			order := &query.Ordering[i]
			var part string
			if order.Operator != "" {
				part = fmt.Sprintf("%s %s %s %s",
					r.renderField(order.Field),
					r.renderOperator(order.Operator),
					finalCtx.addParam(order.Param),
					order.Direction)
			} else {
				part = fmt.Sprintf("%s %s", r.renderField(order.Field), order.Direction)
			}
			orderParts = append(orderParts, part)
		}
		sql.WriteString(strings.Join(orderParts, ", "))
	}

	// SQL Server uses OFFSET/FETCH instead of LIMIT/OFFSET
	if query.Offset != nil || query.Limit != nil {
		// OFFSET/FETCH requires ORDER BY
		if len(query.Ordering) == 0 {
			return nil, render.NewUnsupportedFeatureError("mssql", "LIMIT/OFFSET without ORDER BY",
				"add ORDER BY clause when using LIMIT or OFFSET")
		}

		sql.WriteString(" OFFSET ")
		if query.Offset != nil {
			sql.WriteString(r.renderPaginationValue(query.Offset, finalCtx))
		} else {
			sql.WriteString("0")
		}
		sql.WriteString(" ROWS")

		if query.Limit != nil {
			sql.WriteString(" FETCH NEXT ")
			sql.WriteString(r.renderPaginationValue(query.Limit, finalCtx))
			sql.WriteString(" ROWS ONLY")
		}
	}

	return &types.QueryResult{
		SQL:            sql.String(),
		RequiredParams: params,
	}, nil
}

// validateAST checks for SQL Server-unsupported features.
func (r *Renderer) validateAST(ast *types.AST) error {
	if len(ast.DistinctOn) > 0 {
		return render.NewUnsupportedFeatureError("mssql", "DISTINCT ON",
			"use GROUP BY with aggregates or ROW_NUMBER() instead")
	}

	if ast.Lock != nil {
		return render.NewUnsupportedFeatureError("mssql", "row-level locking syntax",
			"use WITH (ROWLOCK, UPDLOCK) table hints instead")
	}

	if ast.OnConflict != nil {
		return render.NewUnsupportedFeatureError("mssql", "ON CONFLICT / upsert",
			"use MERGE statement or separate INSERT/UPDATE with EXISTS check")
	}

	// Check for JSONB fields in all field locations
	for _, field := range ast.Fields {
		if err := r.checkJSONBField(field); err != nil {
			return err
		}
	}

	for i := range ast.FieldExpressions {
		if err := r.validateFieldExpression(&ast.FieldExpressions[i]); err != nil {
			return err
		}
	}

	for _, field := range ast.GroupBy {
		if err := r.checkJSONBField(field); err != nil {
			return err
		}
	}

	for i := range ast.Ordering {
		if err := r.checkJSONBField(ast.Ordering[i].Field); err != nil {
			return err
		}
	}

	for _, field := range ast.Returning {
		if err := r.checkJSONBField(field); err != nil {
			return err
		}
	}

	for field := range ast.Updates {
		if err := r.checkJSONBField(field); err != nil {
			return err
		}
	}

	for field := range ast.UpdateExpressions {
		if err := r.checkJSONBField(field); err != nil {
			return err
		}
		exprCopy := ast.UpdateExpressions[field]
		if err := r.validateFieldExpression(&exprCopy); err != nil {
			return err
		}
	}

	for _, valueSet := range ast.Values {
		for field := range valueSet {
			if err := r.checkJSONBField(field); err != nil {
				return err
			}
		}
	}

	// Check for unsupported operators and JSONB in conditions
	if ast.WhereClause != nil {
		if err := r.validateCondition(ast.WhereClause); err != nil {
			return err
		}
	}

	for _, join := range ast.Joins {
		if join.On != nil {
			if err := r.validateCondition(join.On); err != nil {
				return err
			}
		}
	}

	for _, having := range ast.Having {
		if err := r.validateCondition(having); err != nil {
			return err
		}
	}

	return nil
}

// validateFieldExpression validates a field expression for JSONB fields.
func (r *Renderer) validateFieldExpression(expr *types.FieldExpression) error {
	if err := r.checkJSONBField(expr.Field); err != nil {
		return err
	}

	if expr.Binary != nil {
		if err := r.checkJSONBField(expr.Binary.Field); err != nil {
			return err
		}
	}

	if expr.Math != nil {
		if err := r.checkJSONBField(expr.Math.Field); err != nil {
			return err
		}
	}

	if expr.String != nil {
		if err := r.checkJSONBField(expr.String.Field); err != nil {
			return err
		}
		for _, f := range expr.String.Fields {
			if err := r.checkJSONBField(f); err != nil {
				return err
			}
		}
	}

	if expr.Date != nil && expr.Date.Field != nil {
		if err := r.checkJSONBField(*expr.Date.Field); err != nil {
			return err
		}
	}

	if expr.Cast != nil {
		if err := r.checkJSONBField(expr.Cast.Field); err != nil {
			return err
		}
	}

	if expr.Window != nil {
		if expr.Window.Field != nil {
			if err := r.checkJSONBField(*expr.Window.Field); err != nil {
				return err
			}
		}
		for _, f := range expr.Window.Window.PartitionBy {
			if err := r.checkJSONBField(f); err != nil {
				return err
			}
		}
		for i := range expr.Window.Window.OrderBy {
			if err := r.checkJSONBField(expr.Window.Window.OrderBy[i].Field); err != nil {
				return err
			}
		}
	}

	return nil
}

// validateCondition recursively checks conditions for unsupported operators and JSONB fields.
func (r *Renderer) validateCondition(cond types.ConditionItem) error {
	switch c := cond.(type) {
	case types.Condition:
		if err := r.checkJSONBField(c.Field); err != nil {
			return err
		}
		return r.validateOperator(c.Operator)
	case types.ConditionGroup:
		for _, sub := range c.Conditions {
			if err := r.validateCondition(sub); err != nil {
				return err
			}
		}
	case types.FieldComparison:
		if err := r.checkJSONBField(c.LeftField); err != nil {
			return err
		}
		if err := r.checkJSONBField(c.RightField); err != nil {
			return err
		}
		return r.validateOperator(c.Operator)
	case types.SubqueryCondition:
		if c.Field != nil {
			if err := r.checkJSONBField(*c.Field); err != nil {
				return err
			}
		}
		if err := r.validateOperator(c.Operator); err != nil {
			return err
		}
		if c.Subquery.AST != nil {
			if err := r.validateAST(c.Subquery.AST); err != nil {
				return err
			}
		}
	case types.AggregateCondition:
		if c.Field != nil {
			if err := r.checkJSONBField(*c.Field); err != nil {
				return err
			}
		}
		return r.validateOperator(c.Operator)
	case types.BetweenCondition:
		if err := r.checkJSONBField(c.Field); err != nil {
			return err
		}
	}
	return nil
}

// validateOperator checks if an operator is supported by SQL Server.
func (r *Renderer) validateOperator(op types.Operator) error {
	switch op {
	case types.ILIKE, types.NotILike:
		return render.NewUnsupportedFeatureError("mssql", "ILIKE",
			"use LIKE with COLLATE for case-insensitive matching")
	case types.RegexMatch, types.RegexIMatch, types.NotRegexMatch, types.NotRegexIMatch:
		return render.NewUnsupportedFeatureError("mssql", "regex operators",
			"use LIKE patterns or PATINDEX() instead")
	case types.ArrayContains, types.ArrayContainedBy, types.ArrayOverlap:
		return render.NewUnsupportedFeatureError("mssql", "array operators",
			"SQL Server does not have native array types")
	case types.VectorL2Distance, types.VectorInnerProduct, types.VectorCosineDistance, types.VectorL1Distance:
		return render.NewUnsupportedFeatureError("mssql", "vector operators",
			"SQL Server does not support pgvector operations")
	}
	return nil
}

func (r *Renderer) renderSelect(ast *types.AST, sql *strings.Builder, ctx *renderContext) error {
	sql.WriteString("SELECT ")

	if ast.Distinct {
		sql.WriteString("DISTINCT ")
	}

	if len(ast.Fields) == 0 && len(ast.FieldExpressions) == 0 {
		sql.WriteString("*")
	} else {
		var selections []string

		for _, field := range ast.Fields {
			if err := r.checkJSONBField(field); err != nil {
				return err
			}
			selections = append(selections, r.renderField(field))
		}

		for i := range ast.FieldExpressions {
			exprStr, err := r.renderFieldExpression(ast.FieldExpressions[i], ctx)
			if err != nil {
				return err
			}
			selections = append(selections, exprStr)
		}

		sql.WriteString(strings.Join(selections, ", "))
	}

	sql.WriteString(" FROM ")
	sql.WriteString(r.renderTable(ast.Target))

	for _, join := range ast.Joins {
		sql.WriteString(" ")
		sql.WriteString(string(join.Type))
		sql.WriteString(" ")
		sql.WriteString(r.renderTable(join.Table))
		if join.Type != types.CrossJoin {
			sql.WriteString(" ON ")
			if err := r.renderCondition(join.On, sql, ctx); err != nil {
				return err
			}
		}
	}

	if ast.WhereClause != nil {
		sql.WriteString(" WHERE ")
		if err := r.renderCondition(ast.WhereClause, sql, ctx); err != nil {
			return err
		}
	}

	if len(ast.GroupBy) > 0 {
		sql.WriteString(" GROUP BY ")
		var groupFields []string
		for _, field := range ast.GroupBy {
			groupFields = append(groupFields, r.renderField(field))
		}
		sql.WriteString(strings.Join(groupFields, ", "))
	}

	if len(ast.Having) > 0 {
		sql.WriteString(" HAVING ")
		for i, cond := range ast.Having {
			if i > 0 {
				sql.WriteString(" AND ")
			}
			if err := r.renderCondition(cond, sql, ctx); err != nil {
				return err
			}
		}
	}

	if len(ast.Ordering) > 0 {
		sql.WriteString(" ORDER BY ")
		var orderParts []string
		for i := range ast.Ordering {
			order := &ast.Ordering[i]
			var part string
			if order.Operator != "" {
				part = fmt.Sprintf("%s %s %s %s",
					r.renderField(order.Field),
					r.renderOperator(order.Operator),
					ctx.addParam(order.Param),
					order.Direction)
			} else {
				part = fmt.Sprintf("%s %s", r.renderField(order.Field), order.Direction)
			}
			// SQL Server doesn't support NULLS FIRST/LAST directly
			orderParts = append(orderParts, part)
		}
		sql.WriteString(strings.Join(orderParts, ", "))
	}

	// SQL Server uses OFFSET/FETCH instead of LIMIT/OFFSET
	if ast.Offset != nil || ast.Limit != nil {
		// OFFSET/FETCH requires ORDER BY
		if len(ast.Ordering) == 0 {
			return render.NewUnsupportedFeatureError("mssql", "LIMIT/OFFSET without ORDER BY",
				"add ORDER BY clause when using LIMIT or OFFSET")
		}

		sql.WriteString(" OFFSET ")
		if ast.Offset != nil {
			sql.WriteString(r.renderPaginationValue(ast.Offset, ctx))
		} else {
			sql.WriteString("0")
		}
		sql.WriteString(" ROWS")

		if ast.Limit != nil {
			sql.WriteString(" FETCH NEXT ")
			sql.WriteString(r.renderPaginationValue(ast.Limit, ctx))
			sql.WriteString(" ROWS ONLY")
		}
	}

	return nil
}

func (r *Renderer) renderInsert(ast *types.AST, sql *strings.Builder, addParam func(types.Param) string) error {
	sql.WriteString("INSERT INTO ")
	sql.WriteString(r.renderTable(ast.Target))

	if len(ast.Values) == 0 {
		return fmt.Errorf("INSERT requires at least one value set")
	}

	fields := make([]string, 0, len(ast.Values[0]))
	fieldObjs := make([]types.Field, 0, len(ast.Values[0]))
	for field := range ast.Values[0] {
		fieldObjs = append(fieldObjs, field)
	}

	sort.Slice(fieldObjs, func(i, j int) bool {
		return fieldObjs[i].Name < fieldObjs[j].Name
	})

	for _, field := range fieldObjs {
		fields = append(fields, r.quoteIdentifier(field.Name))
	}

	// OUTPUT clause for RETURNING (SQL Server syntax)
	if len(ast.Returning) > 0 {
		sql.WriteString(" (")
		sql.WriteString(strings.Join(fields, ", "))
		sql.WriteString(")")

		sql.WriteString(" OUTPUT ")
		var outputFields []string
		for _, field := range ast.Returning {
			outputFields = append(outputFields, "INSERTED."+r.quoteIdentifier(field.Name))
		}
		sql.WriteString(strings.Join(outputFields, ", "))

		sql.WriteString(" VALUES ")
	} else {
		sql.WriteString(" (")
		sql.WriteString(strings.Join(fields, ", "))
		sql.WriteString(") VALUES ")
	}

	valueSets := make([]string, 0, len(ast.Values))
	for _, valueSet := range ast.Values {
		var values []string
		for _, field := range fieldObjs {
			param := valueSet[field]
			values = append(values, addParam(param))
		}
		valueSets = append(valueSets, "("+strings.Join(values, ", ")+")")
	}
	sql.WriteString(strings.Join(valueSets, ", "))

	return nil
}

func (r *Renderer) renderUpdate(ast *types.AST, sql *strings.Builder, addParam func(types.Param) string) error {
	sql.WriteString("UPDATE ")
	sql.WriteString(r.renderTable(ast.Target))
	sql.WriteString(" SET ")

	updateFields := make([]types.Field, 0, len(ast.Updates))
	for field := range ast.Updates {
		updateFields = append(updateFields, field)
	}

	sort.Slice(updateFields, func(i, j int) bool {
		return updateFields[i].Name < updateFields[j].Name
	})

	updates := make([]string, 0, len(ast.Updates)+len(ast.UpdateExpressions))
	for _, field := range updateFields {
		param := ast.Updates[field]
		updates = append(updates, fmt.Sprintf("%s = %s", r.quoteIdentifier(field.Name), addParam(param)))
	}

	// Render expression-based updates
	exprFields := make([]types.Field, 0, len(ast.UpdateExpressions))
	for field := range ast.UpdateExpressions {
		exprFields = append(exprFields, field)
	}
	sort.Slice(exprFields, func(i, j int) bool {
		return exprFields[i].Name < exprFields[j].Name
	})
	for _, field := range exprFields {
		expr := ast.UpdateExpressions[field]
		ctx := newRenderContext(addParam)
		rendered, err := r.renderFieldExpression(expr, ctx)
		if err != nil {
			return err
		}
		updates = append(updates, fmt.Sprintf("%s = %s", r.quoteIdentifier(field.Name), rendered))
	}

	sql.WriteString(strings.Join(updates, ", "))

	// OUTPUT clause for RETURNING
	if len(ast.Returning) > 0 {
		sql.WriteString(" OUTPUT ")
		var outputFields []string
		for _, field := range ast.Returning {
			outputFields = append(outputFields, "INSERTED."+r.quoteIdentifier(field.Name))
		}
		sql.WriteString(strings.Join(outputFields, ", "))
	}

	if ast.WhereClause != nil {
		sql.WriteString(" WHERE ")
		ctx := newRenderContext(addParam)
		if err := r.renderCondition(ast.WhereClause, sql, ctx); err != nil {
			return err
		}
	}

	return nil
}

func (r *Renderer) renderDelete(ast *types.AST, sql *strings.Builder, addParam func(types.Param) string) error {
	sql.WriteString("DELETE FROM ")
	sql.WriteString(r.renderTable(ast.Target))

	// OUTPUT clause for RETURNING
	if len(ast.Returning) > 0 {
		sql.WriteString(" OUTPUT ")
		var outputFields []string
		for _, field := range ast.Returning {
			outputFields = append(outputFields, "DELETED."+r.quoteIdentifier(field.Name))
		}
		sql.WriteString(strings.Join(outputFields, ", "))
	}

	if ast.WhereClause != nil {
		sql.WriteString(" WHERE ")
		ctx := newRenderContext(addParam)
		if err := r.renderCondition(ast.WhereClause, sql, ctx); err != nil {
			return err
		}
	}

	return nil
}

func (r *Renderer) renderCount(ast *types.AST, sql *strings.Builder, addParam func(types.Param) string) error {
	sql.WriteString("SELECT " + countStarSQL + " FROM ")
	sql.WriteString(r.renderTable(ast.Target))

	for _, join := range ast.Joins {
		sql.WriteString(" ")
		sql.WriteString(string(join.Type))
		sql.WriteString(" ")
		sql.WriteString(r.renderTable(join.Table))
		if join.Type != types.CrossJoin {
			sql.WriteString(" ON ")
			ctx := newRenderContext(addParam)
			if err := r.renderCondition(join.On, sql, ctx); err != nil {
				return err
			}
		}
	}

	if ast.WhereClause != nil {
		sql.WriteString(" WHERE ")
		ctx := newRenderContext(addParam)
		if err := r.renderCondition(ast.WhereClause, sql, ctx); err != nil {
			return err
		}
	}

	return nil
}

// quoteIdentifier quotes a SQL Server identifier with square brackets.
func (r *Renderer) quoteIdentifier(name string) string {
	escaped := strings.ReplaceAll(name, "]", "]]")
	return "[" + escaped + "]"
}

func (r *Renderer) renderTable(table types.Table) string {
	quotedName := r.quoteIdentifier(table.Name)
	if table.Alias != "" {
		return fmt.Sprintf("%s %s", quotedName, table.Alias)
	}
	return quotedName
}

func (r *Renderer) renderField(field types.Field) string {
	quotedName := r.quoteIdentifier(field.Name)
	if field.Table != "" {
		return fmt.Sprintf("%s.%s", field.Table, quotedName)
	}
	return quotedName
}

// checkJSONBField returns an error if the field uses JSONB access operators.
func (r *Renderer) checkJSONBField(field types.Field) error {
	if field.JSONBTextKey != nil || field.JSONBPathKey != nil {
		return render.NewUnsupportedFeatureError("mssql", "JSONB field access operators",
			"use JSON_VALUE() or JSON_QUERY() instead")
	}
	return nil
}

// renderPaginationValue renders a pagination value.
func (r *Renderer) renderPaginationValue(pv *types.PaginationValue, ctx *renderContext) string {
	if pv.Param != nil {
		return ctx.addParam(*pv.Param)
	}
	if pv.Static != nil {
		return fmt.Sprintf("%d", *pv.Static)
	}
	return "0"
}

func (r *Renderer) renderAggregateExpression(aggregate types.AggregateFunc, field types.Field) string {
	switch aggregate {
	case types.AggCountField:
		if field.Name == "" {
			return countStarSQL
		}
		return fmt.Sprintf("COUNT(%s)", r.renderField(field))
	case types.AggCountDistinct:
		return fmt.Sprintf("COUNT(DISTINCT %s)", r.renderField(field))
	case types.AggSum:
		return fmt.Sprintf("SUM(%s)", r.renderField(field))
	case types.AggAvg:
		return fmt.Sprintf("AVG(%s)", r.renderField(field))
	case types.AggMin:
		return fmt.Sprintf("MIN(%s)", r.renderField(field))
	case types.AggMax:
		return fmt.Sprintf("MAX(%s)", r.renderField(field))
	default:
		return r.renderField(field)
	}
}

func (r *Renderer) renderFieldExpression(expr types.FieldExpression, ctx *renderContext) (string, error) {
	var result string

	switch {
	case expr.Case != nil:
		caseStr, err := r.renderCaseExpression(*expr.Case, ctx)
		if err != nil {
			return "", err
		}
		result = caseStr
	case expr.Coalesce != nil:
		coalesceStr, err := r.renderCoalesceExpression(*expr.Coalesce, ctx)
		if err != nil {
			return "", err
		}
		result = coalesceStr
	case expr.NullIf != nil:
		nullifStr, err := r.renderNullIfExpression(*expr.NullIf, ctx)
		if err != nil {
			return "", err
		}
		result = nullifStr
	case expr.Math != nil:
		mathStr, err := r.renderMathExpression(*expr.Math, ctx)
		if err != nil {
			return "", err
		}
		result = mathStr
	case expr.String != nil:
		stringStr, err := r.renderStringExpression(*expr.String, ctx)
		if err != nil {
			return "", err
		}
		result = stringStr
	case expr.Date != nil:
		dateStr, err := r.renderDateExpression(*expr.Date, ctx)
		if err != nil {
			return "", err
		}
		result = dateStr
	case expr.Cast != nil:
		result = fmt.Sprintf("CAST(%s AS %s)", r.renderField(expr.Cast.Field), r.mapCastType(expr.Cast.CastType))
	case expr.Window != nil:
		windowStr, err := r.renderWindowExpression(*expr.Window, ctx)
		if err != nil {
			return "", err
		}
		result = windowStr
	case expr.Binary != nil:
		// Validate operator
		if err := r.validateOperator(expr.Binary.Operator); err != nil {
			return "", err
		}
		// Check for unsupported JSONB field
		if err := r.checkJSONBField(expr.Binary.Field); err != nil {
			return "", err
		}
		// Render binary expression
		paramStr := ctx.addParam(expr.Binary.Param)
		opStr := r.renderOperator(expr.Binary.Operator)
		result = fmt.Sprintf("%s %s %s", r.renderField(expr.Binary.Field), opStr, paramStr)
	case expr.Aggregate != "":
		result = r.renderAggregateExpression(expr.Aggregate, expr.Field)
		if expr.Filter != nil {
			// SQL Server doesn't support FILTER clause
			return "", render.NewUnsupportedFeatureError("mssql", "FILTER clause on aggregates",
				"use CASE WHEN inside the aggregate instead")
		}
	default:
		result = r.renderField(expr.Field)
	}

	if expr.Alias != "" {
		result += " AS " + r.quoteIdentifier(expr.Alias)
	}

	return result, nil
}

// mapCastType maps PostgreSQL cast types to SQL Server equivalents.
func (r *Renderer) mapCastType(castType types.CastType) string {
	switch castType {
	case types.CastText:
		return "NVARCHAR(MAX)"
	case types.CastInteger:
		return "INT"
	case types.CastSmallint:
		return "SMALLINT"
	case types.CastBigint:
		return "BIGINT"
	case types.CastNumeric:
		return "DECIMAL"
	case types.CastReal:
		return "REAL"
	case types.CastDoublePrecision:
		return "FLOAT"
	case types.CastBoolean:
		return "BIT"
	case types.CastBytea:
		return "VARBINARY(MAX)"
	default:
		return "NVARCHAR(MAX)"
	}
}

func (r *Renderer) renderCondition(cond types.ConditionItem, sql *strings.Builder, ctx *renderContext) error {
	switch c := cond.(type) {
	case types.Condition:
		sql.WriteString(r.renderSimpleCondition(c, ctx.addParam))
	case types.ConditionGroup:
		if len(c.Conditions) == 0 {
			return fmt.Errorf("empty condition group")
		}
		sql.WriteString("(")
		for i, subCond := range c.Conditions {
			if i > 0 {
				fmt.Fprintf(sql, " %s ", c.Logic)
			}
			if err := r.renderCondition(subCond, sql, ctx); err != nil {
				return err
			}
		}
		sql.WriteString(")")
	case types.FieldComparison:
		fmt.Fprintf(sql, "%s %s %s",
			r.renderField(c.LeftField),
			r.renderOperator(c.Operator),
			r.renderField(c.RightField))
	case types.SubqueryCondition:
		if err := r.renderSubqueryCondition(c, sql, ctx); err != nil {
			return err
		}
	case types.AggregateCondition:
		sql.WriteString(r.renderAggregateCondition(c, ctx.addParam))
	case types.BetweenCondition:
		sql.WriteString(r.renderBetweenCondition(c, ctx.addParam))
	default:
		return fmt.Errorf("unknown condition type: %T", c)
	}
	return nil
}

func (r *Renderer) renderSimpleCondition(cond types.Condition, addParam func(types.Param) string) string {
	field := r.renderField(cond.Field)
	op := r.renderOperator(cond.Operator)

	switch cond.Operator {
	case types.IsNull:
		return fmt.Sprintf("%s IS NULL", field)
	case types.IsNotNull:
		return fmt.Sprintf("%s IS NOT NULL", field)
	case types.IN:
		return fmt.Sprintf("%s IN (%s)", field, addParam(cond.Value))
	case types.NotIn:
		return fmt.Sprintf("%s NOT IN (%s)", field, addParam(cond.Value))
	default:
		return fmt.Sprintf("%s %s %s", field, op, addParam(cond.Value))
	}
}

func (r *Renderer) renderAggregateCondition(cond types.AggregateCondition, addParam func(types.Param) string) string {
	var aggExpr string

	switch cond.Func {
	case types.AggCountField:
		if cond.Field == nil {
			aggExpr = countStarSQL
		} else {
			aggExpr = fmt.Sprintf("COUNT(%s)", r.renderField(*cond.Field))
		}
	case types.AggCountDistinct:
		if cond.Field == nil {
			aggExpr = countStarSQL
		} else {
			aggExpr = fmt.Sprintf("COUNT(DISTINCT %s)", r.renderField(*cond.Field))
		}
	case types.AggSum:
		if cond.Field == nil {
			aggExpr = "SUM(*)"
		} else {
			aggExpr = fmt.Sprintf("SUM(%s)", r.renderField(*cond.Field))
		}
	case types.AggAvg:
		if cond.Field == nil {
			aggExpr = "AVG(*)"
		} else {
			aggExpr = fmt.Sprintf("AVG(%s)", r.renderField(*cond.Field))
		}
	case types.AggMin:
		if cond.Field == nil {
			aggExpr = "MIN(*)"
		} else {
			aggExpr = fmt.Sprintf("MIN(%s)", r.renderField(*cond.Field))
		}
	case types.AggMax:
		if cond.Field == nil {
			aggExpr = "MAX(*)"
		} else {
			aggExpr = fmt.Sprintf("MAX(%s)", r.renderField(*cond.Field))
		}
	default:
		aggExpr = "UNKNOWN_AGG(*)"
	}

	return fmt.Sprintf("%s %s %s", aggExpr, r.renderOperator(cond.Operator), addParam(cond.Value))
}

func (r *Renderer) renderBetweenCondition(cond types.BetweenCondition, addParam func(types.Param) string) string {
	field := r.renderField(cond.Field)
	op := "BETWEEN"
	if cond.Negated {
		op = "NOT BETWEEN"
	}
	return fmt.Sprintf("%s %s %s AND %s", field, op, addParam(cond.Low), addParam(cond.High))
}

func (r *Renderer) renderSubqueryCondition(cond types.SubqueryCondition, sql *strings.Builder, ctx *renderContext) error {
	switch cond.Operator {
	case types.EXISTS, types.NotExists:
		sql.WriteString(string(cond.Operator))
		sql.WriteString(" ")
	default:
		if cond.Field == nil {
			return fmt.Errorf("operator %s requires a field", cond.Operator)
		}
		sql.WriteString(r.renderField(*cond.Field))
		sql.WriteString(" ")
		sql.WriteString(string(cond.Operator))
		sql.WriteString(" ")
	}

	sql.WriteString("(")
	if err := r.renderSubquery(cond.Subquery, sql, ctx); err != nil {
		return err
	}
	sql.WriteString(")")

	return nil
}

func (r *Renderer) renderSubquery(subquery types.Subquery, sql *strings.Builder, ctx *renderContext) error {
	subCtx, err := ctx.withSubquery()
	if err != nil {
		return err
	}

	return r.renderSelect(subquery.AST, sql, subCtx)
}

func (r *Renderer) renderCaseExpression(expr types.CaseExpression, ctx *renderContext) (string, error) {
	var sql strings.Builder
	sql.WriteString("CASE")

	for _, when := range expr.WhenClauses {
		sql.WriteString(" WHEN ")
		if err := r.renderCondition(when.Condition, &sql, ctx); err != nil {
			return "", err
		}
		sql.WriteString(" THEN ")
		sql.WriteString(ctx.addParam(when.Result))
	}

	if expr.ElseValue != nil {
		sql.WriteString(" ELSE ")
		sql.WriteString(ctx.addParam(*expr.ElseValue))
	}

	sql.WriteString(" END")
	return sql.String(), nil
}

func (r *Renderer) renderCoalesceExpression(expr types.CoalesceExpression, ctx *renderContext) (string, error) {
	var sql strings.Builder
	sql.WriteString("COALESCE(")

	params := make([]string, 0, len(expr.Values))
	for _, value := range expr.Values {
		params = append(params, ctx.addParam(value))
	}
	sql.WriteString(strings.Join(params, ", "))
	sql.WriteString(")")
	return sql.String(), nil
}

func (r *Renderer) renderNullIfExpression(expr types.NullIfExpression, ctx *renderContext) (string, error) {
	var sql strings.Builder
	sql.WriteString("NULLIF(")
	sql.WriteString(ctx.addParam(expr.Value1))
	sql.WriteString(", ")
	sql.WriteString(ctx.addParam(expr.Value2))
	sql.WriteString(")")
	return sql.String(), nil
}

func (r *Renderer) renderMathExpression(expr types.MathExpression, ctx *renderContext) (string, error) {
	var sql strings.Builder

	switch expr.Function {
	case types.MathRound:
		sql.WriteString("ROUND(")
		sql.WriteString(r.renderField(expr.Field))
		sql.WriteString(", ")
		if expr.Precision != nil {
			sql.WriteString(ctx.addParam(*expr.Precision))
		} else {
			sql.WriteString("0")
		}
		sql.WriteString(")")
	case types.MathFloor:
		sql.WriteString("FLOOR(")
		sql.WriteString(r.renderField(expr.Field))
		sql.WriteString(")")
	case types.MathCeil:
		sql.WriteString("CEILING(")
		sql.WriteString(r.renderField(expr.Field))
		sql.WriteString(")")
	case types.MathAbs:
		sql.WriteString("ABS(")
		sql.WriteString(r.renderField(expr.Field))
		sql.WriteString(")")
	case types.MathPower:
		sql.WriteString("POWER(")
		sql.WriteString(r.renderField(expr.Field))
		if expr.Exponent != nil {
			sql.WriteString(", ")
			sql.WriteString(ctx.addParam(*expr.Exponent))
		}
		sql.WriteString(")")
	case types.MathSqrt:
		sql.WriteString("SQRT(")
		sql.WriteString(r.renderField(expr.Field))
		sql.WriteString(")")
	default:
		return "", fmt.Errorf("unsupported math function: %s", expr.Function)
	}

	return sql.String(), nil
}

func (r *Renderer) renderStringExpression(expr types.StringExpression, ctx *renderContext) (string, error) {
	var sql strings.Builder

	switch expr.Function {
	case types.StringUpper:
		sql.WriteString("UPPER(")
		sql.WriteString(r.renderField(expr.Field))
		sql.WriteString(")")
	case types.StringLower:
		sql.WriteString("LOWER(")
		sql.WriteString(r.renderField(expr.Field))
		sql.WriteString(")")
	case types.StringTrim:
		sql.WriteString("TRIM(")
		sql.WriteString(r.renderField(expr.Field))
		sql.WriteString(")")
	case types.StringLTrim:
		sql.WriteString("LTRIM(")
		sql.WriteString(r.renderField(expr.Field))
		sql.WriteString(")")
	case types.StringRTrim:
		sql.WriteString("RTRIM(")
		sql.WriteString(r.renderField(expr.Field))
		sql.WriteString(")")
	case types.StringLength:
		// SQL Server uses LEN()
		sql.WriteString("LEN(")
		sql.WriteString(r.renderField(expr.Field))
		sql.WriteString(")")
	case types.StringSubstring:
		// SQL Server uses SUBSTRING(string, start, length)
		sql.WriteString("SUBSTRING(")
		sql.WriteString(r.renderField(expr.Field))
		if len(expr.Args) >= 2 {
			sql.WriteString(", ")
			sql.WriteString(ctx.addParam(expr.Args[0]))
			sql.WriteString(", ")
			sql.WriteString(ctx.addParam(expr.Args[1]))
		}
		sql.WriteString(")")
	case types.StringReplace:
		sql.WriteString("REPLACE(")
		sql.WriteString(r.renderField(expr.Field))
		if len(expr.Args) >= 2 {
			sql.WriteString(", ")
			sql.WriteString(ctx.addParam(expr.Args[0]))
			sql.WriteString(", ")
			sql.WriteString(ctx.addParam(expr.Args[1]))
		}
		sql.WriteString(")")
	case types.StringConcat:
		sql.WriteString("CONCAT(")
		sql.WriteString(r.renderField(expr.Field))
		for _, f := range expr.Fields {
			sql.WriteString(", ")
			sql.WriteString(r.renderField(f))
		}
		sql.WriteString(")")
	default:
		return "", fmt.Errorf("unsupported string function: %s", expr.Function)
	}

	return sql.String(), nil
}

func (r *Renderer) renderDateExpression(expr types.DateExpression, _ *renderContext) (string, error) {
	var sql strings.Builder

	switch expr.Function {
	case types.DateNow:
		sql.WriteString("GETDATE()")
	case types.DateCurrentDate:
		sql.WriteString("CAST(GETDATE() AS DATE)")
	case types.DateCurrentTime:
		sql.WriteString("CAST(GETDATE() AS TIME)")
	case types.DateCurrentTimestamp:
		sql.WriteString("GETDATE()")
	case types.DateExtract:
		if expr.Field == nil {
			return "", fmt.Errorf("EXTRACT requires a field")
		}
		// SQL Server uses DATEPART
		part := r.datePartToMSSQL(expr.Part)
		if part == "" {
			return "", render.NewUnsupportedFeatureError("mssql", fmt.Sprintf("EXTRACT %s", expr.Part),
				"use DATEPART with appropriate part name")
		}
		sql.WriteString("DATEPART(")
		sql.WriteString(part)
		sql.WriteString(", ")
		sql.WriteString(r.renderField(*expr.Field))
		sql.WriteString(")")
	case types.DateTrunc:
		if expr.Field == nil {
			return "", fmt.Errorf("DATE_TRUNC requires a field")
		}
		// SQL Server doesn't have DATE_TRUNC, need to use DATETRUNC (2022+) or DATEADD/DATEDIFF
		switch expr.Part {
		case types.PartDay:
			sql.WriteString("CAST(")
			sql.WriteString(r.renderField(*expr.Field))
			sql.WriteString(" AS DATE)")
		case types.PartMonth:
			sql.WriteString("DATEFROMPARTS(YEAR(")
			sql.WriteString(r.renderField(*expr.Field))
			sql.WriteString("), MONTH(")
			sql.WriteString(r.renderField(*expr.Field))
			sql.WriteString("), 1)")
		case types.PartYear:
			sql.WriteString("DATEFROMPARTS(YEAR(")
			sql.WriteString(r.renderField(*expr.Field))
			sql.WriteString("), 1, 1)")
		default:
			return "", render.NewUnsupportedFeatureError("mssql", fmt.Sprintf("DATE_TRUNC with %s precision", expr.Part),
				"use DATEADD/DATEDIFF or DATEFROMPARTS for date truncation")
		}
	default:
		return "", fmt.Errorf("unsupported date function: %s", expr.Function)
	}

	return sql.String(), nil
}

// datePartToMSSQL maps DatePart to SQL Server DATEPART names.
func (r *Renderer) datePartToMSSQL(part types.DatePart) string {
	switch part {
	case types.PartYear:
		return "YEAR"
	case types.PartMonth:
		return "MONTH"
	case types.PartDay:
		return "DAY"
	case types.PartHour:
		return "HOUR"
	case types.PartMinute:
		return "MINUTE"
	case types.PartSecond:
		return "SECOND"
	case types.PartWeek:
		return "WEEK"
	case types.PartDayOfWeek:
		return "WEEKDAY"
	case types.PartDayOfYear:
		return "DAYOFYEAR"
	case types.PartQuarter:
		return "QUARTER"
	default:
		return ""
	}
}

func (r *Renderer) renderWindowExpression(expr types.WindowExpression, ctx *renderContext) (string, error) {
	var sql strings.Builder

	switch expr.Function {
	case types.WinRowNumber, types.WinRank, types.WinDenseRank:
		sql.WriteString(string(expr.Function))
		sql.WriteString("()")
	case types.WinNtile:
		sql.WriteString("NTILE(")
		if expr.NtileParam != nil {
			sql.WriteString(ctx.addParam(*expr.NtileParam))
		} else {
			return "", fmt.Errorf("NTILE requires a parameter")
		}
		sql.WriteString(")")
	case types.WinLag, types.WinLead:
		sql.WriteString(string(expr.Function))
		sql.WriteString("(")
		if expr.Field != nil {
			sql.WriteString(r.renderField(*expr.Field))
		} else {
			return "", fmt.Errorf("%s requires a field", expr.Function)
		}
		if expr.LagOffset != nil {
			sql.WriteString(", ")
			sql.WriteString(ctx.addParam(*expr.LagOffset))
		}
		if expr.LagDefault != nil {
			sql.WriteString(", ")
			sql.WriteString(ctx.addParam(*expr.LagDefault))
		}
		sql.WriteString(")")
	case types.WinFirstValue, types.WinLastValue:
		sql.WriteString(string(expr.Function))
		sql.WriteString("(")
		if expr.Field != nil {
			sql.WriteString(r.renderField(*expr.Field))
		} else {
			return "", fmt.Errorf("%s requires a field", expr.Function)
		}
		sql.WriteString(")")
	default:
		if expr.Aggregate != "" {
			if expr.Field != nil {
				sql.WriteString(r.renderAggregateExpression(expr.Aggregate, *expr.Field))
			} else {
				sql.WriteString(countStarSQL)
			}
		} else {
			return "", fmt.Errorf("unknown window function: %s", expr.Function)
		}
	}

	sql.WriteString(" OVER (")

	var overParts []string

	if len(expr.Window.PartitionBy) > 0 {
		var partitionFields []string
		for _, field := range expr.Window.PartitionBy {
			partitionFields = append(partitionFields, r.renderField(field))
		}
		overParts = append(overParts, "PARTITION BY "+strings.Join(partitionFields, ", "))
	}

	if len(expr.Window.OrderBy) > 0 {
		var orderParts []string
		for i := range expr.Window.OrderBy {
			order := &expr.Window.OrderBy[i]
			var part string
			if order.Operator != "" {
				part = fmt.Sprintf("%s %s %s %s",
					r.renderField(order.Field),
					r.renderOperator(order.Operator),
					ctx.addParam(order.Param),
					order.Direction)
			} else {
				part = fmt.Sprintf("%s %s", r.renderField(order.Field), order.Direction)
			}
			orderParts = append(orderParts, part)
		}
		overParts = append(overParts, "ORDER BY "+strings.Join(orderParts, ", "))
	}

	if expr.Window.FrameStart != "" {
		framePart := "ROWS BETWEEN " + string(expr.Window.FrameStart) + " AND "
		if expr.Window.FrameEnd != "" {
			framePart += string(expr.Window.FrameEnd)
		} else {
			framePart += "CURRENT ROW"
		}
		overParts = append(overParts, framePart)
	}

	sql.WriteString(strings.Join(overParts, " "))
	sql.WriteString(")")

	return sql.String(), nil
}

func (r *Renderer) renderOperator(op types.Operator) string {
	switch op {
	case types.EQ:
		return "="
	case types.NE:
		return "<>"
	case types.GT:
		return ">"
	case types.GE:
		return ">="
	case types.LT:
		return "<"
	case types.LE:
		return "<="
	case types.LIKE:
		return "LIKE"
	case types.NotLike:
		return "NOT LIKE"
	case types.IN:
		return "IN"
	case types.NotIn:
		return "NOT IN"
	case types.EXISTS:
		return "EXISTS"
	case types.NotExists:
		return "NOT EXISTS"
	default:
		return string(op)
	}
}

// Capabilities returns the SQL features supported by SQL Server.
func (r *Renderer) Capabilities() render.Capabilities {
	return render.Capabilities{
		DistinctOn:          false,
		Upsert:              false,
		ReturningOnInsert:   false,
		ReturningOnUpdate:   false,
		ReturningOnDelete:   false,
		CaseInsensitiveLike: false,
		RegexOperators:      false,
		ArrayOperators:      false,
		InArray:             true,
		RowLocking:          render.RowLockingNone,
	}
}
