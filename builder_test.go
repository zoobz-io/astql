package astql_test

import (
	"fmt"
	"testing"

	"github.com/zoobz-io/astql"
	"github.com/zoobz-io/astql/internal/types"
	"github.com/zoobz-io/astql/postgres"
	"github.com/zoobz-io/dbml"
)

func createBuilderTestInstance(t *testing.T) *astql.ASTQL {
	t.Helper()

	project := dbml.NewProject("test")
	users := dbml.NewTable("users")
	users.AddColumn(dbml.NewColumn("id", "bigint"))
	users.AddColumn(dbml.NewColumn("username", "varchar"))
	users.AddColumn(dbml.NewColumn("email", "varchar"))
	users.AddColumn(dbml.NewColumn("age", "int"))
	project.AddTable(users)

	posts := dbml.NewTable("posts")
	posts.AddColumn(dbml.NewColumn("id", "bigint"))
	posts.AddColumn(dbml.NewColumn("user_id", "bigint"))
	posts.AddColumn(dbml.NewColumn("title", "varchar"))
	project.AddTable(posts)

	instance, err := astql.NewFromDBML(project)
	if err != nil {
		t.Fatalf("Failed to create instance: %v", err)
	}
	return instance
}

func TestSelect(t *testing.T) {
	instance := createBuilderTestInstance(t)
	table := instance.T("users")

	builder := astql.Select(table)
	ast, err := builder.Build()

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if ast.Operation != types.OpSelect {
		t.Errorf("Expected SELECT operation, got %v", ast.Operation)
	}
	if ast.Target.Name != "users" {
		t.Errorf("Expected table 'users', got '%s'", ast.Target.Name)
	}
}

func TestInsert(t *testing.T) {
	instance := createBuilderTestInstance(t)
	table := instance.T("users")

	vm := instance.ValueMap()
	vm[instance.F("username")] = instance.P("username")

	builder := astql.Insert(table).Values(vm)
	ast, err := builder.Build()

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if ast.Operation != types.OpInsert {
		t.Errorf("Expected INSERT operation, got %v", ast.Operation)
	}
}

func TestUpdate(t *testing.T) {
	instance := createBuilderTestInstance(t)
	table := instance.T("users")

	builder := astql.Update(table)
	ast := builder.GetAST()

	if ast.Operation != types.OpUpdate {
		t.Errorf("Expected UPDATE operation, got %v", ast.Operation)
	}
	if ast.Updates == nil {
		t.Error("Expected Updates map to be initialized")
	}
}

func TestDelete(t *testing.T) {
	instance := createBuilderTestInstance(t)
	table := instance.T("users")

	builder := astql.Delete(table)
	ast, err := builder.Build()

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if ast.Operation != types.OpDelete {
		t.Errorf("Expected DELETE operation, got %v", ast.Operation)
	}
}

func TestCount(t *testing.T) {
	instance := createBuilderTestInstance(t)
	table := instance.T("users")

	builder := astql.Count(table)
	ast, err := builder.Build()

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if ast.Operation != types.OpCount {
		t.Errorf("Expected COUNT operation, got %v", ast.Operation)
	}
}

func TestFields(t *testing.T) {
	instance := createBuilderTestInstance(t)
	table := instance.T("users")

	builder := astql.Select(table).
		Fields(instance.F("id"), instance.F("username"))

	ast, err := builder.Build()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if len(ast.Fields) != 2 {
		t.Errorf("Expected 2 fields, got %d", len(ast.Fields))
	}
}

func TestFields_WrongOperation(t *testing.T) {
	instance := createBuilderTestInstance(t)
	table := instance.T("users")

	builder := astql.Insert(table).
		Fields(instance.F("id"))

	_, err := builder.Build()
	if err == nil {
		t.Fatal("Expected error when using Fields() with INSERT")
	}
}

func TestWhere(t *testing.T) {
	instance := createBuilderTestInstance(t)
	table := instance.T("users")

	cond := instance.C(instance.F("id"), "=", instance.P("user_id"))
	builder := astql.Select(table).Where(cond)

	ast, err := builder.Build()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if ast.WhereClause == nil {
		t.Error("Expected WHERE clause to be set")
	}
}

