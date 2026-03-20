package mssql

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

	// SQL Server uses square brackets for quoting
	expected := "SELECT [id], [name] FROM [users]"
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

	// sqlx converts : to @ for MSSQL
	expected := "SELECT [id] FROM [users] WHERE [active] = :is_active"
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

	expected := "INSERT INTO [users] ([email], [name]) VALUES (:email_val, :name_val)"
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

	// SQL Server uses OUTPUT for RETURNING
	if !strings.Contains(result.SQL, "OUTPUT INSERTED.[id]") {
		t.Errorf("SQL = %q, want to contain 'OUTPUT INSERTED.[id]'", result.SQL)
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

	expected := "UPDATE [users] SET [name] = :new_name WHERE [id] = :user_id"
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

	expected := "UPDATE [users] SET [age] = [age] + :increment WHERE [id] = :user_id"
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

	expected := "DELETE FROM [users] WHERE [id] = :user_id"
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_OffsetFetch(t *testing.T) {
	r := New()
	limit := 10
	offset := 20
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "users"},
		Fields:    []types.Field{{Name: "id"}},
		Ordering:  []types.OrderBy{{Field: types.Field{Name: "id"}, Direction: types.ASC}},
		Limit:     &types.PaginationValue{Static: &limit},
		Offset:    &types.PaginationValue{Static: &offset},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	// SQL Server uses OFFSET/FETCH syntax
	if !strings.Contains(result.SQL, "OFFSET 20 ROWS") {
		t.Errorf("SQL = %q, want to contain 'OFFSET 20 ROWS'", result.SQL)
	}
	if !strings.Contains(result.SQL, "FETCH NEXT 10 ROWS ONLY") {
		t.Errorf("SQL = %q, want to contain 'FETCH NEXT 10 ROWS ONLY'", result.SQL)
	}
}

func TestRender_LimitWithoutOrderBy_RejectsError(t *testing.T) {
	r := New()
	limit := 10
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "users"},
		Fields:    []types.Field{{Name: "id"}},
		Limit:     &types.PaginationValue{Static: &limit},
	}

	_, err := r.Render(ast)
	if err == nil {
		t.Fatal("expected error for LIMIT without ORDER BY")
	}
	if !strings.Contains(err.Error(), "ORDER BY") {
		t.Errorf("error = %q, want to mention ORDER BY", err.Error())
	}
}

func TestRender_RejectsOnConflict(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpInsert,
		Target:    types.Table{Name: "users"},
		Values: []map[types.Field]types.Param{
			{
				{Name: "id"}:   {Name: "id_val"},
				{Name: "name"}: {Name: "name_val"},
			},
		},
		OnConflict: &types.ConflictClause{
			Columns: []types.Field{{Name: "id"}},
			Action:  types.DoNothing,
		},
	}

	_, err := r.Render(ast)
	if err == nil {
		t.Fatal("expected error for ON CONFLICT")
	}
	if !strings.Contains(err.Error(), "ON CONFLICT") && !strings.Contains(err.Error(), "upsert") {
		t.Errorf("error = %q, want to mention ON CONFLICT or upsert", err.Error())
	}
}

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
		t.Fatal("expected error for DISTINCT ON")
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
		t.Fatal("expected error for ILIKE")
	}
}

