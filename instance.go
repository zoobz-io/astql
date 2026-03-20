package astql

import (
	"fmt"
	"strings"

	"github.com/zoobz-io/astql/internal/types"
	"github.com/zoobz-io/dbml"
)

// ASTQL represents an instance of the query builder with a specific DBML schema.
type ASTQL struct {
	project *dbml.Project
	// Internal indexes for fast validation
	tables map[string]*dbml.Table
	fields map[string]map[string]*dbml.Column // table -> field -> column
}

// NewFromDBML creates a new ASTQL instance from a DBML project.
func NewFromDBML(project *dbml.Project) (*ASTQL, error) {
	if project == nil {
		return nil, fmt.Errorf("project cannot be nil")
	}

	a := &ASTQL{
		project: project,
		tables:  make(map[string]*dbml.Table),
		fields:  make(map[string]map[string]*dbml.Column),
	}

	// Build indexes for fast validation
	for _, table := range project.Tables {
		a.tables[table.Name] = table
		a.fields[table.Name] = make(map[string]*dbml.Column)
		for _, col := range table.Columns {
			a.fields[table.Name][col.Name] = col
		}
	}

	return a, nil
}

// validateTable checks if a table exists in the schema.
func (a *ASTQL) validateTable(name string) error {
	if _, ok := a.tables[name]; !ok {
		return fmt.Errorf("table '%s' not found in schema", name)
	}
	return nil
}

// validateField checks if a field exists in any table in the schema.
func (a *ASTQL) validateField(field string) error {
	// Handle SQL expressions with AS aliases like "c.name AS customer_name"
	if asIndex := findAS(field); asIndex != -1 {
		field = field[:asIndex]
	}

	// Handle table aliases like "o.id" by extracting just the field name
	fieldName := field
	if dotIndex := lastDotIndex(field); dotIndex != -1 {
		fieldName = field[dotIndex+1:]
	}

	// Check if field exists in any table
	for _, tableFields := range a.fields {
		if _, ok := tableFields[fieldName]; ok {
			return nil // Found it
		}
	}

	return fmt.Errorf("field '%s' not found in schema", field)
}

// validateTableOrAlias validates both table names and aliases.
func (a *ASTQL) validateTableOrAlias(tableOrAlias string) error {
	// Must be either:
	// 1. A single lowercase letter (table alias), OR
	// 2. A registered table name
	if isValidTableAlias(tableOrAlias) {
		// It's a valid single-letter alias
		return nil
	}
	if err := a.validateTable(tableOrAlias); err == nil {
		// It's a valid table name
		return nil
	}
	return fmt.Errorf("WithTable requires single-letter alias (a-z) or valid table name, got: %s", tableOrAlias)
}

// isValidTableAlias checks if a string is a valid single-letter table alias.
func isValidTableAlias(alias string) bool {
	return len(alias) == 1 && alias[0] >= 'a' && alias[0] <= 'z'
}

// findAS finds the position of " AS " in a string.
func findAS(s string) int {
	for i := 0; i < len(s)-3; i++ {
		if s[i] == ' ' && s[i+1] == 'A' && s[i+2] == 'S' && s[i+3] == ' ' {
			return i
		}
	}
	return -1
}

// lastDotIndex finds the last dot in a string.
func lastDotIndex(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '.' {
			return i
		}
	}
	return -1
}

// isValidSQLIdentifier checks if a string is a valid SQL identifier.
func isValidSQLIdentifier(s string) bool {
	if s == "" {
		return false
	}

	// Must start with letter or underscore
	first := s[0]
	if !((first >= 'a' && first <= 'z') || //nolint:staticcheck // readability over De Morgan
		(first >= 'A' && first <= 'Z') ||
		first == '_') {
		return false
	}

	// Rest must be alphanumeric or underscore
	for i := 1; i < len(s); i++ {
		ch := s[i]
		if !((ch >= 'a' && ch <= 'z') || //nolint:staticcheck // readability over De Morgan
			(ch >= 'A' && ch <= 'Z') ||
			(ch >= '0' && ch <= '9') ||
			ch == '_') {
			return false
		}
	}

	// Check for SQL injection patterns
	lower := strings.ToLower(s)

	suspiciousPatterns := []string{
		";", "--", "/*", "*/", "'", "\"", "`", "\\",
		" or ", " and ", "drop table", "delete from",
		"insert into", "update set", "select ",
		"union all", "union select",
	}

	for _, pattern := range suspiciousPatterns {
		if strings.Contains(lower, pattern) {
			return false
		}
	}

	// Reject if it contains spaces
	if strings.Contains(s, " ") {
		return false
	}

	return true
}