func TestWhere_Multiple(t *testing.T) {
	instance := createBuilderTestInstance(t)
	table := instance.T("users")

	cond1 := instance.C(instance.F("id"), "=", instance.P("user_id"))
	cond2 := instance.C(instance.F("username"), "=", instance.P("username"))

	builder := astql.Select(table).
		Where(cond1).
		Where(cond2)

	ast, err := builder.Build()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Multiple Where() calls should combine with AND
	group, ok := ast.WhereClause.(types.ConditionGroup)
	if !ok {
		t.Fatal("Expected ConditionGroup for multiple WHERE clauses")
	}
	if group.Logic != types.AND {
		t.Errorf("Expected AND logic, got %v", group.Logic)
	}
	if len(group.Conditions) != 2 {
		t.Errorf("Expected 2 conditions, got %d", len(group.Conditions))
	}
}

func TestWhereField(t *testing.T) {
	instance := createBuilderTestInstance(t)
	table := instance.T("users")

	builder := astql.Select(table).
		WhereField(instance.F("id"), "=", instance.P("user_id"))

	ast, err := builder.Build()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if ast.WhereClause == nil {
		t.Error("Expected WHERE clause to be set")
	}
}

func TestSet(t *testing.T) {
	instance := createBuilderTestInstance(t)
	table := instance.T("users")

	builder := astql.Update(table).
		Set(instance.F("username"), instance.P("new_username"))

	ast, err := builder.Build()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if len(ast.Updates) != 1 {
		t.Errorf("Expected 1 update, got %d", len(ast.Updates))
	}
}

func TestSet_WrongOperation(t *testing.T) {
	instance := createBuilderTestInstance(t)
	table := instance.T("users")

	builder := astql.Select(table).
		Set(instance.F("username"), instance.P("new_username"))

	_, err := builder.Build()
	if err == nil {
		t.Fatal("Expected error when using Set() with SELECT")
	}
}

func TestSetExpr(t *testing.T) {
	instance := createBuilderTestInstance(t)
	table := instance.T("users")

	builder := astql.Update(table).
		SetExpr(instance.F("age"), types.FieldExpression{
			Binary: &types.BinaryExpression{
				Field:    instance.F("age"),
				Operator: "+",
				Param:    instance.P("increment"),
			},
		})

	ast, err := builder.Build()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if len(ast.UpdateExpressions) != 1 {
		t.Errorf("Expected 1 update expression, got %d", len(ast.UpdateExpressions))
	}
}

func TestSetExpr_PreviousError(t *testing.T) {
	instance := createBuilderTestInstance(t)
	table := instance.T("users")

	// Force an error first via Set on SELECT, then chain SetExpr
	builder := astql.Select(table).
		Set(instance.F("age"), instance.P("val")).
		SetExpr(instance.F("age"), types.FieldExpression{
			Binary: &types.BinaryExpression{
				Field:    instance.F("age"),
				Operator: "+",
				Param:    instance.P("increment"),
			},
		})

	_, err := builder.Build()
	if err == nil {
		t.Fatal("Expected error from previous Set() on SELECT")
	}
}

func TestSetExpr_WrongOperation(t *testing.T) {
	instance := createBuilderTestInstance(t)
	table := instance.T("users")

	builder := astql.Select(table).
		SetExpr(instance.F("age"), types.FieldExpression{
			Binary: &types.BinaryExpression{
				Field:    instance.F("age"),
				Operator: "+",
				Param:    instance.P("increment"),
			},
		})

	_, err := builder.Build()
	if err == nil {
		t.Fatal("Expected error when using SetExpr() with SELECT")
	}
}

func TestValues(t *testing.T) {
	instance := createBuilderTestInstance(t)
	table := instance.T("users")

	vm := instance.ValueMap()
	vm[instance.F("username")] = instance.P("username")
	vm[instance.F("email")] = instance.P("email")

	builder := astql.Insert(table).Values(vm)

	ast, err := builder.Build()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if len(ast.Values) != 1 {
		t.Errorf("Expected 1 value set, got %d", len(ast.Values))
	}
	if len(ast.Values[0]) != 2 {
		t.Errorf("Expected 2 fields in value set, got %d", len(ast.Values[0]))
	}
}

