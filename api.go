// Package astql provides a type-safe SQL query builder with multi-provider support.
//
// The package generates an Abstract Syntax Tree (AST) from fluent builder
// calls, then renders it to SQL with named parameters compatible with sqlx.
// Schema validation is available through DBML integration.
//
// # Basic Usage
//
// Queries can be built directly using the package-level builder functions:
//
//	import "github.com/zoobz-io/astql/postgres"
//
//	query := astql.Select(table).
//		Fields(field1, field2).
//		Where(condition).
//		OrderBy(field1, astql.ASC).
//		Limit(10)
//
//	result, err := query.Render(postgres.New())
//	// result.SQL: SELECT "field1", "field2" FROM "table" WHERE ... ORDER BY "field1" ASC LIMIT 10
//	// result.RequiredParams: []string{"param_name", ...}
//
// # Multi-Provider Support
//
// The package supports multiple SQL dialects through the Renderer interface.
// Available providers: postgres, mariadb, sqlite, mssql.
//
//	import "github.com/zoobz-io/astql/mariadb"
//
//	result, err := query.Render(mariadb.New())
//
// # Schema-Validated Usage
//
// For compile-time safety, create an ASTQL instance from a DBML schema:
//
//	instance, err := astql.NewFromDBML(project)
//	if err != nil {
//		return err
//	}
//
//	// These panic if the field/table doesn't exist in the schema
//	users := instance.T("users")
//	email := instance.F("email")
//
// # Supported Operations
//
// The package supports SELECT, INSERT, UPDATE, DELETE, and COUNT operations,
// along with JOINs, subqueries, window functions, aggregates, CASE expressions,
// set operations (UNION, INTERSECT, EXCEPT), and PostgreSQL-specific features
// like DISTINCT ON, row locking, RETURNING, ON CONFLICT, and pgvector operators.
//
// # Output Format
//
// All queries use named parameters (`:param_name`) for use with sqlx.NamedExec
// and similar functions. Identifiers are quoted to handle reserved words.
package astql

import "github.com/zoobz-io/astql/internal/types"

// AST represents the abstract syntax tree for a query.
// This is re-exported from internal/types for use by consumers.
type AST = types.AST

// QueryResult contains the rendered SQL and required parameters.
type QueryResult = types.QueryResult

// Operation represents the type of query operation.
type Operation = types.Operation

// Re-export operation constants for public API.
const (
	OpSelect = types.OpSelect
	OpInsert = types.OpInsert
	OpUpdate = types.OpUpdate
	OpDelete = types.OpDelete
	OpCount  = types.OpCount
)

// Direction represents sort direction.
type Direction = types.Direction

// Re-export direction constants for public API.
const (
	ASC  = types.ASC
	DESC = types.DESC
)

// NullsOrdering represents NULL ordering in ORDER BY.
type NullsOrdering = types.NullsOrdering

// Re-export nulls ordering constants for public API.
const (
	NullsFirst = types.NullsFirst
	NullsLast  = types.NullsLast
)

// Operator represents SQL comparison operators.
type Operator = types.Operator

// Re-export operator constants for public API.
const (
	// Basic comparison operators.
	EQ = types.EQ
	NE = types.NE
	GT = types.GT
	GE = types.GE
	LT = types.LT
	LE = types.LE

	// Extended operators.
	IN        = types.IN
	NotIn     = types.NotIn
	LIKE      = types.LIKE
	NotLike   = types.NotLike
	ILIKE     = types.ILIKE
	NotILike  = types.NotILike
	IsNull    = types.IsNull
	IsNotNull = types.IsNotNull
	EXISTS    = types.EXISTS
	NotExists = types.NotExists

	// Regex operators (PostgreSQL).
	RegexMatch     = types.RegexMatch
	RegexIMatch    = types.RegexIMatch
	NotRegexMatch  = types.NotRegexMatch
	NotRegexIMatch = types.NotRegexIMatch

	// Array operators (PostgreSQL).
	ArrayContains    = types.ArrayContains
	ArrayContainedBy = types.ArrayContainedBy
	ArrayOverlap     = types.ArrayOverlap

	// Vector operators (pgvector).
	VectorL2Distance     = types.VectorL2Distance
	VectorInnerProduct   = types.VectorInnerProduct
	VectorCosineDistance = types.VectorCosineDistance
	VectorL1Distance     = types.VectorL1Distance
)

// ConditionItem represents either a single condition or a group of conditions.
type ConditionItem = types.ConditionItem

// AggregateCondition represents a HAVING condition on an aggregate function.
// Use with Builder.HavingAgg() for conditions like COUNT(*) > 10.
type AggregateCondition = types.AggregateCondition

// BetweenCondition represents a BETWEEN condition with two bounds.
type BetweenCondition = types.BetweenCondition

// BinaryExpression represents a binary operation between a field and a parameter.
// Used for expressions like vector distance calculations: field <-> :param
type BinaryExpression = types.BinaryExpression

// AggregateFunc represents SQL aggregate functions.
type AggregateFunc = types.AggregateFunc

// Re-export aggregate function constants for public API.
const (
	AggSum           = types.AggSum
	AggAvg           = types.AggAvg
	AggMin           = types.AggMin
	AggMax           = types.AggMax
	AggCountField    = types.AggCountField
	AggCountDistinct = types.AggCountDistinct
)

// CastType represents allowed PostgreSQL data types for casting.
type CastType = types.CastType

// Re-export cast type constants for public API.
const (
	CastText            = types.CastText
	CastInteger         = types.CastInteger
	CastBigint          = types.CastBigint
	CastSmallint        = types.CastSmallint
	CastNumeric         = types.CastNumeric
	CastReal            = types.CastReal
	CastDoublePrecision = types.CastDoublePrecision
	CastBoolean         = types.CastBoolean
	CastDate            = types.CastDate
	CastTime            = types.CastTime
	CastTimestamp       = types.CastTimestamp
	CastTimestampTZ     = types.CastTimestampTZ
	CastInterval        = types.CastInterval
	CastUUID            = types.CastUUID
	CastJSON            = types.CastJSON
	CastJSONB           = types.CastJSONB
	CastBytea           = types.CastBytea
)

// WindowFunc represents window function types.
type WindowFunc = types.WindowFunc

// Re-export window function constants for public API.
const (
	WinRowNumber  = types.WinRowNumber
	WinRank       = types.WinRank
	WinDenseRank  = types.WinDenseRank
	WinNtile      = types.WinNtile
	WinLag        = types.WinLag
	WinLead       = types.WinLead
	WinFirstValue = types.WinFirstValue
	WinLastValue  = types.WinLastValue
)

// FrameBound represents window frame boundaries.
type FrameBound = types.FrameBound

// Re-export frame bound constants for public API.
const (
	FrameUnboundedPreceding = types.FrameUnboundedPreceding
	FrameCurrentRow         = types.FrameCurrentRow
	FrameUnboundedFollowing = types.FrameUnboundedFollowing
)

// WindowSpec represents a window specification.
type WindowSpec = types.WindowSpec

// SetOperation represents SQL set operations (UNION, INTERSECT, EXCEPT).
type SetOperation = types.SetOperation

// Re-export set operation constants for public API.
const (
	SetUnion        = types.SetUnion
	SetUnionAll     = types.SetUnionAll
	SetIntersect    = types.SetIntersect
	SetIntersectAll = types.SetIntersectAll
	SetExcept       = types.SetExcept
	SetExceptAll    = types.SetExceptAll
)

// SetOperand represents one operand in a set operation.
type SetOperand = types.SetOperand

// CompoundQuery represents a query with set operations.
type CompoundQuery = types.CompoundQuery
