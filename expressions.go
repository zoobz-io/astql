package astql

import (
	"fmt"

	"github.com/zoobz-io/astql/internal/types"
)

// Helper functions for creating field expressions.

// Sum creates a SUM aggregate expression.
func Sum(field types.Field) types.FieldExpression {
	return types.FieldExpression{
		Field:     field,
		Aggregate: types.AggSum,
	}
}

// Avg creates an AVG aggregate expression.
func Avg(field types.Field) types.FieldExpression {
	return types.FieldExpression{
		Field:     field,
		Aggregate: types.AggAvg,
	}
}

// Min creates a MIN aggregate expression.
func Min(field types.Field) types.FieldExpression {
	return types.FieldExpression{
		Field:     field,
		Aggregate: types.AggMin,
	}
}

// Max creates a MAX aggregate expression.
func Max(field types.Field) types.FieldExpression {
	return types.FieldExpression{
		Field:     field,
		Aggregate: types.AggMax,
	}
}

// CountField creates a COUNT aggregate expression for a specific field.
func CountField(field types.Field) types.FieldExpression {
	return types.FieldExpression{
		Field:     field,
		Aggregate: types.AggCountField,
	}
}

// CountDistinct creates a COUNT(DISTINCT) aggregate expression.
func CountDistinct(field types.Field) types.FieldExpression {
	return types.FieldExpression{
		Field:     field,
		Aggregate: types.AggCountDistinct,
	}
}

// CountStar creates a COUNT(*) aggregate expression for use in SELECT.
func CountStar() types.FieldExpression {
	return types.FieldExpression{
		Aggregate: types.AggCountField,
		// Field is zero value (empty), which renders as COUNT(*)
	}
}

// Example: SumFilter(field, condition) -> SUM("field") FILTER (WHERE condition).
func SumFilter(field types.Field, filter types.ConditionItem) types.FieldExpression {
	return types.FieldExpression{
		Field:     field,
		Aggregate: types.AggSum,
		Filter:    filter,
	}
}

// AvgFilter creates an AVG aggregate with a FILTER clause.
func AvgFilter(field types.Field, filter types.ConditionItem) types.FieldExpression {
	return types.FieldExpression{
		Field:     field,
		Aggregate: types.AggAvg,
		Filter:    filter,
	}
}

// MinFilter creates a MIN aggregate with a FILTER clause.
func MinFilter(field types.Field, filter types.ConditionItem) types.FieldExpression {
	return types.FieldExpression{
		Field:     field,
		Aggregate: types.AggMin,
		Filter:    filter,
	}
}

// MaxFilter creates a MAX aggregate with a FILTER clause.
func MaxFilter(field types.Field, filter types.ConditionItem) types.FieldExpression {
	return types.FieldExpression{
		Field:     field,
		Aggregate: types.AggMax,
		Filter:    filter,
	}
}

// CountFieldFilter creates a COUNT(field) aggregate with a FILTER clause.
func CountFieldFilter(field types.Field, filter types.ConditionItem) types.FieldExpression {
	return types.FieldExpression{
		Field:     field,
		Aggregate: types.AggCountField,
		Filter:    filter,
	}
}

// CountDistinctFilter creates a COUNT(DISTINCT field) aggregate with a FILTER clause.
func CountDistinctFilter(field types.Field, filter types.ConditionItem) types.FieldExpression {
	return types.FieldExpression{
		Field:     field,
		Aggregate: types.AggCountDistinct,
		Filter:    filter,
	}
}

// Example: Between(field, low, high) -> field BETWEEN :low AND :high.
func Between(field types.Field, low, high types.Param) types.BetweenCondition {
	return types.BetweenCondition{
		Field:   field,
		Low:     low,
		High:    high,
		Negated: false,
	}
}

// Example: NotBetween(field, low, high) -> field NOT BETWEEN :low AND :high.
func NotBetween(field types.Field, low, high types.Param) types.BetweenCondition {
	return types.BetweenCondition{
		Field:   field,
		Low:     low,
		High:    high,
		Negated: true,
	}
}

// CF creates a field comparison condition.
func CF(left types.Field, op types.Operator, right types.Field) types.FieldComparison {
	return types.FieldComparison{
		LeftField:  left,
		Operator:   op,
		RightField: right,
	}
}