func TestRender_RejectsRowLocking(t *testing.T) {
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
		t.Fatal("expected error for row locking")
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

	expected := "(SELECT [id] FROM [users]) UNION (SELECT [id] FROM [admins])"
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

	expected := "SELECT CONCAT([first_name], [last_name]) AS [full_name] FROM [users]"
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_StringLength(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "users"},
		FieldExpressions: []types.FieldExpression{
			{
				String: &types.StringExpression{
					Function: types.StringLength,
					Field:    types.Field{Name: "name"},
				},
				Alias: "name_len",
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	// SQL Server uses LEN()
	expected := "SELECT LEN([name]) AS [name_len] FROM [users]"
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

	// SQL Server uses GETDATE()
	expected := "SELECT GETDATE() AS [current_time] FROM [users]"
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

	// SQL Server uses DATEPART
	expected := "SELECT DATEPART(YEAR, [created_at]) AS [year] FROM [users]"
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_DateTruncMonth(t *testing.T) {
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

	// SQL Server uses DATEFROMPARTS for month truncation
	if !strings.Contains(result.SQL, "DATEFROMPARTS") {
		t.Errorf("SQL = %q, want to contain 'DATEFROMPARTS'", result.SQL)
	}
}

func TestRender_WindowFunction(t *testing.T) {
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

func TestRender_NotEqual(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "users"},
		Fields:    []types.Field{{Name: "id"}},
		WhereClause: types.Condition{
			Field:    types.Field{Name: "status"},
			Operator: types.NE,
			Value:    types.Param{Name: "status"},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	// SQL Server prefers <> over !=
	if !strings.Contains(result.SQL, "<>") {
		t.Errorf("SQL = %q, want to contain '<>'", result.SQL)
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

	expected := "SELECT COUNT(*) FROM [users]"
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

	expected := "SELECT SUM([total]) AS [total_sum] FROM [orders]"
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

	expected := "SELECT [user_id], COUNT(*) AS [order_count] FROM [orders] GROUP BY [user_id]"
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_CaseExpression(t *testing.T) {
	r := New()
	elseVal := types.Param{Name: "default_val"}
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "users"},
		FieldExpressions: []types.FieldExpression{
			{
				Case: &types.CaseExpression{
					WhenClauses: []types.WhenClause{
						{
							Condition: types.Condition{
								Field:    types.Field{Name: "status"},
								Operator: types.EQ,
								Value:    types.Param{Name: "active_status"},
							},
							Result: types.Param{Name: "active_label"},
						},
					},
					ElseValue: &elseVal,
				},
				Alias: "status_label",
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if !strings.Contains(result.SQL, "CASE WHEN") {
		t.Errorf("SQL = %q, want to contain 'CASE WHEN'", result.SQL)
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

	if !strings.Contains(result.SQL, "COALESCE") {
		t.Errorf("SQL = %q, want to contain 'COALESCE'", result.SQL)
	}
}

func TestRender_BetweenCondition(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "products"},
		Fields:    []types.Field{{Name: "name"}},
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

	expected := "SELECT [name] FROM [products] WHERE [price] BETWEEN :min_price AND :max_price"
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
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

	if !strings.Contains(result.SQL, "OR") {
		t.Errorf("SQL = %q, want to contain 'OR'", result.SQL)
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

	if !strings.Contains(result.SQL, "IN (SELECT") {
		t.Errorf("SQL = %q, want to contain 'IN (SELECT'", result.SQL)
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

	if !strings.Contains(result.SQL, "HAVING COUNT(*)") {
		t.Errorf("SQL = %q, want to contain 'HAVING COUNT(*)'", result.SQL)
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

	if !strings.Contains(result.SQL, "INNER JOIN") {
		t.Errorf("SQL = %q, want to contain 'INNER JOIN'", result.SQL)
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

	expected := "SELECT FLOOR([price]) AS [floor_price] FROM [products]"
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

	if !strings.Contains(result.SQL, "NULLIF") {
		t.Errorf("SQL = %q, want to contain 'NULLIF'", result.SQL)
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

	expected := "SELECT [id] FROM [users] WHERE [deleted_at] IS NULL"
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

	expected := "SELECT [id] FROM [users] WHERE [email] IS NOT NULL"
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
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

	expected := "(SELECT [id] FROM [users]) UNION ALL (SELECT [id] FROM [admins])"
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

	expected := "(SELECT [id] FROM [users]) INTERSECT (SELECT [id] FROM [active_users])"
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

	expected := "(SELECT [id] FROM [users]) EXCEPT (SELECT [id] FROM [banned_users])"
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

	// MSSQL uses OFFSET/FETCH syntax for pagination
	expected := "(SELECT [id], [name] FROM [users]) UNION (SELECT [id], [name] FROM [admins]) ORDER BY [name] ASC OFFSET 0 ROWS FETCH NEXT 10 ROWS ONLY"
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

	expected := "(SELECT [id] FROM [users]) UNION (SELECT [id] FROM [admins]) INTERSECT (SELECT [id] FROM [active_users])"
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

	// sqlx converts : to @ for MSSQL
	expected := "(SELECT [id] FROM [users] WHERE [active] = :q0_is_active) UNION (SELECT [id] FROM [admins] WHERE [active] = :q1_is_active)"
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
	if caps.Upsert {
		t.Error("Upsert should be false")
	}
	if caps.ReturningOnInsert {
		t.Error("ReturningOnInsert should be false")
	}
	if caps.ReturningOnUpdate {
		t.Error("ReturningOnUpdate should be false")
	}
	if caps.ReturningOnDelete {
		t.Error("ReturningOnDelete should be false")
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
	if !caps.InArray {
		t.Error("InArray should be true")
	}
	if caps.RowLocking != render.RowLockingNone {
		t.Errorf("RowLocking = %v, want RowLockingNone", caps.RowLocking)
	}
}

// =============================================================================
// Math Expression Tests
// =============================================================================

func TestRender_MathRound(t *testing.T) {
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

	expected := "SELECT ROUND([price], :precision) AS [rounded_price] FROM [products]"
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_MathCeil(t *testing.T) {
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

	expected := "SELECT CEILING([price]) AS [ceil_price] FROM [products]"
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_MathAbs(t *testing.T) {
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

	expected := "SELECT ABS([amount]) AS [abs_amount] FROM [transactions]"
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_MathPower(t *testing.T) {
	r := New()
	exponent := types.Param{Name: "exp"}
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "calculations"},
		FieldExpressions: []types.FieldExpression{
			{
				Math: &types.MathExpression{
					Function: types.MathPower,
					Field:    types.Field{Name: "base"},
					Exponent: &exponent,
				},
				Alias: "result",
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := "SELECT POWER([base], :exp) AS [result] FROM [calculations]"
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_MathSqrt(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "calculations"},
		FieldExpressions: []types.FieldExpression{
			{
				Math: &types.MathExpression{
					Function: types.MathSqrt,
					Field:    types.Field{Name: "value"},
				},
				Alias: "sqrt_value",
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := "SELECT SQRT([value]) AS [sqrt_value] FROM [calculations]"
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

// =============================================================================
// String Expression Tests
// =============================================================================

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

	expected := "SELECT UPPER([name]) AS [upper_name] FROM [users]"
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

	expected := "SELECT LOWER([email]) AS [lower_email] FROM [users]"
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_StringTrim(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "users"},
		FieldExpressions: []types.FieldExpression{
			{
				String: &types.StringExpression{
					Function: types.StringTrim,
					Field:    types.Field{Name: "name"},
				},
				Alias: "trimmed_name",
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	// MSSQL 2017+ uses TRIM() directly
	expected := "SELECT TRIM([name]) AS [trimmed_name] FROM [users]"
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_StringLTrim(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "users"},
		FieldExpressions: []types.FieldExpression{
			{
				String: &types.StringExpression{
					Function: types.StringLTrim,
					Field:    types.Field{Name: "name"},
				},
				Alias: "ltrimmed_name",
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := "SELECT LTRIM([name]) AS [ltrimmed_name] FROM [users]"
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_StringRTrim(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "users"},
		FieldExpressions: []types.FieldExpression{
			{
				String: &types.StringExpression{
					Function: types.StringRTrim,
					Field:    types.Field{Name: "name"},
				},
				Alias: "rtrimmed_name",
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := "SELECT RTRIM([name]) AS [rtrimmed_name] FROM [users]"
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

	// MSSQL uses SUBSTRING
	expected := "SELECT SUBSTRING([name], :start, :length) AS [substr] FROM [users]"
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

	expected := "SELECT REPLACE([name], :search, :replacement) AS [replaced] FROM [users]"
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

// =============================================================================
// Date Expression Tests
// =============================================================================

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

	// MSSQL uses CAST(GETDATE() AS DATE)
	if !strings.Contains(result.SQL, "GETDATE") {
		t.Errorf("SQL = %q, want to contain 'GETDATE'", result.SQL)
	}
}

func TestRender_DateCurrentTime(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "users"},
		FieldExpressions: []types.FieldExpression{
			{
				Date: &types.DateExpression{
					Function: types.DateCurrentTime,
				},
				Alias: "time_now",
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	// MSSQL uses CAST(GETDATE() AS TIME)
	if !strings.Contains(result.SQL, "GETDATE") {
		t.Errorf("SQL = %q, want to contain 'GETDATE'", result.SQL)
	}
}

func TestRender_DateCurrentTimestamp(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "users"},
		FieldExpressions: []types.FieldExpression{
			{
				Date: &types.DateExpression{
					Function: types.DateCurrentTimestamp,
				},
				Alias: "ts",
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	// MSSQL uses GETDATE() for timestamp
	expected := "SELECT GETDATE() AS [ts] FROM [users]"
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_DateTruncYear(t *testing.T) {
	r := New()
	createdAt := types.Field{Name: "created_at"}
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "users"},
		FieldExpressions: []types.FieldExpression{
			{
				Date: &types.DateExpression{
					Function: types.DateTrunc,
					Part:     types.PartYear,
					Field:    &createdAt,
				},
				Alias: "year_start",
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	// MSSQL uses DATEFROMPARTS for year truncation
	if !strings.Contains(result.SQL, "DATEFROMPARTS") {
		t.Errorf("SQL = %q, want to contain 'DATEFROMPARTS'", result.SQL)
	}
}

func TestRender_DateTruncDay(t *testing.T) {
	r := New()
	createdAt := types.Field{Name: "created_at"}
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "users"},
		FieldExpressions: []types.FieldExpression{
			{
				Date: &types.DateExpression{
					Function: types.DateTrunc,
					Part:     types.PartDay,
					Field:    &createdAt,
				},
				Alias: "day_start",
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	// MSSQL uses CAST AS DATE for day truncation
	if !strings.Contains(result.SQL, "DATE") {
		t.Errorf("SQL = %q, want to contain DATE cast", result.SQL)
	}
}

func TestRender_DateExtractMonth(t *testing.T) {
	r := New()
	createdAt := types.Field{Name: "created_at"}
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "users"},
		FieldExpressions: []types.FieldExpression{
			{
				Date: &types.DateExpression{
					Function: types.DateExtract,
					Part:     types.PartMonth,
					Field:    &createdAt,
				},
				Alias: "month",
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := "SELECT DATEPART(MONTH, [created_at]) AS [month] FROM [users]"
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_DateExtractDay(t *testing.T) {
	r := New()
	createdAt := types.Field{Name: "created_at"}
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "users"},
		FieldExpressions: []types.FieldExpression{
			{
				Date: &types.DateExpression{
					Function: types.DateExtract,
					Part:     types.PartDay,
					Field:    &createdAt,
				},
				Alias: "day",
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := "SELECT DATEPART(DAY, [created_at]) AS [day] FROM [users]"
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_DateExtractHour(t *testing.T) {
	r := New()
	createdAt := types.Field{Name: "created_at"}
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "users"},
		FieldExpressions: []types.FieldExpression{
			{
				Date: &types.DateExpression{
					Function: types.DateExtract,
					Part:     types.PartHour,
					Field:    &createdAt,
				},
				Alias: "hour",
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := "SELECT DATEPART(HOUR, [created_at]) AS [hour] FROM [users]"
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

// =============================================================================
// Aggregate Expression Tests
// =============================================================================

func TestRender_AggregateAvg(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "orders"},
		FieldExpressions: []types.FieldExpression{
			{
				Aggregate: types.AggAvg,
				Field:     types.Field{Name: "total"},
				Alias:     "avg_total",
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := "SELECT AVG([total]) AS [avg_total] FROM [orders]"
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_AggregateMin(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "orders"},
		FieldExpressions: []types.FieldExpression{
			{
				Aggregate: types.AggMin,
				Field:     types.Field{Name: "total"},
				Alias:     "min_total",
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := "SELECT MIN([total]) AS [min_total] FROM [orders]"
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_AggregateMax(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "orders"},
		FieldExpressions: []types.FieldExpression{
			{
				Aggregate: types.AggMax,
				Field:     types.Field{Name: "total"},
				Alias:     "max_total",
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := "SELECT MAX([total]) AS [max_total] FROM [orders]"
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

func TestRender_AggregateCountDistinct(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "orders"},
		FieldExpressions: []types.FieldExpression{
			{
				Aggregate: types.AggCountDistinct,
				Field:     types.Field{Name: "user_id"},
				Alias:     "unique_users",
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := "SELECT COUNT(DISTINCT [user_id]) AS [unique_users] FROM [orders]"
	if result.SQL != expected {
		t.Errorf("SQL = %q, want %q", result.SQL, expected)
	}
}

// =============================================================================
// HAVING Aggregate Condition Tests
// =============================================================================

func TestRender_HavingSum(t *testing.T) {
	r := New()
	totalField := types.Field{Name: "total"}
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "orders"},
		Fields:    []types.Field{{Name: "user_id"}},
		FieldExpressions: []types.FieldExpression{
			{
				Aggregate: types.AggSum,
				Field:     types.Field{Name: "total"},
				Alias:     "sum_total",
			},
		},
		GroupBy: []types.Field{{Name: "user_id"}},
		Having: []types.ConditionItem{
			types.AggregateCondition{
				Func:     types.AggSum,
				Field:    &totalField,
				Operator: types.GT,
				Value:    types.Param{Name: "min_sum"},
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if !strings.Contains(result.SQL, "HAVING SUM([total])") {
		t.Errorf("SQL = %q, want to contain 'HAVING SUM([total])'", result.SQL)
	}
}

func TestRender_HavingAvg(t *testing.T) {
	r := New()
	priceField := types.Field{Name: "price"}
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "products"},
		Fields:    []types.Field{{Name: "category"}},
		FieldExpressions: []types.FieldExpression{
			{
				Aggregate: types.AggAvg,
				Field:     types.Field{Name: "price"},
				Alias:     "avg_price",
			},
		},
		GroupBy: []types.Field{{Name: "category"}},
		Having: []types.ConditionItem{
			types.AggregateCondition{
				Func:     types.AggAvg,
				Field:    &priceField,
				Operator: types.LT,
				Value:    types.Param{Name: "max_avg"},
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if !strings.Contains(result.SQL, "HAVING AVG([price])") {
		t.Errorf("SQL = %q, want to contain 'HAVING AVG([price])'", result.SQL)
	}
}

func TestRender_HavingMin(t *testing.T) {
	r := New()
	priceField := types.Field{Name: "price"}
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "products"},
		Fields:    []types.Field{{Name: "category"}},
		FieldExpressions: []types.FieldExpression{
			{
				Aggregate: types.AggMin,
				Field:     types.Field{Name: "price"},
				Alias:     "min_price",
			},
		},
		GroupBy: []types.Field{{Name: "category"}},
		Having: []types.ConditionItem{
			types.AggregateCondition{
				Func:     types.AggMin,
				Field:    &priceField,
				Operator: types.GE,
				Value:    types.Param{Name: "min_threshold"},
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if !strings.Contains(result.SQL, "HAVING MIN([price])") {
		t.Errorf("SQL = %q, want to contain 'HAVING MIN([price])'", result.SQL)
	}
}

func TestRender_HavingMax(t *testing.T) {
	r := New()
	priceField := types.Field{Name: "price"}
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "products"},
		Fields:    []types.Field{{Name: "category"}},
		FieldExpressions: []types.FieldExpression{
			{
				Aggregate: types.AggMax,
				Field:     types.Field{Name: "price"},
				Alias:     "max_price",
			},
		},
		GroupBy: []types.Field{{Name: "category"}},
		Having: []types.ConditionItem{
			types.AggregateCondition{
				Func:     types.AggMax,
				Field:    &priceField,
				Operator: types.LE,
				Value:    types.Param{Name: "max_threshold"},
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if !strings.Contains(result.SQL, "HAVING MAX([price])") {
		t.Errorf("SQL = %q, want to contain 'HAVING MAX([price])'", result.SQL)
	}
}

// =============================================================================
// Window Function Tests
// =============================================================================

func TestRender_WindowRank(t *testing.T) {
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

	if !strings.Contains(result.SQL, "RANK()") {
		t.Errorf("SQL = %q, want to contain 'RANK()'", result.SQL)
	}
}

func TestRender_WindowDenseRank(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "scores"},
		Fields:    []types.Field{{Name: "player_id"}, {Name: "score"}},
		FieldExpressions: []types.FieldExpression{
			{
				Window: &types.WindowExpression{
					Function: types.WinDenseRank,
					Window: types.WindowSpec{
						OrderBy: []types.OrderBy{{Field: types.Field{Name: "score"}, Direction: types.DESC}},
					},
				},
				Alias: "dense_rank",
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if !strings.Contains(result.SQL, "DENSE_RANK()") {
		t.Errorf("SQL = %q, want to contain 'DENSE_RANK()'", result.SQL)
	}
}

func TestRender_WindowNtile(t *testing.T) {
	r := New()
	ntileParam := types.Param{Name: "buckets"}
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "scores"},
		Fields:    []types.Field{{Name: "player_id"}},
		FieldExpressions: []types.FieldExpression{
			{
				Window: &types.WindowExpression{
					Function:   types.WinNtile,
					NtileParam: &ntileParam,
					Window: types.WindowSpec{
						OrderBy: []types.OrderBy{{Field: types.Field{Name: "score"}, Direction: types.DESC}},
					},
				},
				Alias: "quartile",
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if !strings.Contains(result.SQL, "NTILE") {
		t.Errorf("SQL = %q, want to contain 'NTILE'", result.SQL)
	}
}

func TestRender_WindowLag(t *testing.T) {
	r := New()
	offset := types.Param{Name: "offset"}
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "sales"},
		Fields:    []types.Field{{Name: "date"}, {Name: "amount"}},
		FieldExpressions: []types.FieldExpression{
			{
				Window: &types.WindowExpression{
					Function:  types.WinLag,
					Field:     &types.Field{Name: "amount"},
					LagOffset: &offset,
					Window: types.WindowSpec{
						OrderBy: []types.OrderBy{{Field: types.Field{Name: "date"}, Direction: types.ASC}},
					},
				},
				Alias: "prev_amount",
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if !strings.Contains(result.SQL, "LAG") {
		t.Errorf("SQL = %q, want to contain 'LAG'", result.SQL)
	}
}

func TestRender_WindowLead(t *testing.T) {
	r := New()
	offset := types.Param{Name: "offset"}
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "sales"},
		Fields:    []types.Field{{Name: "date"}, {Name: "amount"}},
		FieldExpressions: []types.FieldExpression{
			{
				Window: &types.WindowExpression{
					Function:  types.WinLead,
					Field:     &types.Field{Name: "amount"},
					LagOffset: &offset, // LagOffset is reused for both Lag and Lead
					Window: types.WindowSpec{
						OrderBy: []types.OrderBy{{Field: types.Field{Name: "date"}, Direction: types.ASC}},
					},
				},
				Alias: "next_amount",
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if !strings.Contains(result.SQL, "LEAD") {
		t.Errorf("SQL = %q, want to contain 'LEAD'", result.SQL)
	}
}

func TestRender_WindowFirstValue(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "sales"},
		Fields:    []types.Field{{Name: "category"}, {Name: "amount"}},
		FieldExpressions: []types.FieldExpression{
			{
				Window: &types.WindowExpression{
					Function: types.WinFirstValue,
					Field:    &types.Field{Name: "amount"},
					Window: types.WindowSpec{
						PartitionBy: []types.Field{{Name: "category"}},
						OrderBy:     []types.OrderBy{{Field: types.Field{Name: "date"}, Direction: types.ASC}},
					},
				},
				Alias: "first_amount",
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if !strings.Contains(result.SQL, "FIRST_VALUE") {
		t.Errorf("SQL = %q, want to contain 'FIRST_VALUE'", result.SQL)
	}
}

func TestRender_WindowLastValue(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "sales"},
		Fields:    []types.Field{{Name: "category"}, {Name: "amount"}},
		FieldExpressions: []types.FieldExpression{
			{
				Window: &types.WindowExpression{
					Function: types.WinLastValue,
					Field:    &types.Field{Name: "amount"},
					Window: types.WindowSpec{
						PartitionBy: []types.Field{{Name: "category"}},
						OrderBy:     []types.OrderBy{{Field: types.Field{Name: "date"}, Direction: types.ASC}},
					},
				},
				Alias: "last_amount",
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if !strings.Contains(result.SQL, "LAST_VALUE") {
		t.Errorf("SQL = %q, want to contain 'LAST_VALUE'", result.SQL)
	}
}

func TestRender_WindowSumOver(t *testing.T) {
	r := New()
	field := types.Field{Name: "amount"}
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "sales"},
		Fields:    []types.Field{{Name: "category"}, {Name: "amount"}},
		FieldExpressions: []types.FieldExpression{
			{
				Window: &types.WindowExpression{
					Aggregate: types.AggSum,
					Field:     &field,
					Window: types.WindowSpec{
						PartitionBy: []types.Field{{Name: "category"}},
					},
				},
				Alias: "category_total",
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if !strings.Contains(result.SQL, "SUM([amount]) OVER") {
		t.Errorf("SQL = %q, want to contain 'SUM([amount]) OVER'", result.SQL)
	}
}

// =============================================================================
// Count Operation Tests
// =============================================================================

func TestRender_CountWithJoin(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpCount,
		Target:    types.Table{Name: "orders", Alias: "o"},
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
		WhereClause: types.Condition{
			Field:    types.Field{Name: "active", Table: "u"},
			Operator: types.EQ,
			Value:    types.Param{Name: "is_active"},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if !strings.Contains(result.SQL, "COUNT(*)") {
		t.Errorf("SQL = %q, want to contain 'COUNT(*)'", result.SQL)
	}
	if !strings.Contains(result.SQL, "INNER JOIN") {
		t.Errorf("SQL = %q, want to contain 'INNER JOIN'", result.SQL)
	}
}

// =============================================================================
// Cast Type Tests
// =============================================================================

func TestRender_CastToVarchar(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "data"},
		FieldExpressions: []types.FieldExpression{
			{
				Cast: &types.CastExpression{
					Field:    types.Field{Name: "value"},
					CastType: types.CastText,
				},
				Alias: "text_val",
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	// MSSQL uses VARCHAR or NVARCHAR for text
	if !strings.Contains(result.SQL, "CAST") {
		t.Errorf("SQL = %q, want to contain 'CAST'", result.SQL)
	}
}

func TestRender_CastToInt(t *testing.T) {
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

	if !strings.Contains(result.SQL, "CAST") && !strings.Contains(result.SQL, "INT") {
		t.Errorf("SQL = %q, want to contain CAST and INT", result.SQL)
	}
}

func TestRender_CastToNumeric(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "data"},
		FieldExpressions: []types.FieldExpression{
			{
				Cast: &types.CastExpression{
					Field:    types.Field{Name: "value"},
					CastType: types.CastNumeric,
				},
				Alias: "num_val",
			},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	// MSSQL uses DECIMAL or NUMERIC
	if !strings.Contains(result.SQL, "CAST") {
		t.Errorf("SQL = %q, want to contain 'CAST'", result.SQL)
	}
}

// =============================================================================
// Additional Operator Tests
// =============================================================================

func TestRender_NotIn(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "users"},
		Fields:    []types.Field{{Name: "id"}},
		WhereClause: types.Condition{
			Field:    types.Field{Name: "status"},
			Operator: types.NotIn,
			Value:    types.Param{Name: "excluded_statuses"},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if !strings.Contains(result.SQL, "NOT IN") {
		t.Errorf("SQL = %q, want to contain 'NOT IN'", result.SQL)
	}
}

func TestRender_NotLike(t *testing.T) {
	r := New()
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "users"},
		Fields:    []types.Field{{Name: "id"}},
		WhereClause: types.Condition{
			Field:    types.Field{Name: "name"},
			Operator: types.NotLike,
			Value:    types.Param{Name: "pattern"},
		},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if !strings.Contains(result.SQL, "NOT LIKE") {
		t.Errorf("SQL = %q, want to contain 'NOT LIKE'", result.SQL)
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

	if !strings.Contains(result.SQL, "LEFT JOIN") {
		t.Errorf("SQL = %q, want to contain 'LEFT JOIN'", result.SQL)
	}
}

func TestRender_Operators(t *testing.T) {
	tests := []struct {
		name     string
		operator types.Operator
		want     string
	}{
		{"GT", types.GT, ">"},
		{"GE", types.GE, ">="},
		{"LT", types.LT, "<"},
		{"LE", types.LE, "<="},
		{"LIKE", types.LIKE, "LIKE"},
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

			if !strings.Contains(result.SQL, tt.want) {
				t.Errorf("SQL = %q, want to contain %q", result.SQL, tt.want)
			}
		})
	}
}

// =============================================================================
// Parameterized Pagination Tests
// =============================================================================

func TestRender_ParameterizedLimit(t *testing.T) {
	r := New()
	pageSize := types.Param{Name: "page_size"}
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "users"},
		Fields:    []types.Field{{Name: "id"}},
		Ordering:  []types.OrderBy{{Field: types.Field{Name: "id"}, Direction: types.ASC}},
		Limit:     &types.PaginationValue{Param: &pageSize},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	// MSSQL uses FETCH NEXT with parameter
	if !strings.Contains(result.SQL, ":page_size") {
		t.Errorf("SQL = %q, want to contain ':page_size'", result.SQL)
	}
}

func TestRender_ParameterizedOffset(t *testing.T) {
	r := New()
	offset := types.Param{Name: "offset"}
	ast := &types.AST{
		Operation: types.OpSelect,
		Target:    types.Table{Name: "users"},
		Fields:    []types.Field{{Name: "id"}},
		Ordering:  []types.OrderBy{{Field: types.Field{Name: "id"}, Direction: types.ASC}},
		Offset:    &types.PaginationValue{Param: &offset},
	}

	result, err := r.Render(ast)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if !strings.Contains(result.SQL, "OFFSET :offset ROWS") {
		t.Errorf("SQL = %q, want to contain 'OFFSET :offset ROWS'", result.SQL)
	}
}