// TryF creates a validated field reference, returning an error if invalid.
func (a *ASTQL) TryF(name string) (types.Field, error) {
	if err := a.validateField(name); err != nil {
		return types.Field{}, fmt.Errorf("invalid field: %w", err)
	}
	return types.Field{Name: name}, nil
}

// F creates a validated field reference.
func (a *ASTQL) F(name string) types.Field {
	f, err := a.TryF(name)
	if err != nil {
		panic(err)
	}
	return f
}

// TryT creates a validated table reference, returning an error if invalid.
func (a *ASTQL) TryT(name string, alias ...string) (types.Table, error) {
	if err := a.validateTable(name); err != nil {
		return types.Table{}, fmt.Errorf("invalid table: %w", err)
	}

	var tableAlias string
	if len(alias) > 0 {
		if len(alias) > 1 {
			return types.Table{}, fmt.Errorf("only one alias allowed")
		}
		tableAlias = alias[0]
		if !isValidTableAlias(tableAlias) {
			return types.Table{}, fmt.Errorf("alias must be single lowercase letter (a-z), got: %s", tableAlias)
		}
	}

	return types.Table{Name: name, Alias: tableAlias}, nil
}

// T creates a validated table reference.
func (a *ASTQL) T(name string, alias ...string) types.Table {
	t, err := a.TryT(name, alias...)
	if err != nil {
		panic(err)
	}
	return t
}

// TryP creates a validated parameter reference, returning an error if invalid.
func (*ASTQL) TryP(name string) (types.Param, error) {
	if !isValidSQLIdentifier(name) {
		return types.Param{}, fmt.Errorf("invalid parameter name: %s", name)
	}
	return types.Param{Name: name}, nil
}

// P creates a validated parameter reference.
func (a *ASTQL) P(name string) types.Param {
	p, err := a.TryP(name)
	if err != nil {
		panic(err)
	}
	return p
}

// TryC creates a validated condition, returning an error if invalid.
func (a *ASTQL) TryC(field types.Field, op types.Operator, param types.Param) (types.Condition, error) {
	// Validate field exists
	if err := a.validateField(field.Name); err != nil {
		return types.Condition{}, err
	}
	return types.Condition{
		Field:    field,
		Operator: op,
		Value:    param,
	}, nil
}

// C creates a validated condition.
func (a *ASTQL) C(field types.Field, op types.Operator, param types.Param) types.Condition {
	c, err := a.TryC(field, op, param)
	if err != nil {
		panic(err)
	}
	return c
}

// TryAggC creates a validated aggregate condition for HAVING clauses.
// Use nil for field to create COUNT(*).
func (a *ASTQL) TryAggC(aggFunc types.AggregateFunc, field *types.Field, op types.Operator, param types.Param) (types.AggregateCondition, error) {
	// Validate field if provided
	if field != nil {
		if err := a.validateField(field.Name); err != nil {
			return types.AggregateCondition{}, err
		}
	}
	return types.AggregateCondition{
		Func:     aggFunc,
		Field:    field,
		Operator: op,
		Value:    param,
	}, nil
}

// AggC creates a validated aggregate condition for HAVING clauses.
// Use nil for field to create COUNT(*).
func (a *ASTQL) AggC(aggFunc types.AggregateFunc, field *types.Field, op types.Operator, param types.Param) types.AggregateCondition {
	c, err := a.TryAggC(aggFunc, field, op, param)
	if err != nil {
		panic(err)
	}
	return c
}

// TryNull creates a NULL condition, returning an error if invalid.
func (a *ASTQL) TryNull(field types.Field) (types.Condition, error) {
	if err := a.validateField(field.Name); err != nil {
		return types.Condition{}, err
	}
	return types.Condition{
		Field:    field,
		Operator: types.IsNull,
	}, nil
}

// Null creates a NULL condition.
func (a *ASTQL) Null(field types.Field) types.Condition {
	c, err := a.TryNull(field)
	if err != nil {
		panic(err)
	}
	return c
}

// TryNotNull creates a NOT NULL condition, returning an error if invalid.
func (a *ASTQL) TryNotNull(field types.Field) (types.Condition, error) {
	if err := a.validateField(field.Name); err != nil {
		return types.Condition{}, err
	}
	return types.Condition{
		Field:    field,
		Operator: types.IsNotNull,
	}, nil
}

