package postgres

import (
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

	if len(result.RequiredParams) != 1 || result.RequiredParams[0] != "is_active" {
		t.Errorf("RequiredParams = %v, want [is_active]", result.RequiredParams)
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

	// Fields are sorted alphabetically
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

func TestRender_IN_PostgresArraySyntax(t *testing.T) {
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

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	// PostgreSQL uses = ANY(:array) syntax
	expected := `SELECT "id" FROM "users" WHERE "status" = ANY(:statuses)`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_NotIN_PostgresArraySyntax(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "users"},
		Fields:    []types.Field{{Name: "id"}},
		WhereClause: types.Condition{
			Field:    types.Field{Name: "status"},
			Operator: types.NotIn,
			Value:    types.Param{Name: "statuses"},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	// PostgreSQL uses != ALL(:array) syntax
	expected := `SELECT "id" FROM "users" WHERE "status" != ALL(:statuses)`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_DistinctOn(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation:  types.OpSelect,
		Target:     types.Table{Name: "users"},
		Fields:     []types.Field{{Name: "id"}, {Name: "name"}},
		DistinctOn: []types.Field{{Name: "email"}},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := `SELECT DISTINCT ON ("email") "id", "name" FROM "users"`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
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

	expected := `(SELECT "id" FROM "users") UNION (SELECT "id" FROM "admins")`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
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

	expected := `SELECT CONCAT("first_name", "last_name") AS "full_name" FROM "users"`
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

	expected := `SELECT SUBSTRING("name" FROM :start FOR :length) AS "substr" FROM "users"`
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

	expected := `SELECT NOW() AS "current_time" FROM "users"`
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

	expected := `SELECT CURRENT_DATE AS "today" FROM "users"`
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

	expected := `SELECT EXTRACT(YEAR FROM "created_at") AS "year" FROM "users"`
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

	expected := `SELECT DATE_TRUNC('month', "created_at") AS "month_start" FROM "users"`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

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
			Value:    types.Param{Name: "is_active"},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := `SELECT COUNT(*) FROM "users" WHERE "active" = :is_active`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_CaseExpression(t *testing.T) {
	r := New()
	seniorLabel := types.Param{Name: "senior_label"}
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "users"},
		FieldExpressions: []types.FieldExpression{
			{
				Case: &types.CaseExpression{
					WhenClauses: []types.WhenClause{
						{
							Condition: types.Condition{
								Field:    types.Field{Name: "age"},
								Operator: types.LT,
								Value:    types.Param{Name: "young_age"},
							},
							Result: types.Param{Name: "young_label"},
						},
						{
							Condition: types.Condition{
								Field:    types.Field{Name: "age"},
								Operator: types.LT,
								Value:    types.Param{Name: "mid_age"},
							},
							Result: types.Param{Name: "mid_label"},
						},
					},
					ElseValue: &seniorLabel,
				},
				Alias: "age_group",
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := `SELECT CASE WHEN "age" < :young_age THEN :young_label WHEN "age" < :mid_age THEN :mid_label ELSE :senior_label END AS "age_group" FROM "users"`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
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
					Values: []types.Param{{Name: "nickname"}, {Name: "username"}},
				},
				Alias: "display_name",
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := `SELECT COALESCE(:nickname, :username) AS "display_name" FROM "users"`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_NullIfExpression(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "users"},
		FieldExpressions: []types.FieldExpression{
			{
				NullIf: &types.NullIfExpression{
					Value1: types.Param{Name: "status"},
					Value2: types.Param{Name: "empty_status"},
				},
				Alias: "clean_status",
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := `SELECT NULLIF(:status, :empty_status) AS "clean_status" FROM "users"`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_MathExpression_Round(t *testing.T) {
	r := New()
	precision := types.Param{Name: "precision"}
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "products"},
		FieldExpressions: []types.FieldExpression{
			{
				Math: &types.MathExpression{
					Function:  types.MathRound,
					Field:     types.Field{Name: "price"},
					Precision: &precision,
				},
				Alias: "rounded_price",
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := `SELECT ROUND("price", :precision) AS "rounded_price" FROM "products"`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_MathExpression_Floor(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "products"},
		FieldExpressions: []types.FieldExpression{
			{
				Math: &types.MathExpression{
					Function: types.MathFloor,
					Field:    types.Field{Name: "price"},
				},
				Alias: "floor_price",
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := `SELECT FLOOR("price") AS "floor_price" FROM "products"`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_MathExpression_Ceil(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "products"},
		FieldExpressions: []types.FieldExpression{
			{
				Math: &types.MathExpression{
					Function: types.MathCeil,
					Field:    types.Field{Name: "price"},
				},
				Alias: "ceil_price",
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := `SELECT CEIL("price") AS "ceil_price" FROM "products"`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_MathExpression_Abs(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "transactions"},
		FieldExpressions: []types.FieldExpression{
			{
				Math: &types.MathExpression{
					Function: types.MathAbs,
					Field:    types.Field{Name: "amount"},
				},
				Alias: "abs_amount",
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := `SELECT ABS("amount") AS "abs_amount" FROM "transactions"`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_WindowFunction(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "sales"},
		Fields:    []types.Field{{Name: "product_id"}, {Name: "amount"}},
		FieldExpressions: []types.FieldExpression{
			{
				Window: &types.WindowExpression{
					Function: types.WinRowNumber,
					Window: types.WindowSpec{
						PartitionBy: []types.Field{{Name: "product_id"}},
						OrderBy:     []types.OrderBy{{Field: types.Field{Name: "amount"}, Direction: types.DESC}},
					},
				},
				Alias: "row_num",
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := `SELECT "product_id", "amount", ROW_NUMBER() OVER (PARTITION BY "product_id" ORDER BY "amount" DESC) AS "row_num" FROM "sales"`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_WindowFunction_Rank(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "scores"},
		Fields:    []types.Field{{Name: "player_id"}, {Name: "score"}},
		FieldExpressions: []types.FieldExpression{
			{
				Window: &types.WindowExpression{
					Function: types.WinRank,
					Window: types.WindowSpec{
						OrderBy: []types.OrderBy{{Field: types.Field{Name: "score"}, Direction: types.DESC}},
					},
				},
				Alias: "rank",
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := `SELECT "player_id", "score", RANK() OVER (ORDER BY "score" DESC) AS "rank" FROM "scores"`
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
		Fields:    []types.Field{{Name: "user_id"}},
		FieldExpressions: []types.FieldExpression{
			{
				Aggregate: types.AggCountField,
				Alias:     "order_count",
			},
		},
		GroupBy: []types.Field{{Name: "user_id"}},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := `SELECT "user_id", COUNT(*) AS "order_count" FROM "orders" GROUP BY "user_id"`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_BetweenCondition(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "products"},
		Fields:    []types.Field{{Name: "name"}, {Name: "price"}},
		WhereClause: types.BetweenCondition{
			Field: types.Field{Name: "price"},
			Low:   types.Param{Name: "min_price"},
			High:  types.Param{Name: "max_price"},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := `SELECT "name", "price" FROM "products" WHERE "price" BETWEEN :min_price AND :max_price`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_ConditionGroup_AND(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "users"},
		Fields:    []types.Field{{Name: "id"}, {Name: "name"}},
		WhereClause: types.ConditionGroup{
			Logic: types.AND,
			Conditions: []types.ConditionItem{
				types.Condition{
					Field:    types.Field{Name: "active"},
					Operator: types.EQ,
					Value:    types.Param{Name: "is_active"},
				},
				types.Condition{
					Field:    types.Field{Name: "age"},
					Operator: types.GE,
					Value:    types.Param{Name: "min_age"},
				},
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := `SELECT "id", "name" FROM "users" WHERE ("active" = :is_active AND "age" >= :min_age)`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
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
					Value:    types.Param{Name: "admin_role"},
				},
				types.Condition{
					Field:    types.Field{Name: "role"},
					Operator: types.EQ,
					Value:    types.Param{Name: "super_role"},
				},
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := `SELECT "id" FROM "users" WHERE ("role" = :admin_role OR "role" = :super_role)`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_Join(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "users", Alias: "u"},
		Fields:    []types.Field{{Name: "name", Table: "u"}},
		Joins: []types.Join{
			{
				Type:  types.InnerJoin,
				Table: types.Table{Name: "orders", Alias: "o"},
				On: types.FieldComparison{
					LeftField:  types.Field{Name: "id", Table: "u"},
					Operator:   types.EQ,
					RightField: types.Field{Name: "user_id", Table: "o"},
				},
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := `SELECT u."name" FROM "users" u INNER JOIN "orders" o ON u."id" = o."user_id"`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_LeftJoin(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "users"},
		Fields:    []types.Field{{Name: "name"}},
		Joins: []types.Join{
			{
				Type:  types.LeftJoin,
				Table: types.Table{Name: "profiles"},
				On: types.FieldComparison{
					LeftField:  types.Field{Name: "id", Table: "users"},
					Operator:   types.EQ,
					RightField: types.Field{Name: "user_id", Table: "profiles"},
				},
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := `SELECT "name" FROM "users" LEFT JOIN "profiles" ON users."id" = profiles."user_id"`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_HavingCondition(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "orders"},
		Fields:    []types.Field{{Name: "user_id"}},
		FieldExpressions: []types.FieldExpression{
			{
				Aggregate: types.AggCountField,
				Alias:     "order_count",
			},
		},
		GroupBy: []types.Field{{Name: "user_id"}},
		Having: []types.ConditionItem{
			types.AggregateCondition{
				Func:     types.AggCountField,
				Operator: types.GE,
				Value:    types.Param{Name: "min_orders"},
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := `SELECT "user_id", COUNT(*) AS "order_count" FROM "orders" GROUP BY "user_id" HAVING COUNT(*) >= :min_orders`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_Subquery(t *testing.T) {
	r := New()
	subquery := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "orders"},
		Fields:    []types.Field{{Name: "user_id"}},
		WhereClause: types.Condition{
			Field:    types.Field{Name: "status"},
			Operator: types.EQ,
			Value:    types.Param{Name: "order_status"},
		},
	}

	idField := types.Field{Name: "id"}
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "users"},
		Fields:    []types.Field{{Name: "name"}},
		WhereClause: types.SubqueryCondition{
			Field:    &idField,
			Operator: types.IN,
			Subquery: types.Subquery{AST: subquery},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := `SELECT "name" FROM "users" WHERE "id" IN (SELECT "user_id" FROM "orders" WHERE "status" = :sq1_order_status)`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_InsertWithReturning(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpInsert,
		Target:    types.Table{Name: "users"},
		Values: []map[types.Field]types.Param{
			{
				{Name: "name"}: {Name: "name_val"},
			},
		},
		Returning: []types.Field{{Name: "id"}},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := `INSERT INTO "users" ("name") VALUES (:name_val) RETURNING "id"`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_OnConflictDoNothing(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpInsert,
		Target:    types.Table{Name: "users"},
		Values: []map[types.Field]types.Param{
			{
				{Name: "email"}: {Name: "email_val"},
			},
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

	expected := `INSERT INTO "users" ("email") VALUES (:email_val) ON CONFLICT ("email") DO NOTHING`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_OnConflictDoUpdate(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpInsert,
		Target:    types.Table{Name: "users"},
		Values: []map[types.Field]types.Param{
			{
				{Name: "email"}: {Name: "email_val"},
				{Name: "name"}:  {Name: "name_val"},
			},
		},
		OnConflict: &types.ConflictClause{
			Columns: []types.Field{{Name: "email"}},
			Action:  types.DoUpdate,
			Updates: map[types.Field]types.Param{
				{Name: "name"}: {Name: "new_name"},
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := `INSERT INTO "users" ("email", "name") VALUES (:email_val, :name_val) ON CONFLICT ("email") DO UPDATE SET "name" = :new_name`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_ForUpdate(t *testing.T) {
	r := New()
	lockForUpdate := types.LockForUpdate
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "users"},
		Fields:    []types.Field{{Name: "id"}, {Name: "name"}},
		Lock:      &lockForUpdate,
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := `SELECT "id", "name" FROM "users" FOR UPDATE`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_ForShare(t *testing.T) {
	r := New()
	lockForShare := types.LockForShare
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "users"},
		Fields:    []types.Field{{Name: "id"}},
		Lock:      &lockForShare,
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := `SELECT "id" FROM "users" FOR SHARE`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_ForUpdateSkipLocked(t *testing.T) {
	r := New()
	lock := types.LockForUpdate
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "jobs"},
		Fields:    []types.Field{{Name: "id"}},
		Lock:      &lock,
		LockWait:  types.LockWaitSkipLocked,
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := `SELECT "id" FROM "jobs" FOR UPDATE SKIP LOCKED`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_ForUpdateNoWait(t *testing.T) {
	r := New()
	lock := types.LockForUpdate
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "jobs"},
		Fields:    []types.Field{{Name: "id"}},
		Lock:      &lock,
		LockWait:  types.LockWaitNoWait,
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := `SELECT "id" FROM "jobs" FOR UPDATE NOWAIT`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_ForShareSkipLocked(t *testing.T) {
	r := New()
	lock := types.LockForShare
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "jobs"},
		Fields:    []types.Field{{Name: "id"}},
		Lock:      &lock,
		LockWait:  types.LockWaitSkipLocked,
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := `SELECT "id" FROM "jobs" FOR SHARE SKIP LOCKED`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_Operators(t *testing.T) {
	tests := []struct {
		name     string
		operator types.Operator
		expected string
	}{
		{"NE", types.NE, `SELECT "id" FROM "users" WHERE "status" != :status`},
		{"GT", types.GT, `SELECT "id" FROM "users" WHERE "status" > :status`},
		{"GE", types.GE, `SELECT "id" FROM "users" WHERE "status" >= :status`},
		{"LT", types.LT, `SELECT "id" FROM "users" WHERE "status" < :status`},
		{"LE", types.LE, `SELECT "id" FROM "users" WHERE "status" <= :status`},
		{"LIKE", types.LIKE, `SELECT "id" FROM "users" WHERE "status" LIKE :status`},
		{"ILIKE", types.ILIKE, `SELECT "id" FROM "users" WHERE "status" ILIKE :status`},
	}

	r := New()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ast := &types.AST{
				Operation: types.OpSelect,
				Target:    types.Table{Name: "users"},
				Fields:    []types.Field{{Name: "id"}},
				WhereClause: types.Condition{
					Field:    types.Field{Name: "status"},
					Operator: tt.operator,
					Value:    types.Param{Name: "status"},
				},
			}

			result, err := r.Render(ast)
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}

			if result.SQL != tt.expected {
				t.Errorf("SQL = %q, want %q", result.SQL, tt.expected)
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

// =============================================================================
// RenderCompound Tests
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

	expected := `(SELECT "id" FROM "users") UNION ALL (SELECT "id" FROM "admins")`
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

	expected := `(SELECT "id" FROM "users") INTERSECT (SELECT "id" FROM "active_users")`
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

	expected := `(SELECT "id" FROM "users") EXCEPT (SELECT "id" FROM "banned_users")`
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

	expected := `(SELECT "id", "name" FROM "users") UNION (SELECT "id", "name" FROM "admins") ORDER BY "name" ASC LIMIT 10`
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

	expected := `(SELECT "id" FROM "users") UNION (SELECT "id" FROM "admins") INTERSECT (SELECT "id" FROM "active_users")`
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
	expected := `(SELECT "id" FROM "users" WHERE "active" = :q0_is_active) UNION (SELECT "id" FROM "admins" WHERE "active" = :q1_is_active)`
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}

	// Should have both namespaced parameters
	if len(result.RequiredParams) != 2 {
		t.Errorf("RequiredParams = %v, want 2 params", result.RequiredParams)
	}
}

func TestCapabilities(t *testing.T) {
	r := New()
	caps := r.Capabilities()

	if !caps.DistinctOn {
		t.Error("DistinctOn should be true")
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
	if !caps.CaseInsensitiveLike {
		t.Error("CaseInsensitiveLike should be true")
	}
	if !caps.RegexOperators {
		t.Error("RegexOperators should be true")
	}
	if !caps.ArrayOperators {
		t.Error("ArrayOperators should be true")
	}
	if !caps.InArray {
		t.Error("InArray should be true")
	}
	if caps.RowLocking != render.RowLockingFull {
		t.Errorf("RowLocking = %v, want RowLockingFull", caps.RowLocking)
	}
	if !caps.RowLockingWaitPolicy {
		t.Error("RowLockingWaitPolicy should be true")
	}
}
