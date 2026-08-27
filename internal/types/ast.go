package types

import (
	"fmt"
	"sort"
	"strings"
)

// Operation represents the type of query operation.
type Operation string

const (
	OpSelect Operation = "SELECT"
	OpInsert Operation = "INSERT"
	OpUpdate Operation = "UPDATE"
	OpDelete Operation = "DELETE"
	OpCount  Operation = "COUNT"
)

// Direction represents sort direction.
type Direction string

const (
	ASC  Direction = "ASC"
	DESC Direction = "DESC"
)

// NullsOrdering represents NULL ordering in ORDER BY.
type NullsOrdering string

const (
	NullsFirst NullsOrdering = "NULLS FIRST"
	NullsLast  NullsOrdering = "NULLS LAST"
)

// OrderBy represents an ORDER BY clause.
type OrderBy struct {
	Field     Field
	Direction Direction
	Nulls     NullsOrdering // Optional NULLS FIRST/LAST
	// Expression fields for ORDER BY field <op> param (e.g., vector distance).
	// When Operator is set, renders as: field <op> param [direction]
	Operator Operator
	Param    Param
}

// PaginationValue represents a LIMIT or OFFSET value that can be
// either a static integer or a parameterized value.
type PaginationValue struct {
	Static *int   // Static integer value
	Param  *Param // Parameterized value (takes precedence if set)
}

// LockMode represents row-level locking modes (PostgreSQL).
type LockMode string

const (
	LockForUpdate      LockMode = "FOR UPDATE"
	LockForNoKeyUpdate LockMode = "FOR NO KEY UPDATE"
	LockForShare       LockMode = "FOR SHARE"
	LockForKeyShare    LockMode = "FOR KEY SHARE"
)

// LockWait represents the wait policy applied to a row-level lock,
// controlling what happens when a target row is already locked.
type LockWait string

const (
	LockWaitNone       LockWait = ""            // Wait for the lock (default)
	LockWaitSkipLocked LockWait = "SKIP LOCKED" // Skip rows that are already locked
	LockWaitNoWait     LockWait = "NOWAIT"      // Error immediately if a row is locked
)

// JoinType represents the type of SQL join.
type JoinType string

const (
	InnerJoin     JoinType = "INNER JOIN"
	LeftJoin      JoinType = "LEFT JOIN"
	RightJoin     JoinType = "RIGHT JOIN"
	FullOuterJoin JoinType = "FULL OUTER JOIN"
	CrossJoin     JoinType = "CROSS JOIN"
)

// Join represents a SQL JOIN clause.
type Join struct {
	On    ConditionItem
	Table Table
	Type  JoinType
}

// ConflictAction represents what to do on conflict.
type ConflictAction string

const (
	DoNothing ConflictAction = "DO NOTHING"
	DoUpdate  ConflictAction = "DO UPDATE"
)

// ConflictClause represents PostgreSQL's ON CONFLICT clause.
type ConflictClause struct {
	Updates map[Field]Param
	Action  ConflictAction
	Columns []Field
}

// AggregateFunc represents SQL aggregate functions.
type AggregateFunc string

const (
	AggSum AggregateFunc = "SUM"
	AggAvg AggregateFunc = "AVG"
	AggMin AggregateFunc = "MIN"
	AggMax AggregateFunc = "MAX"
	// Note: COUNT is already an operation, but can also be an aggregate on fields.
	AggCountField    AggregateFunc = "COUNT"
	AggCountDistinct AggregateFunc = "COUNT_DISTINCT"
)

// FieldExpression represents a field with optional aggregate function or SQL expression.
type FieldExpression struct {
	Field     Field
	Aggregate AggregateFunc
	Filter    ConditionItem       // For FILTER clause on aggregates
	Case      *CaseExpression     // For CASE expressions in SELECT
	Coalesce  *CoalesceExpression // For COALESCE expressions
	NullIf    *NullIfExpression   // For NULLIF expressions
	Math      *MathExpression     // For math functions
	String    *StringExpression   // For string functions
	Date      *DateExpression     // For date/time functions
	Cast      *CastExpression     // For type casting
	Window    *WindowExpression   // For window functions
	Binary    *BinaryExpression   // For field <op> param expressions (e.g., vector distance)
	Alias     string
}

// BinaryExpression represents a binary operation between a field and a parameter.
// Used for expressions like vector distance calculations: field <-> :param
type BinaryExpression struct {
	Field    Field
	Operator Operator
	Param    Param
}