// NotNull creates a NOT NULL condition.
func (a *ASTQL) NotNull(field types.Field) types.Condition {
	c, err := a.TryNotNull(field)
	if err != nil {
		panic(err)
	}
	return c
}

// TryAnd creates an AND condition group, returning an error if invalid.
func (*ASTQL) TryAnd(conditions ...types.ConditionItem) (types.ConditionGroup, error) {
	if len(conditions) == 0 {
		return types.ConditionGroup{}, fmt.Errorf("AND requires at least one condition")
	}
	return types.ConditionGroup{
		Logic:      types.AND,
		Conditions: conditions,
	}, nil
}

// And creates an AND condition group.
func (a *ASTQL) And(conditions ...types.ConditionItem) types.ConditionGroup {
	g, err := a.TryAnd(conditions...)
	if err != nil {
		panic(err)
	}
	return g
}

// TryOr creates an OR condition group, returning an error if invalid.
func (*ASTQL) TryOr(conditions ...types.ConditionItem) (types.ConditionGroup, error) {
	if len(conditions) == 0 {
		return types.ConditionGroup{}, fmt.Errorf("OR requires at least one condition")
	}
	return types.ConditionGroup{
		Logic:      types.OR,
		Conditions: conditions,
	}, nil
}

// Or creates an OR condition group.
func (a *ASTQL) Or(conditions ...types.ConditionItem) types.ConditionGroup {
	g, err := a.TryOr(conditions...)
	if err != nil {
		panic(err)
	}
	return g
}

// WithTable creates a new Field with a table/alias prefix, validated against the schema.
func (a *ASTQL) WithTable(field types.Field, tableOrAlias string) types.Field {
	if err := a.validateTableOrAlias(tableOrAlias); err != nil {
		panic(err)
	}
	return types.Field{
		Name:  field.Name,
		Table: tableOrAlias,
	}
}

// TryWithTable creates a new Field with a table/alias prefix, returning an error if invalid.
func (a *ASTQL) TryWithTable(field types.Field, tableOrAlias string) (types.Field, error) {
	if err := a.validateTableOrAlias(tableOrAlias); err != nil {
		return types.Field{}, err
	}
	return types.Field{
		Name:  field.Name,
		Table: tableOrAlias,
	}, nil
}

// GetInstance returns the ASTQL instance (for use by provider packages).
func (a *ASTQL) GetInstance() *ASTQL {
	return a
}

// Fields returns an empty slice of Field for programmatic query building.
func (*ASTQL) Fields() []types.Field {
	return []types.Field{}
}

// Conditions returns an empty slice of Condition for programmatic query building.
func (*ASTQL) Conditions() []types.Condition {
	return []types.Condition{}
}

// ConditionItems returns an empty slice of ConditionItem for programmatic query building.
func (*ASTQL) ConditionItems() []types.ConditionItem {
	return []types.ConditionItem{}
}

// Params returns an empty slice of Param for programmatic query building.
func (*ASTQL) Params() []types.Param {
	return []types.Param{}
}

// ValueMap returns an empty map for programmatic INSERT value building.
func (*ASTQL) ValueMap() map[types.Field]types.Param {
	return make(map[types.Field]types.Param)
}

// Operation constants.

// OpSelect returns the Select operation constant.
func (*ASTQL) OpSelect() types.Operation {
	return types.OpSelect
}

// OpInsert returns the Insert operation constant.
func (*ASTQL) OpInsert() types.Operation {
	return types.OpInsert
}

// OpUpdate returns the Update operation constant.
func (*ASTQL) OpUpdate() types.Operation {
	return types.OpUpdate
}

// OpDelete returns the Delete operation constant.
func (*ASTQL) OpDelete() types.Operation {
	return types.OpDelete
}

// OpCount returns the Count operation constant.
func (*ASTQL) OpCount() types.Operation {
	return types.OpCount
}

// Direction constants.

// ASC returns the ascending sort direction constant.
func (*ASTQL) ASC() types.Direction {
	return types.ASC
}

// DESC returns the descending sort direction constant.
func (*ASTQL) DESC() types.Direction {
	return types.DESC
}

// Operator constants.

// EQ returns the equals operator constant.
func (*ASTQL) EQ() types.Operator {
	return types.EQ
}

// NE returns the not equals operator constant.
func (*ASTQL) NE() types.Operator {
	return types.NE
}

// GT returns the greater than operator constant.
func (*ASTQL) GT() types.Operator {
	return types.GT
}

// GE returns the greater than or equal operator constant.
func (*ASTQL) GE() types.Operator {
	return types.GE
}