func TestValues_WrongOperation(t *testing.T) {
	instance := createBuilderTestInstance(t)
	table := instance.T("users")

	vm := instance.ValueMap()
	vm[instance.F("username")] = instance.P("username")

	builder := astql.Select(table).Values(vm)

	_, err := builder.Build()
	if err == nil {
		t.Fatal("Expected error when using Values() with SELECT")
	}
}

func TestValues_MultiRow(t *testing.T) {
	instance := createBuilderTestInstance(t)
	table := instance.T("users")

	vm1 := instance.ValueMap()
	vm1[instance.F("username")] = instance.P("user1")
	vm1[instance.F("email")] = instance.P("email1")

	vm2 := instance.ValueMap()
	vm2[instance.F("username")] = instance.P("user2")
	vm2[instance.F("email")] = instance.P("email2")

	builder := astql.Insert(table).Values(vm1).Values(vm2)

	ast, err := builder.Build()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if len(ast.Values) != 2 {
		t.Errorf("Expected 2 value sets, got %d", len(ast.Values))
	}
	if len(ast.Values[0]) != 2 {
		t.Errorf("Expected 2 fields in first value set, got %d", len(ast.Values[0]))
	}
	if len(ast.Values[1]) != 2 {
		t.Errorf("Expected 2 fields in second value set, got %d", len(ast.Values[1]))
	}
}

func TestValues_EmptyMap(t *testing.T) {
	instance := createBuilderTestInstance(t)
	table := instance.T("users")

	vm := instance.ValueMap()

	builder := astql.Insert(table).Values(vm)

	_, err := builder.Build()
	if err == nil {
		t.Fatal("Expected error when using Values() with empty map")
	}
}

func TestValues_SingleRow(t *testing.T) {
	instance := createBuilderTestInstance(t)
	table := instance.T("users")

	vm := instance.ValueMap()
	vm[instance.F("username")] = instance.P("username")

	builder := astql.Insert(table).Values(vm)

	ast, err := builder.Build()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if len(ast.Values) != 1 {
		t.Errorf("Expected 1 value set, got %d", len(ast.Values))
	}
	if len(ast.Values[0]) != 1 {
		t.Errorf("Expected 1 field in value set, got %d", len(ast.Values[0]))
	}
}

func TestOrderBy(t *testing.T) {
	instance := createBuilderTestInstance(t)
	table := instance.T("users")

	builder := astql.Select(table).
		OrderBy(instance.F("username"), types.ASC)

	ast, err := builder.Build()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if len(ast.Ordering) != 1 {
		t.Errorf("Expected 1 ordering, got %d", len(ast.Ordering))
	}
	if ast.Ordering[0].Direction != types.ASC {
		t.Errorf("Expected ASC direction, got %v", ast.Ordering[0].Direction)
	}
}

func TestLimit(t *testing.T) {
	instance := createBuilderTestInstance(t)
	table := instance.T("users")

	builder := astql.Select(table).Limit(10)

	ast, err := builder.Build()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if ast.Limit == nil || ast.Limit.Static == nil {
		t.Fatal("Expected Limit to be set")
	}
	if *ast.Limit.Static != 10 {
		t.Errorf("Expected limit 10, got %d", *ast.Limit.Static)
	}
}

func TestOffset(t *testing.T) {
	instance := createBuilderTestInstance(t)
	table := instance.T("users")

	builder := astql.Select(table).Offset(20)

	ast, err := builder.Build()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if ast.Offset == nil || ast.Offset.Static == nil {
		t.Fatal("Expected Offset to be set")
	}
	if *ast.Offset.Static != 20 {
		t.Errorf("Expected offset 20, got %d", *ast.Offset.Static)
	}
}

func TestLimitParam(t *testing.T) {
	instance := createBuilderTestInstance(t)
	table := instance.T("users")
	pageSize := instance.P("page_size")

	builder := astql.Select(table).LimitParam(pageSize)

	ast, err := builder.Build()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if ast.Limit == nil || ast.Limit.Param == nil {
		t.Fatal("Expected Limit.Param to be set")
	}
	if ast.Limit.Param.Name != "page_size" {
		t.Errorf("Expected limit param 'page_size', got %q", ast.Limit.Param.Name)
	}
}