// CSub creates a subquery condition with a field.
func CSub(field types.Field, op types.Operator, subquery types.Subquery) types.SubqueryCondition {
	// Validate operator is appropriate for subqueries
	switch op {
	case types.IN, types.NotIn:
		// Valid operators that require a field
	default:
		panic(fmt.Errorf("operator %s cannot be used with CSub - use CSubExists for EXISTS/NOT EXISTS", op))
	}

	return types.SubqueryCondition{
		Field:    &field,
		Operator: op,
		Subquery: subquery,
	}
}

// CSubExists creates an EXISTS/NOT EXISTS subquery condition.
func CSubExists(op types.Operator, subquery types.Subquery) types.SubqueryCondition {
	// Validate operator
	switch op {
	case types.EXISTS, types.NotExists:
		// Valid operators
	default:
		panic(fmt.Errorf("CSubExists only accepts EXISTS or NOT EXISTS, got %s", op))
	}

	return types.SubqueryCondition{
		Field:    nil,
		Operator: op,
		Subquery: subquery,
	}
}

// Sub creates a subquery from a builder.
func Sub(builder *Builder) types.Subquery {
	ast, err := builder.Build()
	if err != nil {
		panic(fmt.Errorf("failed to build subquery: %w", err))
	}
	return types.Subquery{AST: ast}
}

// Case creates a new CASE expression builder.
func Case() *CaseBuilder {
	return &CaseBuilder{
		expr: &types.CaseExpression{},
	}
}

// CaseBuilder provides fluent API for building CASE expressions.
type CaseBuilder struct {
	expr *types.CaseExpression
}

// When adds a WHEN...THEN clause.
func (cb *CaseBuilder) When(condition types.ConditionItem, result types.Param) *CaseBuilder {
	cb.expr.WhenClauses = append(cb.expr.WhenClauses, types.WhenClause{
		Condition: condition,
		Result:    result,
	})
	return cb
}

// Else sets the ELSE clause.
func (cb *CaseBuilder) Else(result types.Param) *CaseBuilder {
	cb.expr.ElseValue = &result
	return cb
}

// As adds an alias to the CASE expression.
func (cb *CaseBuilder) As(alias string) *CaseBuilder {
	if !isValidSQLIdentifier(alias) {
		panic(fmt.Errorf("invalid alias '%s': must be alphanumeric/underscore, start with letter/underscore, and contain no SQL keywords", alias))
	}
	cb.expr.Alias = alias
	return cb
}

// Build returns the CaseExpression wrapped in a FieldExpression.
func (cb *CaseBuilder) Build() types.FieldExpression {
	return types.FieldExpression{
		Case:  cb.expr,
		Alias: cb.expr.Alias,
	}
}

// Coalesce creates a COALESCE expression that returns the first non-null value.
func Coalesce(values ...types.Param) types.FieldExpression {
	if len(values) < 2 {
		panic("COALESCE requires at least 2 values")
	}
	return types.FieldExpression{
		Coalesce: &types.CoalesceExpression{Values: values},
	}
}

// NullIf creates a NULLIF expression that returns NULL if two values are equal.
func NullIf(value1, value2 types.Param) types.FieldExpression {
	return types.FieldExpression{
		NullIf: &types.NullIfExpression{
			Value1: value1,
			Value2: value2,
		},
	}
}

// Round creates a ROUND math expression.
func Round(field types.Field, precision ...types.Param) types.FieldExpression {
	expr := types.MathExpression{
		Function: types.MathRound,
		Field:    field,
	}
	if len(precision) > 0 {
		expr.Precision = &precision[0]
	}
	return types.FieldExpression{
		Math: &expr,
	}
}

// Floor creates a FLOOR math expression.
func Floor(field types.Field) types.FieldExpression {
	return types.FieldExpression{
		Math: &types.MathExpression{
			Function: types.MathFloor,
			Field:    field,
		},
	}
}

// Ceil creates a CEIL math expression.
func Ceil(field types.Field) types.FieldExpression {
	return types.FieldExpression{
		Math: &types.MathExpression{
			Function: types.MathCeil,
			Field:    field,
		},
	}
}