// CaseExpression represents a SQL CASE expression.
type CaseExpression struct {
	ElseValue   *Param
	Alias       string
	WhenClauses []WhenClause
}

// WhenClause represents a single WHEN...THEN clause.
type WhenClause struct {
	Condition ConditionItem
	Result    Param
}

// CoalesceExpression represents a COALESCE function call.
type CoalesceExpression struct {
	Alias  string
	Values []Param
}

// NullIfExpression represents a NULLIF function call.
type NullIfExpression struct {
	Alias  string
	Value1 Param
	Value2 Param
}

// MathFunc represents SQL math functions.
type MathFunc string

const (
	MathRound MathFunc = "ROUND"
	MathFloor MathFunc = "FLOOR"
	MathCeil  MathFunc = "CEIL"
	MathAbs   MathFunc = "ABS"
	MathPower MathFunc = "POWER"
	MathSqrt  MathFunc = "SQRT"
)

// MathExpression represents a math function call.
type MathExpression struct {
	Function  MathFunc
	Field     Field
	Precision *Param // Optional, for ROUND
	Exponent  *Param // Optional, for POWER
	Alias     string
}

// StringFunc represents SQL string functions.
type StringFunc string

const (
	StringUpper     StringFunc = "UPPER"
	StringLower     StringFunc = "LOWER"
	StringTrim      StringFunc = "TRIM"
	StringLTrim     StringFunc = "LTRIM"
	StringRTrim     StringFunc = "RTRIM"
	StringLength    StringFunc = "LENGTH"
	StringSubstring StringFunc = "SUBSTRING"
	StringReplace   StringFunc = "REPLACE"
	StringConcat    StringFunc = "CONCAT"
)

// StringExpression represents a string function call.
type StringExpression struct {
	Function StringFunc
	Field    Field   // Primary field/column
	Args     []Param // Additional arguments (for SUBSTRING, REPLACE, CONCAT)
	Fields   []Field // Additional fields (for CONCAT with multiple fields)
	Alias    string
}

// DateFunc represents SQL date/time functions.
type DateFunc string

const (
	DateNow              DateFunc = "NOW"
	DateCurrentDate      DateFunc = "CURRENT_DATE"
	DateCurrentTime      DateFunc = "CURRENT_TIME"
	DateCurrentTimestamp DateFunc = "CURRENT_TIMESTAMP"
	DateExtract          DateFunc = "EXTRACT"
	DateTrunc            DateFunc = "DATE_TRUNC"
)

// DatePart represents date/time parts for EXTRACT and DATE_TRUNC.
type DatePart string

const (
	PartYear      DatePart = "YEAR"
	PartMonth     DatePart = "MONTH"
	PartDay       DatePart = "DAY"
	PartHour      DatePart = "HOUR"
	PartMinute    DatePart = "MINUTE"
	PartSecond    DatePart = "SECOND"
	PartWeek      DatePart = "WEEK"
	PartQuarter   DatePart = "QUARTER"
	PartDayOfWeek DatePart = "DOW"
	PartDayOfYear DatePart = "DOY"
	PartEpoch     DatePart = "EPOCH"
)

// DateExpression represents a date/time function call.
type DateExpression struct {
	Function DateFunc
	Field    *Field   // Optional - not needed for NOW, CURRENT_DATE, etc.
	Part     DatePart // For EXTRACT and DATE_TRUNC
	Alias    string
}

// CastType represents allowed PostgreSQL data types for casting.
type CastType string

const (
	CastText            CastType = "TEXT"
	CastInteger         CastType = "INTEGER"
	CastBigint          CastType = "BIGINT"
	CastSmallint        CastType = "SMALLINT"
	CastNumeric         CastType = "NUMERIC"
	CastReal            CastType = "REAL"
	CastDoublePrecision CastType = "DOUBLE PRECISION"
	CastBoolean         CastType = "BOOLEAN"
	CastDate            CastType = "DATE"
	CastTime            CastType = "TIME"
	CastTimestamp       CastType = "TIMESTAMP"
	CastTimestampTZ     CastType = "TIMESTAMPTZ"
	CastInterval        CastType = "INTERVAL"
	CastUUID            CastType = "UUID"
	CastJSON            CastType = "JSON"
	CastJSONB           CastType = "JSONB"
	CastBytea           CastType = "BYTEA"
)

// CastExpression represents a type cast.
type CastExpression struct {
	Field    Field
	CastType CastType
}