func TestOffsetParam(t *testing.T) {
	instance := createBuilderTestInstance(t)
	table := instance.T("users")
	offset := instance.P("offset")

	builder := astql.Select(table).OffsetParam(offset)

	ast, err := builder.Build()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if ast.Offset == nil || ast.Offset.Param == nil {
		t.Fatal("Expected Offset.Param to be set")
	}
	if ast.Offset.Param.Name != "offset" {
		t.Errorf("Expected offset param 'offset', got %q", ast.Offset.Param.Name)
	}
}

func TestMustBuild_Success(t *testing.T) {
	instance := createBuilderTestInstance(t)
	table := instance.T("users")

	builder := astql.Select(table)
	ast := builder.MustBuild()

	if ast == nil {
		t.Fatal("Expected AST, got nil")
	}
}

func TestMustBuild_Panics(t *testing.T) {
	instance := createBuilderTestInstance(t)
	table := instance.T("users")

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic from MustBuild")
		}
	}()

	// Create an invalid builder (Fields on INSERT)
	astql.Insert(table).
		Fields(instance.F("id")).
		MustBuild()
}

func TestErrorPropagation(t *testing.T) {
	instance := createBuilderTestInstance(t)
	table := instance.T("users")

	// Create a builder with an operation error (Fields on INSERT)
	builder := astql.Insert(table).
		Fields(instance.F("id"))

	// Further method calls should not panic even with an error set
	builder.Where(instance.C(instance.F("id"), "=", instance.P("id")))
	builder.Limit(10)

	// Error should be returned on Build()
	_, err := builder.Build()
	if err == nil {
		t.Error("Expected error to be propagated")
	}
}

// Test GetError method.
func TestGetError(t *testing.T) {
	instance := createBuilderTestInstance(t)
	table := instance.T("users")

	// Create a builder with an error
	builder := astql.Insert(table).Fields(instance.F("id"))

	// GetError should return the error
	err := builder.GetError()
	if err == nil {
		t.Error("Expected GetError to return error")
	}
	if err.Error() != "Fields() can only be used with SELECT queries" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

// Test SetError method.
func TestSetError(t *testing.T) {
	instance := createBuilderTestInstance(t)
	table := instance.T("users")

	// Create a valid builder
	builder := astql.Select(table)

	// Manually set an error
	builder.SetError(fmt.Errorf("custom error"))

	// Error should be returned on Build()
	_, err := builder.Build()
	if err == nil {
		t.Error("Expected error to be returned")
	}
	if err.Error() != "custom error" {
		t.Errorf("Expected 'custom error', got: %v", err)
	}
}

// Test MustRender success case.
func TestMustRender_Success(t *testing.T) {
	instance := createBuilderTestInstance(t)

	result := astql.Select(instance.T("users")).
		Fields(instance.F("id")).
		MustRender(postgres.New())

	if result.SQL != `SELECT "id" FROM "users"` {
		t.Errorf("Expected SQL, got: %s", result.SQL)
	}
}

// Test MustRender panics on error.
func TestMustRender_Panics(t *testing.T) {
	instance := createBuilderTestInstance(t)

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected MustRender to panic")
		}
	}()

	// Create invalid builder (Fields on INSERT)
	astql.Insert(instance.T("users")).
		Fields(instance.F("id")).
		MustRender(postgres.New())
}

