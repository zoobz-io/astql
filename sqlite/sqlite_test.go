package sqlite

import (
	"strings"
	"testing"

	"github.com/zoobz-io/astql/internal/render"
	"github.com/zoobz-io/astql/internal/types"
)

func TestNew(t *testing.T) {
	r := New()
	if r == nil {
		t.Fatal("New() returned nil")
	}
}

func TestRender_SimpleSelect(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "users"},
		Fields:    []types.Field{{Name: "id"}, {Name: "name"}},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := `SELECT "id", "name" FROM "users"`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_SelectWithWhere(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "users"},
		Fields:    []types.Field{{Name: "id"}},
		WhereClause: types.Condition{
			Field:    types.Field{Name: "active"},
			Operator: types.EQ,
			Value:    types.Param{Name: "is_active"},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := `SELECT "id" FROM "users" WHERE "active" = :is_active`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_Insert(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpInsert,
		Target:    types.Table{Name: "users"},
		Values: []map[types.Field]types.Param{
			{
				{Name: "name"}:  {Name: "name_val"},
				{Name: "email"}: {Name: "email_val"},
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := `INSERT INTO "users" ("email", "name") VALUES (:email_val, :name_val)`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_Update(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpUpdate,
		Target:    types.Table{Name: "users"},
		Updates: map[types.Field]types.Param{
			{Name: "name"}: {Name: "new_name"},
		},
		WhereClause: types.Condition{
			Field:    types.Field{Name: "id"},
			Operator: types.EQ,
			Value:    types.Param{Name: "user_id"},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := `UPDATE "users" SET "name" = :new_name WHERE "id" = :user_id`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_UpdateSetExpr(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpUpdate,
		Target:    types.Table{Name: "users"},
		UpdateExpressions: map[types.Field]types.FieldExpression{
			{Name: "age"}: {
				Binary: &types.BinaryExpression{
					Field:    types.Field{Name: "age"},
					Operator: "+",
					Param:    types.Param{Name: "increment"},
				},
			},
		},
		WhereClause: types.Condition{
			Field:    types.Field{Name: "id"},
			Operator: types.EQ,
			Value:    types.Param{Name: "user_id"},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := `UPDATE "users" SET "age" = "age" + :increment WHERE "id" = :user_id`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_UpdateSetExpr_JSONB(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpUpdate,
		Target:    types.Table{Name: "users"},
		UpdateExpressions: map[types.Field]types.FieldExpression{
			{Name: "data", JSONBTextKey: &types.Param{Name: "key"}}: {
				Binary: &types.BinaryExpression{
					Field:    types.Field{Name: "data"},
					Operator: "+",
					Param:    types.Param{Name: "val"},
				},
			},
		},
		WhereClause: types.Condition{
			Field:    types.Field{Name: "id"},
			Operator: types.EQ,
			Value:    types.Param{Name: "user_id"},
		},
	}

	_, err := r.Render(ast)
	if err == nil {
		t.Fatal("Expected error for JSONB field in UpdateExpressions")
	}
}

func TestRender_UpdateSetExpr_ExprError(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpUpdate,
		Target:    types.Table{Name: "users"},
		UpdateExpressions: map[types.Field]types.FieldExpression{
			{Name: "age"}: {
				Case: &types.CaseExpression{
					WhenClauses: []types.WhenClause{
						{Condition: nil, Result: types.Param{Name: "val"}},
					},
				},
			},
		},
		WhereClause: types.Condition{
			Field:    types.Field{Name: "id"},
			Operator: types.EQ,
			Value:    types.Param{Name: "user_id"},
		},
	}

	_, err := r.Render(ast)
	if err == nil {
		t.Fatal("Expected error for invalid expression in UpdateExpressions")
	}
}

func TestRender_UpdateSetExpr_ExprJSONB(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpUpdate,
		Target:    types.Table{Name: "users"},
		UpdateExpressions: map[types.Field]types.FieldExpression{
			{Name: "age"}: {
				Binary: &types.BinaryExpression{
					Field:    types.Field{Name: "data", JSONBTextKey: &types.Param{Name: "key"}},
					Operator: "+",
					Param:    types.Param{Name: "val"},
				},
			},
		},
		WhereClause: types.Condition{
			Field:    types.Field{Name: "id"},
			Operator: types.EQ,
			Value:    types.Param{Name: "user_id"},
		},
	}

	_, err := r.Render(ast)
	if err == nil {
		t.Fatal("Expected error for JSONB in expression field")
	}
}

func TestRender_Delete(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpDelete,
		Target:    types.Table{Name: "users"},
		WhereClause: types.Condition{
			Field:    types.Field{Name: "id"},
			Operator: types.EQ,
			Value:    types.Param{Name: "user_id"},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := `DELETE FROM "users" WHERE "id" = :user_id`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

// Test unsupported features are rejected

func TestRender_RejectsDistinctOn(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation:  types.OpSelect,
		Target:     types.Table{Name: "users"},
		Fields:     []types.Field{{Name: "id"}},
		DistinctOn: []types.Field{{Name: "email"}},
	}

	_, err := r.Render(ast)
	if err == nil {
		t.Fatal("expected error for DISTINCT ON, got nil")
	}
	if !strings.Contains(err.Error(), "DISTINCT ON") {
		t.Errorf("error = %q, want to contain 'DISTINCT ON'", err.Error())
	}
}

func TestRender_RejectsForUpdate(t *testing.T) {
	r := New()
	lock := types.LockForUpdate
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "users"},
		Fields:    []types.Field{{Name: "id"}},
		Lock:      &lock,
	}

	_, err := r.Render(ast)
	if err == nil {
		t.Fatal("expected error for FOR UPDATE, got nil")
	}
	if !strings.Contains(err.Error(), "row-level locking") {
		t.Errorf("error = %q, want to contain 'row-level locking'", err.Error())
	}
}

func TestRender_RejectsIN(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "users"},
		Fields:    []types.Field{{Name: "id"}},
		WhereClause: types.Condition{
			Field:    types.Field{Name: "status"},
			Operator: types.IN,
			Value:    types.Param{Name: "statuses"},
		},
	}

	_, err := r.Render(ast)
	if err == nil {
		t.Fatal("expected error for IN, got nil")
	}
	if !strings.Contains(err.Error(), "IN") {
		t.Errorf("error = %q, want to contain 'IN'", err.Error())
	}
}

func TestRender_RejectsILIKE(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "users"},
		Fields:    []types.Field{{Name: "id"}},
		WhereClause: types.Condition{
			Field:    types.Field{Name: "name"},
			Operator: types.ILIKE,
			Value:    types.Param{Name: "pattern"},
		},
	}

	_, err := r.Render(ast)
	if err == nil {
		t.Fatal("expected error for ILIKE, got nil")
	}
	if !strings.Contains(err.Error(), "ILIKE") {
		t.Errorf("error = %q, want to contain 'ILIKE'", err.Error())
	}
}