// Abs creates an ABS math expression.
func Abs(field types.Field) types.FieldExpression {
	return types.FieldExpression{
		Math: &types.MathExpression{
			Function: types.MathAbs,
			Field:    field,
		},
	}
}

// Power creates a POWER math expression.
func Power(field types.Field, exponent types.Param) types.FieldExpression {
	return types.FieldExpression{
		Math: &types.MathExpression{
			Function: types.MathPower,
			Field:    field,
			Exponent: &exponent,
		},
	}
}

// Sqrt creates a SQRT math expression.
func Sqrt(field types.Field) types.FieldExpression {
	return types.FieldExpression{
		Math: &types.MathExpression{
			Function: types.MathSqrt,
			Field:    field,
		},
	}
}

// Example: Cast(field, CastText) -> CAST("field" AS TEXT).
func Cast(field types.Field, castType types.CastType) types.FieldExpression {
	return types.FieldExpression{
		Cast: &types.CastExpression{
			Field:    field,
			CastType: castType,
		},
	}
}

// BinaryExpr creates a binary expression for field <op> param patterns.
// Commonly used for vector distance calculations, e.g., embedding <-> :query
// Example: BinaryExpr(embedding, VectorL2Distance, query) -> "embedding" <-> :query
func BinaryExpr(field types.Field, op types.Operator, param types.Param) types.FieldExpression {
	return types.FieldExpression{
		Binary: &types.BinaryExpression{
			Field:    field,
			Operator: op,
			Param:    param,
		},
	}
}

// String functions

// Upper creates an UPPER string expression.
// Example: Upper(field) -> UPPER("field")
func Upper(field types.Field) types.FieldExpression {
	return types.FieldExpression{
		String: &types.StringExpression{
			Function: types.StringUpper,
			Field:    field,
		},
	}
}

// Lower creates a LOWER string expression.
// Example: Lower(field) -> LOWER("field")
func Lower(field types.Field) types.FieldExpression {
	return types.FieldExpression{
		String: &types.StringExpression{
			Function: types.StringLower,
			Field:    field,
		},
	}
}

// Trim creates a TRIM string expression.
// Example: Trim(field) -> TRIM("field")
func Trim(field types.Field) types.FieldExpression {
	return types.FieldExpression{
		String: &types.StringExpression{
			Function: types.StringTrim,
			Field:    field,
		},
	}
}

// LTrim creates an LTRIM string expression.
// Example: LTrim(field) -> LTRIM("field")
func LTrim(field types.Field) types.FieldExpression {
	return types.FieldExpression{
		String: &types.StringExpression{
			Function: types.StringLTrim,
			Field:    field,
		},
	}
}

// RTrim creates an RTRIM string expression.
// Example: RTrim(field) -> RTRIM("field")
func RTrim(field types.Field) types.FieldExpression {
	return types.FieldExpression{
		String: &types.StringExpression{
			Function: types.StringRTrim,
			Field:    field,
		},
	}
}

// Length creates a LENGTH string expression.
// Example: Length(field) -> LENGTH("field")
func Length(field types.Field) types.FieldExpression {
	return types.FieldExpression{
		String: &types.StringExpression{
			Function: types.StringLength,
			Field:    field,
		},
	}
}

// Substring creates a SUBSTRING string expression.
// Example: Substring(field, start, length) -> SUBSTRING("field", :start, :length)
func Substring(field types.Field, start types.Param, length types.Param) types.FieldExpression {
	return types.FieldExpression{
		String: &types.StringExpression{
			Function: types.StringSubstring,
			Field:    field,
			Args:     []types.Param{start, length},
		},
	}
}

// Replace creates a REPLACE string expression.
// Example: Replace(field, search, replacement) -> REPLACE("field", :search, :replacement)
func Replace(field types.Field, search types.Param, replacement types.Param) types.FieldExpression {
	return types.FieldExpression{
		String: &types.StringExpression{
			Function: types.StringReplace,
			Field:    field,
			Args:     []types.Param{search, replacement},
		},
	}
}