// Test Join method (wrapper for InnerJoin).
func TestJoin(t *testing.T) {
	instance := createBuilderTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		Fields(instance.F("username")).
		Join(
			instance.T("posts"),
			astql.CF(instance.F("id"), "=", instance.F("user_id")),
		).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT "username" FROM "users" INNER JOIN "posts" ON "id" = "user_id"`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

// =============================================================================
// DISTINCT ON Tests
// =============================================================================

func TestDistinctOn_Single(t *testing.T) {
	instance := createBuilderTestInstance(t)

	result, err := astql.Select(instance.T("posts")).
		DistinctOn(instance.F("user_id")).
		Fields(instance.F("user_id"), instance.F("title")).
		OrderBy(instance.F("user_id"), types.ASC).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT DISTINCT ON ("user_id") "user_id", "title" FROM "posts" ORDER BY "user_id" ASC`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

func TestDistinctOn_Multiple(t *testing.T) {
	instance := createBuilderTestInstance(t)

	result, err := astql.Select(instance.T("posts")).
		DistinctOn(instance.F("user_id"), instance.F("title")).
		Fields(instance.F("user_id"), instance.F("title")).
		OrderBy(instance.F("user_id"), types.ASC).
		OrderBy(instance.F("title"), types.ASC).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT DISTINCT ON ("user_id", "title") "user_id", "title" FROM "posts" ORDER BY "user_id" ASC, "title" ASC`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

// =============================================================================
// FOR UPDATE/FOR SHARE Tests
// =============================================================================

func TestForUpdate(t *testing.T) {
	instance := createBuilderTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		Fields(instance.F("id")).
		Where(instance.C(instance.F("id"), "=", instance.P("user_id"))).
		ForUpdate().
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT "id" FROM "users" WHERE "id" = :user_id FOR UPDATE`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

func TestForNoKeyUpdate(t *testing.T) {
	instance := createBuilderTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		ForNoKeyUpdate().
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT * FROM "users" FOR NO KEY UPDATE`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

func TestForShare(t *testing.T) {
	instance := createBuilderTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		ForShare().
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT * FROM "users" FOR SHARE`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

func TestForKeyShare(t *testing.T) {
	instance := createBuilderTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		ForKeyShare().
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT * FROM "users" FOR KEY SHARE`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

// =============================================================================
// NULLS FIRST/LAST Tests
// =============================================================================

func TestOrderByNulls_First(t *testing.T) {
	instance := createBuilderTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		OrderByNulls(instance.F("age"), types.ASC, astql.NullsFirst).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT * FROM "users" ORDER BY "age" ASC NULLS FIRST`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

func TestOrderByNulls_Last(t *testing.T) {
	instance := createBuilderTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		OrderByNulls(instance.F("age"), types.DESC, astql.NullsLast).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT * FROM "users" ORDER BY "age" DESC NULLS LAST`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

// =============================================================================
// FULL OUTER JOIN Test
// =============================================================================

func TestFullOuterJoin(t *testing.T) {
	instance := createBuilderTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		Fields(instance.F("username")).
		FullOuterJoin(
			instance.T("posts"),
			astql.CF(instance.F("id"), "=", instance.F("user_id")),
		).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT "username" FROM "users" FULL OUTER JOIN "posts" ON "id" = "user_id"`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

// =============================================================================
// Compound Query (UNION/INTERSECT/EXCEPT) Tests
// =============================================================================

func TestCompoundQuery_Union(t *testing.T) {
	instance := createBuilderTestInstance(t)

	query1 := astql.Select(instance.T("users")).
		Fields(instance.F("id"), instance.F("username")).
		Where(instance.C(instance.F("age"), ">", instance.P("min_age")))

	query2 := astql.Select(instance.T("users")).
		Fields(instance.F("id"), instance.F("username")).
		Where(instance.C(instance.F("email"), "LIKE", instance.P("pattern")))

	result, err := astql.Union(query1, query2).Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if result.SQL == "" {
		t.Error("Expected non-empty SQL")
	}

	// Params should be namespaced
	hasQ0 := false
	hasQ1 := false
	for _, p := range result.RequiredParams {
		if p[:3] == "q0_" {
			hasQ0 = true
		}
		if p[:3] == "q1_" {
			hasQ1 = true
		}
	}
	if !hasQ0 || !hasQ1 {
		t.Errorf("Expected namespaced params (q0_, q1_), got: %v", result.RequiredParams)
	}
}

func TestCompoundQuery_UnionAll(t *testing.T) {
	instance := createBuilderTestInstance(t)

	query1 := astql.Select(instance.T("users")).Fields(instance.F("id"))
	query2 := astql.Select(instance.T("users")).Fields(instance.F("id"))

	result, err := astql.UnionAll(query1, query2).Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if result.SQL == "" {
		t.Error("Expected non-empty SQL")
	}
}

func TestCompoundQuery_Intersect(t *testing.T) {
	instance := createBuilderTestInstance(t)

	query1 := astql.Select(instance.T("users")).Fields(instance.F("id"))
	query2 := astql.Select(instance.T("users")).Fields(instance.F("id"))

	result, err := astql.Intersect(query1, query2).Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if result.SQL == "" {
		t.Error("Expected non-empty SQL")
	}
}

func TestCompoundQuery_Except(t *testing.T) {
	instance := createBuilderTestInstance(t)

	query1 := astql.Select(instance.T("users")).Fields(instance.F("id"))
	query2 := astql.Select(instance.T("users")).Fields(instance.F("id"))

	result, err := astql.Except(query1, query2).Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if result.SQL == "" {
		t.Error("Expected non-empty SQL")
	}
}

func TestCompoundQuery_IntersectAll(t *testing.T) {
	instance := createBuilderTestInstance(t)

	query1 := astql.Select(instance.T("users")).Fields(instance.F("id"))
	query2 := astql.Select(instance.T("users")).Fields(instance.F("id"))

	result, err := astql.IntersectAll(query1, query2).Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if result.SQL == "" {
		t.Error("Expected non-empty SQL")
	}
}

func TestCompoundQuery_ExceptAll(t *testing.T) {
	instance := createBuilderTestInstance(t)

	query1 := astql.Select(instance.T("users")).Fields(instance.F("id"))
	query2 := astql.Select(instance.T("users")).Fields(instance.F("id"))

	result, err := astql.ExceptAll(query1, query2).Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if result.SQL == "" {
		t.Error("Expected non-empty SQL")
	}
}

func TestCompoundQuery_WithOrderByLimit(t *testing.T) {
	instance := createBuilderTestInstance(t)

	query1 := astql.Select(instance.T("users")).Fields(instance.F("id"), instance.F("username"))
	query2 := astql.Select(instance.T("users")).Fields(instance.F("id"), instance.F("username"))

	result, err := astql.Union(query1, query2).
		OrderBy(instance.F("username"), types.ASC).
		Limit(10).
		Offset(5).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if result.SQL == "" {
		t.Error("Expected non-empty SQL")
	}
}

func TestCompoundQuery_Chain(t *testing.T) {
	instance := createBuilderTestInstance(t)

	query1 := astql.Select(instance.T("users")).Fields(instance.F("id"))
	query2 := astql.Select(instance.T("users")).Fields(instance.F("id"))
	query3 := astql.Select(instance.T("users")).Fields(instance.F("id"))

	result, err := astql.Union(query1, query2).
		Intersect(query3).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if result.SQL == "" {
		t.Error("Expected non-empty SQL")
	}
}

// Test Builder.Union method (not top-level Union function).
func TestBuilder_Union(t *testing.T) {
	instance := createBuilderTestInstance(t)

	query1 := astql.Select(instance.T("users")).Fields(instance.F("id"))
	query2 := astql.Select(instance.T("users")).Fields(instance.F("id"))

	result, err := query1.Union(query2).Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result.SQL == "" {
		t.Error("Expected non-empty SQL")
	}
}

func TestBuilder_UnionAll(t *testing.T) {
	instance := createBuilderTestInstance(t)

	query1 := astql.Select(instance.T("users")).Fields(instance.F("id"))
	query2 := astql.Select(instance.T("users")).Fields(instance.F("id"))

	result, err := query1.UnionAll(query2).Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result.SQL == "" {
		t.Error("Expected non-empty SQL")
	}
}

func TestBuilder_Intersect(t *testing.T) {
	instance := createBuilderTestInstance(t)

	query1 := astql.Select(instance.T("users")).Fields(instance.F("id"))
	query2 := astql.Select(instance.T("users")).Fields(instance.F("id"))

	result, err := query1.Intersect(query2).Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result.SQL == "" {
		t.Error("Expected non-empty SQL")
	}
}

func TestBuilder_IntersectAll(t *testing.T) {
	instance := createBuilderTestInstance(t)

	query1 := astql.Select(instance.T("users")).Fields(instance.F("id"))
	query2 := astql.Select(instance.T("users")).Fields(instance.F("id"))

	result, err := query1.IntersectAll(query2).Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result.SQL == "" {
		t.Error("Expected non-empty SQL")
	}
}

func TestBuilder_Except(t *testing.T) {
	instance := createBuilderTestInstance(t)

	query1 := astql.Select(instance.T("users")).Fields(instance.F("id"))
	query2 := astql.Select(instance.T("users")).Fields(instance.F("id"))

	result, err := query1.Except(query2).Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result.SQL == "" {
		t.Error("Expected non-empty SQL")
	}
}

func TestBuilder_ExceptAll(t *testing.T) {
	instance := createBuilderTestInstance(t)

	query1 := astql.Select(instance.T("users")).Fields(instance.F("id"))
	query2 := astql.Select(instance.T("users")).Fields(instance.F("id"))

	result, err := query1.ExceptAll(query2).Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result.SQL == "" {
		t.Error("Expected non-empty SQL")
	}
}

// Test CompoundBuilder chained methods.
func TestCompoundBuilder_Union(t *testing.T) {
	instance := createBuilderTestInstance(t)

	query1 := astql.Select(instance.T("users")).Fields(instance.F("id"))
	query2 := astql.Select(instance.T("users")).Fields(instance.F("id"))
	query3 := astql.Select(instance.T("users")).Fields(instance.F("id"))

	result, err := astql.Intersect(query1, query2).Union(query3).Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result.SQL == "" {
		t.Error("Expected non-empty SQL")
	}
}

func TestCompoundBuilder_UnionAll(t *testing.T) {
	instance := createBuilderTestInstance(t)

	query1 := astql.Select(instance.T("users")).Fields(instance.F("id"))
	query2 := astql.Select(instance.T("users")).Fields(instance.F("id"))
	query3 := astql.Select(instance.T("users")).Fields(instance.F("id"))

	result, err := astql.Intersect(query1, query2).UnionAll(query3).Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result.SQL == "" {
		t.Error("Expected non-empty SQL")
	}
}

func TestCompoundBuilder_IntersectAll(t *testing.T) {
	instance := createBuilderTestInstance(t)

	query1 := astql.Select(instance.T("users")).Fields(instance.F("id"))
	query2 := astql.Select(instance.T("users")).Fields(instance.F("id"))
	query3 := astql.Select(instance.T("users")).Fields(instance.F("id"))

	result, err := astql.Union(query1, query2).IntersectAll(query3).Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result.SQL == "" {
		t.Error("Expected non-empty SQL")
	}
}

func TestCompoundBuilder_Except(t *testing.T) {
	instance := createBuilderTestInstance(t)

	query1 := astql.Select(instance.T("users")).Fields(instance.F("id"))
	query2 := astql.Select(instance.T("users")).Fields(instance.F("id"))
	query3 := astql.Select(instance.T("users")).Fields(instance.F("id"))

	result, err := astql.Union(query1, query2).Except(query3).Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result.SQL == "" {
		t.Error("Expected non-empty SQL")
	}
}

func TestCompoundBuilder_ExceptAll(t *testing.T) {
	instance := createBuilderTestInstance(t)

	query1 := astql.Select(instance.T("users")).Fields(instance.F("id"))
	query2 := astql.Select(instance.T("users")).Fields(instance.F("id"))
	query3 := astql.Select(instance.T("users")).Fields(instance.F("id"))

	result, err := astql.Union(query1, query2).ExceptAll(query3).Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result.SQL == "" {
		t.Error("Expected non-empty SQL")
	}
}

func TestCompoundBuilder_MustBuild(t *testing.T) {
	instance := createBuilderTestInstance(t)

	query1 := astql.Select(instance.T("users")).Fields(instance.F("id"))
	query2 := astql.Select(instance.T("users")).Fields(instance.F("id"))

	// Should not panic
	compound := astql.Union(query1, query2).MustBuild()
	if compound == nil {
		t.Error("Expected non-nil compound query")
	}
}

func TestCompoundBuilder_MustRender(t *testing.T) {
	instance := createBuilderTestInstance(t)

	query1 := astql.Select(instance.T("users")).Fields(instance.F("id"))
	query2 := astql.Select(instance.T("users")).Fields(instance.F("id"))

	// Should not panic
	result := astql.Union(query1, query2).MustRender(postgres.New())
	if result.SQL == "" {
		t.Error("Expected non-empty SQL")
	}
}