// WindowFunc represents window function types.
type WindowFunc string

const (
	WinRowNumber  WindowFunc = "ROW_NUMBER"
	WinRank       WindowFunc = "RANK"
	WinDenseRank  WindowFunc = "DENSE_RANK"
	WinNtile      WindowFunc = "NTILE"
	WinLag        WindowFunc = "LAG"
	WinLead       WindowFunc = "LEAD"
	WinFirstValue WindowFunc = "FIRST_VALUE"
	WinLastValue  WindowFunc = "LAST_VALUE"
)

// FrameBound represents window frame boundaries.
type FrameBound string

const (
	FrameUnboundedPreceding FrameBound = "UNBOUNDED PRECEDING"
	FrameCurrentRow         FrameBound = "CURRENT ROW"
	FrameUnboundedFollowing FrameBound = "UNBOUNDED FOLLOWING"
)

// WindowSpec represents a window specification.
type WindowSpec struct {
	FrameStart  FrameBound
	FrameEnd    FrameBound
	PartitionBy []Field
	OrderBy     []OrderBy
}

// WindowExpression represents a window function call.
type WindowExpression struct {
	Field      *Field
	NtileParam *Param
	LagOffset  *Param
	LagDefault *Param
	Function   WindowFunc
	Aggregate  AggregateFunc
	Window     WindowSpec
}

// SetOperation represents SQL set operations.
type SetOperation string

const (
	SetUnion        SetOperation = "UNION"
	SetUnionAll     SetOperation = "UNION ALL"
	SetIntersect    SetOperation = "INTERSECT"
	SetIntersectAll SetOperation = "INTERSECT ALL"
	SetExcept       SetOperation = "EXCEPT"
	SetExceptAll    SetOperation = "EXCEPT ALL"
)

// SetOperand represents one operand in a set operation.
type SetOperand struct {
	AST       *AST
	Operation SetOperation // Operation to apply BEFORE this AST
}

// CompoundQuery represents a query with set operations.
type CompoundQuery struct {
	Base     *AST
	Limit    *PaginationValue
	Offset   *PaginationValue
	Operands []SetOperand
	Ordering []OrderBy
}

// FieldComparison represents a comparison between two fields.
type FieldComparison struct {
	LeftField  Field
	Operator   Operator
	RightField Field
}

// SubqueryCondition represents a condition that uses a subquery.
type SubqueryCondition struct {
	Subquery Subquery
	Field    *Field
	Operator Operator
}

// Subquery represents a nested query.
type Subquery struct {
	AST *AST
}

// Constants for query complexity limits to prevent DoS attacks.
const (
	MaxSubqueryDepth   = 3   // Prevent DoS via deep nesting
	MaxJoinCount       = 10  // Maximum number of JOINs per query
	MaxConditionDepth  = 5   // Maximum nesting depth of condition groups
	MaxFieldCount      = 100 // Maximum number of fields in SELECT
	MaxWindowFunctions = 10  // Maximum number of window functions per query
	MaxSetOperations   = 5   // Maximum number of UNION/INTERSECT/EXCEPT operations
)

// Implement ConditionItem interface for new condition types.
func (FieldComparison) IsConditionItem()   {}
func (SubqueryCondition) IsConditionItem() {}

// AST represents the abstract syntax tree for PostgreSQL queries.
// This is exported from the internal package so the base package can use it,
// but external users cannot import this package.
//
//nolint:govet // fieldalignment: Logical grouping is preferred over memory optimization
type AST struct {
	WhereClause       ConditionItem
	Lock              *LockMode
	LockWait          LockWait
	OnConflict        *ConflictClause
	Limit             *PaginationValue
	Offset            *PaginationValue
	Updates           map[Field]Param
	UpdateExpressions map[Field]FieldExpression
	Target            Table
	Operation         Operation
	Values            []map[Field]Param
	Ordering          []OrderBy
	Joins             []Join
	GroupBy           []Field
	Having            []ConditionItem
	FieldExpressions  []FieldExpression
	Returning         []Field
	DistinctOn        []Field
	Fields            []Field
	Distinct          bool
}