func TestRender_RejectsRegex(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "users"},
		Fields:    []types.Field{{Name: "id"}},
		WhereClause: types.Condition{
			Field:    types.Field{Name: "email"},
			Operator: types.RegexMatch,
			Value:    types.Param{Name: "pattern"},
		},
	}

	_, err := r.Render(ast)
	if err == nil {
		t.Fatal("expected error for regex, got nil")
	}
	if !strings.Contains(err.Error(), "regex") {
		t.Errorf("error = %q, want to contain 'regex'", err.Error())
	}
}

func TestRender_RejectsArrayOperators(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "users"},
		Fields:    []types.Field{{Name: "id"}},
		WhereClause: types.Condition{
			Field:    types.Field{Name: "tags"},
			Operator: types.ArrayContains,
			Value:    types.Param{Name: "tag"},
		},
	}

	_, err := r.Render(ast)
	if err == nil {
		t.Fatal("expected error for array operators, got nil")
	}
	if !strings.Contains(err.Error(), "array") {
		t.Errorf("error = %q, want to contain 'array'", err.Error())
	}
}

func TestRender_RejectsVectorOperators(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "items"},
		Fields:    []types.Field{{Name: "id"}},
		WhereClause: types.Condition{
			Field:    types.Field{Name: "embedding"},
			Operator: types.VectorL2Distance,
			Value:    types.Param{Name: "query_vec"},
		},
	}

	_, err := r.Render(ast)
	if err == nil {
		t.Fatal("expected error for vector operators, got nil")
	}
	if !strings.Contains(err.Error(), "vector") {
		t.Errorf("error = %q, want to contain 'vector'", err.Error())
	}
}

// Test supported features

func TestRender_SupportsDistinct(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "users"},
		Fields:    []types.Field{{Name: "name"}},
		Distinct:  true,
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := `SELECT DISTINCT "name" FROM "users"`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_SupportsReturning(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpInsert,
		Target:    types.Table{Name: "users"},
		Values: []map[types.Field]types.Param{
			{{Name: "name"}: {Name: "name_val"}},
		},
		Returning: []types.Field{{Name: "id"}},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if !strings.Contains(result.SQL, "RETURNING") {
		t.Errorf("SQL = %q, want to contain 'RETURNING'", result.SQL)
	}
}