// Concat creates a CONCAT string expression with multiple fields.
// Example: Concat(field1, field2) -> CONCAT("field1", "field2")
func Concat(fields ...types.Field) types.FieldExpression {
	if len(fields) == 0 {
		return types.FieldExpression{}
	}
	return types.FieldExpression{
		String: &types.StringExpression{
			Function: types.StringConcat,
			Field:    fields[0],
			Fields:   fields[1:],
		},
	}
}

// Date/time functions

// Now creates a NOW() date expression returning current timestamp.
// Example: Now() -> NOW()
func Now() types.FieldExpression {
	return types.FieldExpression{
		Date: &types.DateExpression{
			Function: types.DateNow,
		},
	}
}

// CurrentDate creates a CURRENT_DATE expression returning current date.
// Example: CurrentDate() -> CURRENT_DATE
func CurrentDate() types.FieldExpression {
	return types.FieldExpression{
		Date: &types.DateExpression{
			Function: types.DateCurrentDate,
		},
	}
}

// CurrentTime creates a CURRENT_TIME expression returning current time.
// Example: CurrentTime() -> CURRENT_TIME
func CurrentTime() types.FieldExpression {
	return types.FieldExpression{
		Date: &types.DateExpression{
			Function: types.DateCurrentTime,
		},
	}
}

// CurrentTimestamp creates a CURRENT_TIMESTAMP expression.
// Example: CurrentTimestamp() -> CURRENT_TIMESTAMP
func CurrentTimestamp() types.FieldExpression {
	return types.FieldExpression{
		Date: &types.DateExpression{
			Function: types.DateCurrentTimestamp,
		},
	}
}

// Extract creates an EXTRACT expression to get a part of a date/time field.
// Example: Extract(PartYear, field) -> EXTRACT(YEAR FROM "field")
func Extract(part types.DatePart, field types.Field) types.FieldExpression {
	return types.FieldExpression{
		Date: &types.DateExpression{
			Function: types.DateExtract,
			Part:     part,
			Field:    &field,
		},
	}
}

// DateTrunc creates a DATE_TRUNC expression to truncate to specified precision.
// Example: DateTrunc(PartMonth, field) -> DATE_TRUNC('month', "field")
func DateTrunc(part types.DatePart, field types.Field) types.FieldExpression {
	return types.FieldExpression{
		Date: &types.DateExpression{
			Function: types.DateTrunc,
			Part:     part,
			Field:    &field,
		},
	}
}

// Window functions

// WindowBuilder provides a fluent API for building window function expressions.
type WindowBuilder struct {
	expr *types.WindowExpression
}

// Window creates a new WindowSpec for use with window functions.
func Window() *WindowSpecBuilder {
	return &WindowSpecBuilder{
		spec: &types.WindowSpec{},
	}
}

// WindowSpecBuilder provides a fluent API for building window specifications.
type WindowSpecBuilder struct {
	spec *types.WindowSpec
}

// PartitionBy adds PARTITION BY fields to the window specification.
func (wsb *WindowSpecBuilder) PartitionBy(fields ...types.Field) *WindowSpecBuilder {
	wsb.spec.PartitionBy = fields
	return wsb
}

// OrderBy adds ORDER BY to the window specification.
func (wsb *WindowSpecBuilder) OrderBy(field types.Field, direction types.Direction) *WindowSpecBuilder {
	wsb.spec.OrderBy = append(wsb.spec.OrderBy, types.OrderBy{
		Field:     field,
		Direction: direction,
	})
	return wsb
}

// OrderByNulls adds ORDER BY with NULLS ordering to the window specification.
func (wsb *WindowSpecBuilder) OrderByNulls(field types.Field, direction types.Direction, nulls types.NullsOrdering) *WindowSpecBuilder {
	wsb.spec.OrderBy = append(wsb.spec.OrderBy, types.OrderBy{
		Field:     field,
		Direction: direction,
		Nulls:     nulls,
	})
	return wsb
}

// Rows sets the frame clause with ROWS BETWEEN.
func (wsb *WindowSpecBuilder) Rows(start, end types.FrameBound) *WindowSpecBuilder {
	wsb.spec.FrameStart = start
	wsb.spec.FrameEnd = end
	return wsb
}