// Validate performs basic validation on the AST.
func (ast *AST) Validate() error {
	if ast.Target.Name == "" {
		return fmt.Errorf("target table is required")
	}

	// Check complexity limits
	if len(ast.Joins) > MaxJoinCount {
		return fmt.Errorf("too many JOINs: %d (max %d)", len(ast.Joins), MaxJoinCount)
	}

	totalFields := len(ast.Fields) + len(ast.FieldExpressions)
	if totalFields > MaxFieldCount {
		return fmt.Errorf("too many fields: %d (max %d)", totalFields, MaxFieldCount)
	}

	// Count window functions
	windowCount := 0
	for i := range ast.FieldExpressions {
		if ast.FieldExpressions[i].Window != nil {
			windowCount++
		}
	}
	if windowCount > MaxWindowFunctions {
		return fmt.Errorf("too many window functions: %d (max %d)", windowCount, MaxWindowFunctions)
	}

	// Validate condition depth
	if ast.WhereClause != nil {
		if err := validateConditionDepth(ast.WhereClause, 0); err != nil {
			return err
		}
	}

	switch ast.Operation {
	case OpSelect:
		// Fields are optional (defaults to *)
		// Can have JOINs, GROUP BY, HAVING, DISTINCT
	case OpInsert:
		if len(ast.Values) == 0 {
			return fmt.Errorf("INSERT requires at least one value set")
		}
		// Ensure all value sets have the same fields
		if len(ast.Values) > 1 {
			firstKeys := make(map[Field]bool)
			for k := range ast.Values[0] {
				firstKeys[k] = true
			}
			for i, valueSet := range ast.Values[1:] {
				if len(valueSet) != len(firstKeys) {
					return fmt.Errorf("value set %d has different number of fields", i+1)
				}
				for k := range valueSet {
					if !firstKeys[k] {
						return fmt.Errorf("value set %d has different fields", i+1)
					}
				}
			}
		}
		// ON CONFLICT only makes sense with INSERT
		if ast.OnConflict != nil && len(ast.OnConflict.Columns) == 0 {
			return fmt.Errorf("ON CONFLICT requires at least one column")
		}
	case OpUpdate:
		if len(ast.Updates) == 0 && len(ast.UpdateExpressions) == 0 {
			return fmt.Errorf("UPDATE requires at least one field to update")
		}
		// Detect duplicate fields across Updates and UpdateExpressions
		if len(ast.Updates) > 0 && len(ast.UpdateExpressions) > 0 {
			var duplicates []string
			for field := range ast.Updates {
				if _, exists := ast.UpdateExpressions[field]; exists {
					duplicates = append(duplicates, field.Name)
				}
			}
			if len(duplicates) > 0 {
				sort.Strings(duplicates)
				return fmt.Errorf("field(s) %s appear in both Set and SetExpr", strings.Join(duplicates, ", "))
			}
		}
		// UPDATE can have RETURNING but not SELECT features
		if ast.Distinct || len(ast.Joins) > 0 || len(ast.GroupBy) > 0 {
			return fmt.Errorf("UPDATE cannot have SELECT features like DISTINCT, JOIN, or GROUP BY")
		}
	case OpDelete:
		// DELETE can have RETURNING but not SELECT features
		if ast.Distinct || len(ast.Joins) > 0 || len(ast.GroupBy) > 0 {
			return fmt.Errorf("DELETE cannot have SELECT features like DISTINCT, JOIN, or GROUP BY")
		}
	case OpCount:
		// COUNT can have JOINs and WHERE but no fields
		// COUNT can have JOINs
	default:
		return fmt.Errorf("unsupported operation: %s", ast.Operation)
	}

	// HAVING requires GROUP BY
	if len(ast.Having) > 0 && len(ast.GroupBy) == 0 {
		return fmt.Errorf("HAVING requires GROUP BY")
	}

	// DISTINCT ON and DISTINCT are mutually exclusive
	if ast.Distinct && len(ast.DistinctOn) > 0 {
		return fmt.Errorf("cannot use both DISTINCT and DISTINCT ON")
	}

	return nil
}

// validateConditionDepth checks the nesting depth of condition groups.
func validateConditionDepth(cond ConditionItem, depth int) error {
	if depth > MaxConditionDepth {
		return fmt.Errorf("condition nesting too deep: %d (max %d)", depth, MaxConditionDepth)
	}

	switch c := cond.(type) {
	case ConditionGroup:
		for _, subCond := range c.Conditions {
			if err := validateConditionDepth(subCond, depth+1); err != nil {
				return err
			}
		}
	case Condition, FieldComparison, SubqueryCondition, AggregateCondition, BetweenCondition:
		// Leaf nodes, no further depth
	}

	return nil
}