// LT returns the less than operator constant.
func (*ASTQL) LT() types.Operator {
	return types.LT
}

// LE returns the less than or equal operator constant.
func (*ASTQL) LE() types.Operator {
	return types.LE
}

// IN returns the IN operator constant.
func (*ASTQL) IN() types.Operator {
	return types.IN
}

// NotIn returns the NOT IN operator constant.
func (*ASTQL) NotIn() types.Operator {
	return types.NotIn
}

// LIKE returns the LIKE operator constant.
func (*ASTQL) LIKE() types.Operator {
	return types.LIKE
}

// NotLike returns the NOT LIKE operator constant.
func (*ASTQL) NotLike() types.Operator {
	return types.NotLike
}

// IsNull returns the IS NULL operator constant.
func (*ASTQL) IsNull() types.Operator {
	return types.IsNull
}

// IsNotNull returns the IS NOT NULL operator constant.
func (*ASTQL) IsNotNull() types.Operator {
	return types.IsNotNull
}

// EXISTS returns the EXISTS operator constant.
func (*ASTQL) EXISTS() types.Operator {
	return types.EXISTS
}

// NotExists returns the NOT EXISTS operator constant.
func (*ASTQL) NotExists() types.Operator {
	return types.NotExists
}

// VectorL2Distance returns the vector L2/Euclidean distance operator constant (pgvector <->).
func (*ASTQL) VectorL2Distance() types.Operator {
	return types.VectorL2Distance
}

// VectorInnerProduct returns the vector negative inner product operator constant (pgvector <#>).
func (*ASTQL) VectorInnerProduct() types.Operator {
	return types.VectorInnerProduct
}

// VectorCosineDistance returns the vector cosine distance operator constant (pgvector <=>).
func (*ASTQL) VectorCosineDistance() types.Operator {
	return types.VectorCosineDistance
}

// VectorL1Distance returns the vector L1/Manhattan distance operator constant (pgvector <+>).
func (*ASTQL) VectorL1Distance() types.Operator {
	return types.VectorL1Distance
}

// ILIKE returns the case-insensitive LIKE operator constant (PostgreSQL).
func (*ASTQL) ILIKE() types.Operator {
	return types.ILIKE
}

// NotILike returns the NOT ILIKE operator constant (PostgreSQL).
func (*ASTQL) NotILike() types.Operator {
	return types.NotILike
}

// RegexMatch returns the regex match operator constant (PostgreSQL ~).
func (*ASTQL) RegexMatch() types.Operator {
	return types.RegexMatch
}

// RegexIMatch returns the case-insensitive regex match operator constant (PostgreSQL ~*).
func (*ASTQL) RegexIMatch() types.Operator {
	return types.RegexIMatch
}

// NotRegexMatch returns the regex non-match operator constant (PostgreSQL !~).
func (*ASTQL) NotRegexMatch() types.Operator {
	return types.NotRegexMatch
}

// NotRegexIMatch returns the case-insensitive regex non-match operator constant (PostgreSQL !~*).
func (*ASTQL) NotRegexIMatch() types.Operator {
	return types.NotRegexIMatch
}

// ArrayContains returns the array contains operator constant (PostgreSQL @>).
func (*ASTQL) ArrayContains() types.Operator {
	return types.ArrayContains
}

// ArrayContainedBy returns the array contained by operator constant (PostgreSQL <@).
func (*ASTQL) ArrayContainedBy() types.Operator {
	return types.ArrayContainedBy
}

// ArrayOverlap returns the array overlap operator constant (PostgreSQL &&).
func (*ASTQL) ArrayOverlap() types.Operator {
	return types.ArrayOverlap
}

// JSONBText creates a field with JSONB text extraction (->>).
// The key is parameterized for safety - renders as: field->>:key_param
// Example: JSONBText(metadata, P("status_key")) -> "metadata"->>:status_key
func (a *ASTQL) JSONBText(field types.Field, key types.Param) types.Field {
	return types.Field{
		Name:         field.Name,
		Table:        field.Table,
		JSONBTextKey: &key,
	}
}

// JSONBPath creates a field with JSONB path access (->).
// The key is parameterized for safety - renders as: field->:key_param
// Example: JSONBPath(metadata, P("tags_key")) -> "metadata"->:tags_key
func (a *ASTQL) JSONBPath(field types.Field, key types.Param) types.Field {
	return types.Field{
		Name:         field.Name,
		Table:        field.Table,
		JSONBPathKey: &key,
	}
}