// Build returns the WindowSpec.
func (wsb *WindowSpecBuilder) Build() types.WindowSpec {
	return *wsb.spec
}

// RowNumber creates a ROW_NUMBER() window function.
func RowNumber() *WindowBuilder {
	return &WindowBuilder{
		expr: &types.WindowExpression{
			Function: types.WinRowNumber,
		},
	}
}

// Rank creates a RANK() window function.
func Rank() *WindowBuilder {
	return &WindowBuilder{
		expr: &types.WindowExpression{
			Function: types.WinRank,
		},
	}
}

// DenseRank creates a DENSE_RANK() window function.
func DenseRank() *WindowBuilder {
	return &WindowBuilder{
		expr: &types.WindowExpression{
			Function: types.WinDenseRank,
		},
	}
}

// Ntile creates an NTILE(n) window function.
func Ntile(n types.Param) *WindowBuilder {
	return &WindowBuilder{
		expr: &types.WindowExpression{
			Function:   types.WinNtile,
			NtileParam: &n,
		},
	}
}

// Lag creates a LAG(field, offset, default) window function.
func Lag(field types.Field, offset types.Param, defaultVal ...types.Param) *WindowBuilder {
	expr := &types.WindowExpression{
		Function:  types.WinLag,
		Field:     &field,
		LagOffset: &offset,
	}
	if len(defaultVal) > 0 {
		expr.LagDefault = &defaultVal[0]
	}
	return &WindowBuilder{expr: expr}
}

// Lead creates a LEAD(field, offset, default) window function.
func Lead(field types.Field, offset types.Param, defaultVal ...types.Param) *WindowBuilder {
	expr := &types.WindowExpression{
		Function:  types.WinLead,
		Field:     &field,
		LagOffset: &offset,
	}
	if len(defaultVal) > 0 {
		expr.LagDefault = &defaultVal[0]
	}
	return &WindowBuilder{expr: expr}
}

// FirstValue creates a FIRST_VALUE(field) window function.
func FirstValue(field types.Field) *WindowBuilder {
	return &WindowBuilder{
		expr: &types.WindowExpression{
			Function: types.WinFirstValue,
			Field:    &field,
		},
	}
}

// LastValue creates a LAST_VALUE(field) window function.
func LastValue(field types.Field) *WindowBuilder {
	return &WindowBuilder{
		expr: &types.WindowExpression{
			Function: types.WinLastValue,
			Field:    &field,
		},
	}
}

// SumOver creates a SUM() OVER window function.
func SumOver(field types.Field) *WindowBuilder {
	return &WindowBuilder{
		expr: &types.WindowExpression{
			Aggregate: types.AggSum,
			Field:     &field,
		},
	}
}

// AvgOver creates an AVG() OVER window function.
func AvgOver(field types.Field) *WindowBuilder {
	return &WindowBuilder{
		expr: &types.WindowExpression{
			Aggregate: types.AggAvg,
			Field:     &field,
		},
	}
}

// CountOver creates a COUNT(field) OVER window function.
func CountOver(field ...types.Field) *WindowBuilder {
	expr := &types.WindowExpression{
		Aggregate: types.AggCountField,
	}
	if len(field) > 0 {
		expr.Field = &field[0]
	}
	return &WindowBuilder{expr: expr}
}

// MinOver creates a MIN() OVER window function.
func MinOver(field types.Field) *WindowBuilder {
	return &WindowBuilder{
		expr: &types.WindowExpression{
			Aggregate: types.AggMin,
			Field:     &field,
		},
	}
}

// MaxOver creates a MAX() OVER window function.
func MaxOver(field types.Field) *WindowBuilder {
	return &WindowBuilder{
		expr: &types.WindowExpression{
			Aggregate: types.AggMax,
			Field:     &field,
		},
	}
}

// Over sets the window specification for a window function.
func (wb *WindowBuilder) Over(spec types.WindowSpec) *WindowBuilder {
	wb.expr.Window = spec
	return wb
}

// OverBuilder sets the window specification from a builder.
func (wb *WindowBuilder) OverBuilder(builder *WindowSpecBuilder) *WindowBuilder {
	wb.expr.Window = *builder.spec
	return wb
}