func TestRender_SupportsOnConflict(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpInsert,
		Target:    types.Table{Name: "users"},
		Values: []map[types.Field]types.Param{
			{{Name: "email"}: {Name: "email_val"}},
		},
		OnConflict: &types.ConflictClause{
			Columns: []types.Field{{Name: "email"}},
			Action:  types.DoNothing,
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if !strings.Contains(result.SQL, "ON CONFLICT") {
		t.Errorf("SQL = %q, want to contain 'ON CONFLICT'", result.SQL)
	}
	if !strings.Contains(result.SQL, "DO NOTHING") {
		t.Errorf("SQL = %q, want to contain 'DO NOTHING'", result.SQL)
	}
}

func TestRender_SupportsLIKE(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "users"},
		Fields:    []types.Field{{Name: "id"}},
		WhereClause: types.Condition{
			Field:    types.Field{Name: "name"},
			Operator: types.LIKE,
			Value:    types.Param{Name: "pattern"},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := `SELECT "id" FROM "users" WHERE "name" LIKE :pattern`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_SupportsBetween(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "orders"},
		Fields:    []types.Field{{Name: "id"}},
		WhereClause: types.BetweenCondition{
			Field:   types.Field{Name: "amount"},
			Low:     types.Param{Name: "min"},
			High:    types.Param{Name: "max"},
			Negated: false,
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := `SELECT "id" FROM "orders" WHERE "amount" BETWEEN :min AND :max`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_SupportsJoin(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "orders", Alias: "o"},
		Fields:    []types.Field{{Name: "id", Table: "o"}},
		Joins: []types.Join{
			{
				Type:  types.InnerJoin,
				Table: types.Table{Name: "users", Alias: "u"},
				On: types.FieldComparison{
					LeftField:  types.Field{Name: "user_id", Table: "o"},
					Operator:   types.EQ,
					RightField: types.Field{Name: "id", Table: "u"},
				},
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if !strings.Contains(result.SQL, "INNER JOIN") {
		t.Errorf("SQL = %q, want to contain 'INNER JOIN'", result.SQL)
	}
}

func TestRender_SupportsWindowFunction(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "sales"},
		FieldExpressions: []types.FieldExpression{
			{
				Window: &types.WindowExpression{
					Function: types.WinRowNumber,
					Window: types.WindowSpec{
						PartitionBy: []types.Field{{Name: "category"}},
						OrderBy:     []types.OrderBy{{Field: types.Field{Name: "amount"}, Direction: types.DESC}},
					},
				},
				Alias: "rn",
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if !strings.Contains(result.SQL, "ROW_NUMBER()") {
		t.Errorf("SQL = %q, want to contain 'ROW_NUMBER()'", result.SQL)
	}
	if !strings.Contains(result.SQL, "OVER") {
		t.Errorf("SQL = %q, want to contain 'OVER'", result.SQL)
	}
}

func TestRenderCompound_Union(t *testing.T) {
	r := New()
	query := &types.CompoundQuery{
		Base: &types.AST{
			Operation: types.OpSelect,
			Target:    types.Table{Name: "users"},
			Fields:    []types.Field{{Name: "id"}},
		},
		Operands: []types.SetOperand{
			{
				Operation: types.SetUnion,
				AST: &types.AST{
					Operation: types.OpSelect,
					Target:    types.Table{Name: "admins"},
					Fields:    []types.Field{{Name: "id"}},
				},
			},
		},
	}

	result, err := r.RenderCompound(query)
	if err != nil {
		t.Fatalf("RenderCompound() error = %v", err)
	}

	expected := `SELECT "id" FROM "users" UNION SELECT "id" FROM "admins"`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_CastTypesMapCorrectly(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "data"},
		FieldExpressions: []types.FieldExpression{
			{
				Cast: &types.CastExpression{
					Field:    types.Field{Name: "value"},
					CastType: types.CastInteger,
				},
				Alias: "int_val",
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	// SQLite should map to INTEGER, not PostgreSQL's INTEGER
	if !strings.Contains(result.SQL, "CAST") {
		t.Errorf("SQL = %q, want to contain 'CAST'", result.SQL)
	}
	if !strings.Contains(result.SQL, "INTEGER") {
		t.Errorf("SQL = %q, want to contain 'INTEGER'", result.SQL)
	}
}

func TestRender_ParameterizedLimit(t *testing.T) {
	r := New()
	pageSize := types.Param{Name: "page_size"}
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "users"},
		Fields:    []types.Field{{Name: "id"}},
		Limit:     &types.PaginationValue{Param: &pageSize},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := `SELECT "id" FROM "users" LIMIT :page_size`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}

	if len(result.RequiredParams) != 1 || result.RequiredParams[0] != "page_size" {
		t.Errorf("RequiredParams = %v, want [page_size]", result.RequiredParams)
	}
}

func TestRender_ParameterizedOffset(t *testing.T) {
	r := New()
	offset := types.Param{Name: "offset"}
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "users"},
		Fields:    []types.Field{{Name: "id"}},
		Offset:    &types.PaginationValue{Param: &offset},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := `SELECT "id" FROM "users" OFFSET :offset`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}

	if len(result.RequiredParams) != 1 || result.RequiredParams[0] != "offset" {
		t.Errorf("RequiredParams = %v, want [offset]", result.RequiredParams)
	}
}

func TestRender_ParameterizedLimitAndOffset(t *testing.T) {
	r := New()
	pageSize := types.Param{Name: "page_size"}
	offset := types.Param{Name: "offset"}
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "users"},
		Fields:    []types.Field{{Name: "id"}},
		Limit:     &types.PaginationValue{Param: &pageSize},
		Offset:    &types.PaginationValue{Param: &offset},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := `SELECT "id" FROM "users" LIMIT :page_size OFFSET :offset`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}

	if len(result.RequiredParams) != 2 {
		t.Errorf("RequiredParams = %v, want [page_size, offset]", result.RequiredParams)
	}
}

func TestRender_StringUpper(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "users"},
		FieldExpressions: []types.FieldExpression{
			{
				String: &types.StringExpression{
					Function: types.StringUpper,
					Field:    types.Field{Name: "name"},
				},
				Alias: "upper_name",
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := `SELECT UPPER("name") AS "upper_name" FROM "users"`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_StringLower(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "users"},
		FieldExpressions: []types.FieldExpression{
			{
				String: &types.StringExpression{
					Function: types.StringLower,
					Field:    types.Field{Name: "email"},
				},
				Alias: "lower_email",
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := `SELECT LOWER("email") AS "lower_email" FROM "users"`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_StringConcat(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "users"},
		FieldExpressions: []types.FieldExpression{
			{
				String: &types.StringExpression{
					Function: types.StringConcat,
					Field:    types.Field{Name: "first_name"},
					Fields:   []types.Field{{Name: "last_name"}},
				},
				Alias: "full_name",
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	// SQLite uses || for concatenation
	expected := `SELECT ("first_name" || "last_name") AS "full_name" FROM "users"`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_StringSubstring(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "users"},
		FieldExpressions: []types.FieldExpression{
			{
				String: &types.StringExpression{
					Function: types.StringSubstring,
					Field:    types.Field{Name: "name"},
					Args:     []types.Param{{Name: "start"}, {Name: "length"}},
				},
				Alias: "substr",
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	// SQLite uses SUBSTR instead of SUBSTRING
	expected := `SELECT SUBSTR("name", :start, :length) AS "substr" FROM "users"`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_StringReplace(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "users"},
		FieldExpressions: []types.FieldExpression{
			{
				String: &types.StringExpression{
					Function: types.StringReplace,
					Field:    types.Field{Name: "name"},
					Args:     []types.Param{{Name: "search"}, {Name: "replacement"}},
				},
				Alias: "replaced",
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := `SELECT REPLACE("name", :search, :replacement) AS "replaced" FROM "users"`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_DateNow(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "users"},
		FieldExpressions: []types.FieldExpression{
			{
				Date: &types.DateExpression{
					Function: types.DateNow,
				},
				Alias: "current_time",
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	// SQLite uses DATETIME('now') instead of NOW()
	expected := `SELECT DATETIME('now') AS "current_time" FROM "users"`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_DateCurrentDate(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "users"},
		FieldExpressions: []types.FieldExpression{
			{
				Date: &types.DateExpression{
					Function: types.DateCurrentDate,
				},
				Alias: "today",
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	// SQLite uses DATE('now') instead of CURRENT_DATE
	expected := `SELECT DATE('now') AS "today" FROM "users"`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_DateExtract(t *testing.T) {
	r := New()
	createdAt := types.Field{Name: "created_at"}
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "users"},
		FieldExpressions: []types.FieldExpression{
			{
				Date: &types.DateExpression{
					Function: types.DateExtract,
					Part:     types.PartYear,
					Field:    &createdAt,
				},
				Alias: "year",
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	// SQLite uses STRFTIME with CAST to INTEGER for EXTRACT
	expected := `SELECT CAST(STRFTIME('%Y', "created_at") AS INTEGER) AS "year" FROM "users"`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_DateTrunc(t *testing.T) {
	r := New()
	createdAt := types.Field{Name: "created_at"}
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "users"},
		FieldExpressions: []types.FieldExpression{
			{
				Date: &types.DateExpression{
					Function: types.DateTrunc,
					Part:     types.PartMonth,
					Field:    &createdAt,
				},
				Alias: "month_start",
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	// SQLite uses STRFTIME for DATE_TRUNC
	expected := `SELECT STRFTIME('%Y-%m-01 00:00:00', "created_at") AS "month_start" FROM "users"`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

// Additional tests for coverage

func TestRender_Count(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpCount,
		Target:    types.Table{Name: "users"},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := `SELECT COUNT(*) FROM "users"`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_CountWithWhere(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpCount,
		Target:    types.Table{Name: "users"},
		WhereClause: types.Condition{
			Field:    types.Field{Name: "active"},
			Operator: types.EQ,
			Value:    types.Param{Name: "active"},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := `SELECT COUNT(*) FROM "users" WHERE "active" = :active`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_AggregateExpression(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "orders"},
		FieldExpressions: []types.FieldExpression{
			{
				Aggregate: types.AggSum,
				Field:     types.Field{Name: "total"},
				Alias:     "total_sum",
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := `SELECT SUM("total") AS "total_sum" FROM "orders"`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_AggregateWithGroupBy(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "orders"},
		Fields:    []types.Field{{Name: "customer_id"}},
		FieldExpressions: []types.FieldExpression{
			{
				Aggregate: types.AggSum,
				Field:     types.Field{Name: "amount"},
				Alias:     "total_amount",
			},
			{
				Aggregate: types.AggCountField,
				Alias:     "order_count",
			},
		},
		GroupBy: []types.Field{{Name: "customer_id"}},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if !strings.Contains(result.SQL, "SUM") {
		t.Errorf("SQL = %q, want to contain 'SUM'", result.SQL)
	}
	if !strings.Contains(result.SQL, "COUNT") {
		t.Errorf("SQL = %q, want to contain 'COUNT'", result.SQL)
	}
	if !strings.Contains(result.SQL, "GROUP BY") {
		t.Errorf("SQL = %q, want to contain 'GROUP BY'", result.SQL)
	}
}

func TestRender_CaseExpression(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "orders"},
		FieldExpressions: []types.FieldExpression{
			{
				Case: &types.CaseExpression{
					WhenClauses: []types.WhenClause{
						{
							Condition: types.Condition{
								Field:    types.Field{Name: "status"},
								Operator: types.EQ,
								Value:    types.Param{Name: "pending"},
							},
							Result: types.Param{Name: "pending_label"},
						},
						{
							Condition: types.Condition{
								Field:    types.Field{Name: "status"},
								Operator: types.EQ,
								Value:    types.Param{Name: "complete"},
							},
							Result: types.Param{Name: "complete_label"},
						},
					},
					ElseValue: &types.Param{Name: "other_label"},
				},
				Alias: "status_label",
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if !strings.Contains(result.SQL, "CASE") {
		t.Errorf("SQL = %q, want to contain 'CASE'", result.SQL)
	}
	if !strings.Contains(result.SQL, "WHEN") {
		t.Errorf("SQL = %q, want to contain 'WHEN'", result.SQL)
	}
	if !strings.Contains(result.SQL, "THEN") {
		t.Errorf("SQL = %q, want to contain 'THEN'", result.SQL)
	}
	if !strings.Contains(result.SQL, "ELSE") {
		t.Errorf("SQL = %q, want to contain 'ELSE'", result.SQL)
	}
	if !strings.Contains(result.SQL, "END") {
		t.Errorf("SQL = %q, want to contain 'END'", result.SQL)
	}
}

func TestRender_CoalesceExpression(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "users"},
		FieldExpressions: []types.FieldExpression{
			{
				Coalesce: &types.CoalesceExpression{
					Values: []types.Param{
						{Name: "nickname"},
						{Name: "username"},
						{Name: "default_name"},
					},
				},
				Alias: "display_name",
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if !strings.Contains(result.SQL, "COALESCE") {
		t.Errorf("SQL = %q, want to contain 'COALESCE'", result.SQL)
	}
}

func TestRender_NullIfExpression(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "orders"},
		FieldExpressions: []types.FieldExpression{
			{
				NullIf: &types.NullIfExpression{
					Value1: types.Param{Name: "discount"},
					Value2: types.Param{Name: "zero"},
				},
				Alias: "safe_discount",
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if !strings.Contains(result.SQL, "NULLIF") {
		t.Errorf("SQL = %q, want to contain 'NULLIF'", result.SQL)
	}
}

func TestRender_ConditionGroup_AND(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "users"},
		Fields:    []types.Field{{Name: "id"}},
		WhereClause: types.ConditionGroup{
			Logic: types.AND,
			Conditions: []types.ConditionItem{
				types.Condition{
					Field:    types.Field{Name: "active"},
					Operator: types.EQ,
					Value:    types.Param{Name: "active"},
				},
				types.Condition{
					Field:    types.Field{Name: "role"},
					Operator: types.EQ,
					Value:    types.Param{Name: "role"},
				},
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if !strings.Contains(result.SQL, "AND") {
		t.Errorf("SQL = %q, want to contain 'AND'", result.SQL)
	}
}

func TestRender_ConditionGroup_OR(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "users"},
		Fields:    []types.Field{{Name: "id"}},
		WhereClause: types.ConditionGroup{
			Logic: types.OR,
			Conditions: []types.ConditionItem{
				types.Condition{
					Field:    types.Field{Name: "role"},
					Operator: types.EQ,
					Value:    types.Param{Name: "admin"},
				},
				types.Condition{
					Field:    types.Field{Name: "role"},
					Operator: types.EQ,
					Value:    types.Param{Name: "superuser"},
				},
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if !strings.Contains(result.SQL, "OR") {
		t.Errorf("SQL = %q, want to contain 'OR'", result.SQL)
	}
}

func TestRender_Subquery(t *testing.T) {
	// SQLite doesn't support IN with array parameters, but EXISTS works
	r := New()
	subquery := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "customers"},
		Fields:    []types.Field{{Name: "id"}},
		WhereClause: types.Condition{
			Field:    types.Field{Name: "vip"},
			Operator: types.EQ,
			Value:    types.Param{Name: "is_vip"},
		},
	}
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "orders"},
		Fields:    []types.Field{{Name: "id"}},
		WhereClause: types.SubqueryCondition{
			Operator: types.EXISTS,
			Subquery: types.Subquery{AST: subquery},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if !strings.Contains(result.SQL, "(SELECT") {
		t.Errorf("SQL = %q, want to contain subquery", result.SQL)
	}
}

func TestRender_HavingCondition(t *testing.T) {
	r := New()
	amountField := types.Field{Name: "amount"}
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "orders"},
		Fields:    []types.Field{{Name: "customer_id"}},
		FieldExpressions: []types.FieldExpression{
			{
				Aggregate: types.AggSum,
				Field:     types.Field{Name: "amount"},
				Alias:     "total",
			},
		},
		GroupBy: []types.Field{{Name: "customer_id"}},
		Having: []types.ConditionItem{
			types.AggregateCondition{
				Func:     types.AggSum,
				Field:    &amountField,
				Operator: types.GT,
				Value:    types.Param{Name: "min_total"},
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if !strings.Contains(result.SQL, "HAVING") {
		t.Errorf("SQL = %q, want to contain 'HAVING'", result.SQL)
	}
	if !strings.Contains(result.SQL, "SUM") {
		t.Errorf("SQL = %q, want to contain 'SUM'", result.SQL)
	}
}

func TestRender_MathExpression(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "products"},
		FieldExpressions: []types.FieldExpression{
			{
				Math: &types.MathExpression{
					Function: types.MathAbs,
					Field:    types.Field{Name: "price"},
				},
				Alias: "abs_price",
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if !strings.Contains(result.SQL, "ABS") {
		t.Errorf("SQL = %q, want to contain 'ABS'", result.SQL)
	}
}

func TestRender_Operators(t *testing.T) {
	tests := []struct {
		name     string
		operator types.Operator
		want     string
	}{
		{"less than", types.LT, "<"},
		{"greater than", types.GT, ">"},
		{"less than or equal", types.LE, "<="},
		{"greater than or equal", types.GE, ">="},
		{"not equal", types.NE, "!="},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New()
			ast := &types.AST{
				Operation: types.OpSelect,
				Target:    types.Table{Name: "products"},
				Fields:    []types.Field{{Name: "id"}},
				WhereClause: types.Condition{
					Field:    types.Field{Name: "price"},
					Operator: tt.operator,
					Value:    types.Param{Name: "price"},
				},
			}

			result, err := r.Render(ast)
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}

			if !strings.Contains(result.SQL, tt.want) {
				t.Errorf("SQL = %q, want to contain %q", result.SQL, tt.want)
			}
		})
	}
}

func TestRender_IsNull(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "users"},
		Fields:    []types.Field{{Name: "id"}},
		WhereClause: types.Condition{
			Field:    types.Field{Name: "deleted_at"},
			Operator: types.IsNull,
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := `SELECT "id" FROM "users" WHERE "deleted_at" IS NULL`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_IsNotNull(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "users"},
		Fields:    []types.Field{{Name: "id"}},
		WhereClause: types.Condition{
			Field:    types.Field{Name: "email"},
			Operator: types.IsNotNull,
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := `SELECT "id" FROM "users" WHERE "email" IS NOT NULL`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_LeftJoin(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "orders", Alias: "o"},
		Fields:    []types.Field{{Name: "id", Table: "o"}},
		Joins: []types.Join{
			{
				Type:  types.LeftJoin,
				Table: types.Table{Name: "customers", Alias: "c"},
				On: types.FieldComparison{
					LeftField:  types.Field{Name: "customer_id", Table: "o"},
					Operator:   types.EQ,
					RightField: types.Field{Name: "id", Table: "c"},
				},
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if !strings.Contains(result.SQL, "LEFT JOIN") {
		t.Errorf("SQL = %q, want to contain 'LEFT JOIN'", result.SQL)
	}
}

func TestRender_MultipleJoins(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "orders", Alias: "o"},
		Fields:    []types.Field{{Name: "id", Table: "o"}},
		Joins: []types.Join{
			{
				Type:  types.InnerJoin,
				Table: types.Table{Name: "customers", Alias: "c"},
				On: types.FieldComparison{
					LeftField:  types.Field{Name: "customer_id", Table: "o"},
					Operator:   types.EQ,
					RightField: types.Field{Name: "id", Table: "c"},
				},
			},
			{
				Type:  types.LeftJoin,
				Table: types.Table{Name: "products", Alias: "p"},
				On: types.FieldComparison{
					LeftField:  types.Field{Name: "product_id", Table: "o"},
					Operator:   types.EQ,
					RightField: types.Field{Name: "id", Table: "p"},
				},
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if !strings.Contains(result.SQL, "INNER JOIN") {
		t.Errorf("SQL = %q, want to contain 'INNER JOIN'", result.SQL)
	}
	if !strings.Contains(result.SQL, "LEFT JOIN") {
		t.Errorf("SQL = %q, want to contain 'LEFT JOIN'", result.SQL)
	}
}

func TestRender_AggCountStar(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "orders"},
		Fields:    []types.Field{{Name: "status"}},
		FieldExpressions: []types.FieldExpression{
			{
				Aggregate: types.AggCountField,
				Alias:     "count",
			},
		},
		GroupBy: []types.Field{{Name: "status"}},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if !strings.Contains(result.SQL, "COUNT(*)") {
		t.Errorf("SQL = %q, want to contain 'COUNT(*)'", result.SQL)
	}
}

func TestRender_AvgAggregate(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "products"},
		FieldExpressions: []types.FieldExpression{
			{
				Aggregate: types.AggAvg,
				Field:     types.Field{Name: "price"},
				Alias:     "avg_price",
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := `SELECT AVG("price") AS "avg_price" FROM "products"`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_MinMaxAggregate(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "products"},
		FieldExpressions: []types.FieldExpression{
			{
				Aggregate: types.AggMin,
				Field:     types.Field{Name: "price"},
				Alias:     "min_price",
			},
			{
				Aggregate: types.AggMax,
				Field:     types.Field{Name: "price"},
				Alias:     "max_price",
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if !strings.Contains(result.SQL, "MIN") {
		t.Errorf("SQL = %q, want to contain 'MIN'", result.SQL)
	}
	if !strings.Contains(result.SQL, "MAX") {
		t.Errorf("SQL = %q, want to contain 'MAX'", result.SQL)
	}
}

func TestRender_NotBetween(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "orders"},
		Fields:    []types.Field{{Name: "id"}},
		WhereClause: types.BetweenCondition{
			Field:   types.Field{Name: "amount"},
			Low:     types.Param{Name: "min"},
			High:    types.Param{Name: "max"},
			Negated: true,
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := `SELECT "id" FROM "orders" WHERE "amount" NOT BETWEEN :min AND :max`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_OrderByMultiple(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "users"},
		Fields:    []types.Field{{Name: "id"}, {Name: "name"}, {Name: "created_at"}},
		Ordering: []types.OrderBy{
			{Field: types.Field{Name: "name"}, Direction: types.ASC},
			{Field: types.Field{Name: "created_at"}, Direction: types.DESC},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if !strings.Contains(result.SQL, "ORDER BY") {
		t.Errorf("SQL = %q, want to contain 'ORDER BY'", result.SQL)
	}
	if !strings.Contains(result.SQL, "ASC") {
		t.Errorf("SQL = %q, want to contain 'ASC'", result.SQL)
	}
	if !strings.Contains(result.SQL, "DESC") {
		t.Errorf("SQL = %q, want to contain 'DESC'", result.SQL)
	}
}

func TestRender_ExistsSubquery(t *testing.T) {
	r := New()
	subquery := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "orders"},
		Fields:    []types.Field{{Name: "id"}},
		WhereClause: types.FieldComparison{
			LeftField:  types.Field{Name: "customer_id", Table: "orders"},
			Operator:   types.EQ,
			RightField: types.Field{Name: "id", Table: "customers"},
		},
	}
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "customers"},
		Fields:    []types.Field{{Name: "id"}},
		WhereClause: types.SubqueryCondition{
			Operator: types.EXISTS,
			Subquery: types.Subquery{AST: subquery},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if !strings.Contains(result.SQL, "EXISTS") {
		t.Errorf("SQL = %q, want to contain 'EXISTS'", result.SQL)
	}
}

func TestRender_OnConflictDoUpdate(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpInsert,
		Target:    types.Table{Name: "users"},
		Values: []map[types.Field]types.Param{
			{{Name: "email"}: {Name: "email_val"}, {Name: "name"}: {Name: "name_val"}},
		},
		OnConflict: &types.ConflictClause{
			Columns: []types.Field{{Name: "email"}},
			Action:  types.DoUpdate,
			Updates: map[types.Field]types.Param{
				{Name: "name"}: {Name: "name_val"},
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if !strings.Contains(result.SQL, "ON CONFLICT") {
		t.Errorf("SQL = %q, want to contain 'ON CONFLICT'", result.SQL)
	}
	if !strings.Contains(result.SQL, "DO UPDATE SET") {
		t.Errorf("SQL = %q, want to contain 'DO UPDATE SET'", result.SQL)
	}
}

func TestRender_FieldWithTable(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "users", Alias: "u"},
		Fields:    []types.Field{{Name: "id", Table: "u"}, {Name: "name", Table: "u"}},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	// SQLite uses table alias without quoting for qualified fields: u."id"
	if !strings.Contains(result.SQL, `u."id"`) {
		t.Errorf("SQL = %q, want to contain 'u.\"id\"'", result.SQL)
	}
}

// =============================================================================
// Additional RenderCompound Tests
// =============================================================================

func TestRenderCompound_UnionAll(t *testing.T) {
	r := New()
	query := &types.CompoundQuery{
		Base: &types.AST{
			Operation: types.OpSelect,
			Target:    types.Table{Name: "users"},
			Fields:    []types.Field{{Name: "id"}},
		},
		Operands: []types.SetOperand{
			{
				Operation: types.SetUnionAll,
				AST: &types.AST{
					Operation: types.OpSelect,
					Target:    types.Table{Name: "admins"},
					Fields:    []types.Field{{Name: "id"}},
				},
			},
		},
	}

	result, err := r.RenderCompound(query)
	if err != nil {
		t.Fatalf("RenderCompound() error = %v", err)
	}

	expected := `SELECT "id" FROM "users" UNION ALL SELECT "id" FROM "admins"`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRenderCompound_Intersect(t *testing.T) {
	r := New()
	query := &types.CompoundQuery{
		Base: &types.AST{
			Operation: types.OpSelect,
			Target:    types.Table{Name: "users"},
			Fields:    []types.Field{{Name: "id"}},
		},
		Operands: []types.SetOperand{
			{
				Operation: types.SetIntersect,
				AST: &types.AST{
					Operation: types.OpSelect,
					Target:    types.Table{Name: "active_users"},
					Fields:    []types.Field{{Name: "id"}},
				},
			},
		},
	}

	result, err := r.RenderCompound(query)
	if err != nil {
		t.Fatalf("RenderCompound() error = %v", err)
	}

	expected := `SELECT "id" FROM "users" INTERSECT SELECT "id" FROM "active_users"`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRenderCompound_Except(t *testing.T) {
	r := New()
	query := &types.CompoundQuery{
		Base: &types.AST{
			Operation: types.OpSelect,
			Target:    types.Table{Name: "users"},
			Fields:    []types.Field{{Name: "id"}},
		},
		Operands: []types.SetOperand{
			{
				Operation: types.SetExcept,
				AST: &types.AST{
					Operation: types.OpSelect,
					Target:    types.Table{Name: "banned_users"},
					Fields:    []types.Field{{Name: "id"}},
				},
			},
		},
	}

	result, err := r.RenderCompound(query)
	if err != nil {
		t.Fatalf("RenderCompound() error = %v", err)
	}

	expected := `SELECT "id" FROM "users" EXCEPT SELECT "id" FROM "banned_users"`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRenderCompound_WithOrderByLimit(t *testing.T) {
	r := New()
	limitVal := 10
	limit := types.PaginationValue{Static: &limitVal}
	query := &types.CompoundQuery{
		Base: &types.AST{
			Operation: types.OpSelect,
			Target:    types.Table{Name: "users"},
			Fields:    []types.Field{{Name: "id"}, {Name: "name"}},
		},
		Operands: []types.SetOperand{
			{
				Operation: types.SetUnion,
				AST: &types.AST{
					Operation: types.OpSelect,
					Target:    types.Table{Name: "admins"},
					Fields:    []types.Field{{Name: "id"}, {Name: "name"}},
				},
			},
		},
		Ordering: []types.OrderBy{{Field: types.Field{Name: "name"}, Direction: types.ASC}},
		Limit:    &limit,
	}

	result, err := r.RenderCompound(query)
	if err != nil {
		t.Fatalf("RenderCompound() error = %v", err)
	}

	expected := `SELECT "id", "name" FROM "users" UNION SELECT "id", "name" FROM "admins" ORDER BY "name" ASC LIMIT 10`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRenderCompound_MultipleOperations(t *testing.T) {
	r := New()
	query := &types.CompoundQuery{
		Base: &types.AST{
			Operation: types.OpSelect,
			Target:    types.Table{Name: "users"},
			Fields:    []types.Field{{Name: "id"}},
		},
		Operands: []types.SetOperand{
			{
				Operation: types.SetUnion,
				AST: &types.AST{
					Operation: types.OpSelect,
					Target:    types.Table{Name: "admins"},
					Fields:    []types.Field{{Name: "id"}},
				},
			},
			{
				Operation: types.SetIntersect,
				AST: &types.AST{
					Operation: types.OpSelect,
					Target:    types.Table{Name: "active_users"},
					Fields:    []types.Field{{Name: "id"}},
				},
			},
		},
	}

	result, err := r.RenderCompound(query)
	if err != nil {
		t.Fatalf("RenderCompound() error = %v", err)
	}

	expected := `SELECT "id" FROM "users" UNION SELECT "id" FROM "admins" INTERSECT SELECT "id" FROM "active_users"`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRenderCompound_ParameterNamespacing(t *testing.T) {
	r := New()
	query := &types.CompoundQuery{
		Base: &types.AST{
			Operation: types.OpSelect,
			Target:    types.Table{Name: "users"},
			Fields:    []types.Field{{Name: "id"}},
			WhereClause: types.Condition{
				Field:    types.Field{Name: "active"},
				Operator: types.EQ,
				Value:    types.Param{Name: "is_active"},
			},
		},
		Operands: []types.SetOperand{
			{
				Operation: types.SetUnion,
				AST: &types.AST{
					Operation: types.OpSelect,
					Target:    types.Table{Name: "admins"},
					Fields:    []types.Field{{Name: "id"}},
					WhereClause: types.Condition{
						Field:    types.Field{Name: "active"},
						Operator: types.EQ,
						Value:    types.Param{Name: "is_active"},
					},
				},
			},
		},
	}

	result, err := r.RenderCompound(query)
	if err != nil {
		t.Fatalf("RenderCompound() error = %v", err)
	}

	// Parameters should be namespaced with q0_ and q1_ prefixes
	expected := `SELECT "id" FROM "users" WHERE "active" = :q0_is_active UNION SELECT "id" FROM "admins" WHERE "active" = :q1_is_active`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}

	if len(result.RequiredParams) != 2 {
		t.Errorf("RequiredParams = %v, want 2 params", result.RequiredParams)
	}
}

func TestCapabilities(t *testing.T) {
	r := New()
	caps := r.Capabilities()

	if caps.DistinctOn {
		t.Error("DistinctOn should be false")
	}
	if !caps.Upsert {
		t.Error("Upsert should be true")
	}
	if !caps.ReturningOnInsert {
		t.Error("ReturningOnInsert should be true")
	}
	if !caps.ReturningOnUpdate {
		t.Error("ReturningOnUpdate should be true")
	}
	if !caps.ReturningOnDelete {
		t.Error("ReturningOnDelete should be true")
	}
	if caps.CaseInsensitiveLike {
		t.Error("CaseInsensitiveLike should be false")
	}
	if caps.RegexOperators {
		t.Error("RegexOperators should be false")
	}
	if caps.ArrayOperators {
		t.Error("ArrayOperators should be false")
	}
	if caps.InArray {
		t.Error("InArray should be false")
	}
	if caps.RowLocking != render.RowLockingNone {
		t.Errorf("RowLocking = %v, want RowLockingNone", caps.RowLocking)
	}
}