// PartitionBy adds PARTITION BY fields (convenience method).
func (wb *WindowBuilder) PartitionBy(fields ...types.Field) *WindowBuilder {
	wb.expr.Window.PartitionBy = fields
	return wb
}

// OrderBy adds ORDER BY to the window specification (convenience method).
func (wb *WindowBuilder) OrderBy(field types.Field, direction types.Direction) *WindowBuilder {
	wb.expr.Window.OrderBy = append(wb.expr.Window.OrderBy, types.OrderBy{
		Field:     field,
		Direction: direction,
	})
	return wb
}

// Frame sets the frame clause with ROWS BETWEEN (convenience method).
func (wb *WindowBuilder) Frame(start, end types.FrameBound) *WindowBuilder {
	wb.expr.Window.FrameStart = start
	wb.expr.Window.FrameEnd = end
	return wb
}

// As adds an alias to the window function and returns a FieldExpression.
func (wb *WindowBuilder) As(alias string) types.FieldExpression {
	if !isValidSQLIdentifier(alias) {
		panic(fmt.Errorf("invalid alias '%s': must be alphanumeric/underscore, start with letter/underscore, and contain no SQL keywords", alias))
	}
	return types.FieldExpression{
		Window: wb.expr,
		Alias:  alias,
	}
}

// Build returns the FieldExpression without an alias.
func (wb *WindowBuilder) Build() types.FieldExpression {
	return types.FieldExpression{
		Window: wb.expr,
	}
}

// As adds an alias to a field expression.
func As(expr types.FieldExpression, alias string) types.FieldExpression {
	if !isValidSQLIdentifier(alias) {
		panic(fmt.Errorf("invalid alias '%s': must be alphanumeric/underscore, start with letter/underscore, and contain no SQL keywords", alias))
	}
	expr.Alias = alias
	return expr
}

// Aggregate HAVING condition helpers.
// These create AggregateCondition for use with Builder.HavingAgg().

// Example: HavingCount(GT, param) -> HAVING COUNT(*) > :param.
func HavingCount(op types.Operator, value types.Param) types.AggregateCondition {
	return types.AggregateCondition{
		Func:     types.AggCountField,
		Field:    nil, // COUNT(*)
		Operator: op,
		Value:    value,
	}
}

// Example: HavingCountField(field, GT, param) -> HAVING COUNT("field") > :param.
func HavingCountField(field types.Field, op types.Operator, value types.Param) types.AggregateCondition {
	return types.AggregateCondition{
		Func:     types.AggCountField,
		Field:    &field,
		Operator: op,
		Value:    value,
	}
}

// HavingCountDistinct creates a HAVING COUNT(DISTINCT field) condition.
func HavingCountDistinct(field types.Field, op types.Operator, value types.Param) types.AggregateCondition {
	return types.AggregateCondition{
		Func:     types.AggCountDistinct,
		Field:    &field,
		Operator: op,
		Value:    value,
	}
}

// Example: HavingSum(field, GE, param) -> HAVING SUM("field") >= :param.
func HavingSum(field types.Field, op types.Operator, value types.Param) types.AggregateCondition {
	return types.AggregateCondition{
		Func:     types.AggSum,
		Field:    &field,
		Operator: op,
		Value:    value,
	}
}

// Example: HavingAvg(field, LT, param) -> HAVING AVG("field") < :param.
func HavingAvg(field types.Field, op types.Operator, value types.Param) types.AggregateCondition {
	return types.AggregateCondition{
		Func:     types.AggAvg,
		Field:    &field,
		Operator: op,
		Value:    value,
	}
}

// HavingMin creates a HAVING MIN(field) condition.
func HavingMin(field types.Field, op types.Operator, value types.Param) types.AggregateCondition {
	return types.AggregateCondition{
		Func:     types.AggMin,
		Field:    &field,
		Operator: op,
		Value:    value,
	}
}

// HavingMax creates a HAVING MAX(field) condition.
func HavingMax(field types.Field, op types.Operator, value types.Param) types.AggregateCondition {
	return types.AggregateCondition{
		Func:     types.AggMax,
		Field:    &field,
		Operator: op,
		Value:    value,
	}
}
