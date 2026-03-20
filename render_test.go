package astql_test

import (
	"strings"
	"testing"

	"github.com/zoobz-io/astql"
	"github.com/zoobz-io/astql/internal/types"
	"github.com/zoobz-io/astql/mariadb"
	"github.com/zoobz-io/astql/mssql"
	"github.com/zoobz-io/astql/postgres"
	"github.com/zoobz-io/astql/sqlite"
	"github.com/zoobz-io/dbml"
)

func createMariaDBRenderer() *mariadb.Renderer {
	return mariadb.New()
}

func createSQLiteRenderer() *sqlite.Renderer {
	return sqlite.New()
}

func createMSSQLRenderer() *mssql.Renderer {
	return mssql.New()
}

func createRenderTestInstance(t *testing.T) *astql.ASTQL {
	t.Helper()

	project := dbml.NewProject("test")

	users := dbml.NewTable("users")
	users.AddColumn(dbml.NewColumn("id", "bigint"))
	users.AddColumn(dbml.NewColumn("username", "varchar"))
	users.AddColumn(dbml.NewColumn("email", "varchar"))
	users.AddColumn(dbml.NewColumn("active", "boolean"))
	users.AddColumn(dbml.NewColumn("age", "int"))
	users.AddColumn(dbml.NewColumn("created_at", "timestamp"))
	project.AddTable(users)

	posts := dbml.NewTable("posts")
	posts.AddColumn(dbml.NewColumn("id", "bigint"))
	posts.AddColumn(dbml.NewColumn("user_id", "bigint"))
	posts.AddColumn(dbml.NewColumn("title", "varchar"))
	posts.AddColumn(dbml.NewColumn("content", "text"))
	posts.AddColumn(dbml.NewColumn("published", "boolean"))
	project.AddTable(posts)

	comments := dbml.NewTable("comments")
	comments.AddColumn(dbml.NewColumn("id", "bigint"))
	comments.AddColumn(dbml.NewColumn("post_id", "bigint"))
	comments.AddColumn(dbml.NewColumn("user_id", "bigint"))
	comments.AddColumn(dbml.NewColumn("body", "text"))
	project.AddTable(comments)

	instance, err := astql.NewFromDBML(project)
	if err != nil {
		t.Fatalf("Failed to create instance: %v", err)
	}
	return instance
}

// Test basic SELECT queries.
func TestRender_Select_AllFields(t *testing.T) {
	instance := createRenderTestInstance(t)

	result, err := astql.Select(instance.T("users")).Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT * FROM "users"`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}

	if len(result.RequiredParams) != 0 {
		t.Errorf("Expected 0 params, got %d", len(result.RequiredParams))
	}
}

func TestRender_Select_SpecificFields(t *testing.T) {
	instance := createRenderTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		Fields(instance.F("id"), instance.F("username"), instance.F("email")).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT "id", "username", "email" FROM "users"`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

func TestRender_Select_WithAlias(t *testing.T) {
	instance := createRenderTestInstance(t)

	result, err := astql.Select(instance.T("users", "u")).
		Fields(instance.F("id"), instance.F("username")).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT "id", "username" FROM "users" u`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

func TestRender_Select_WithWhere(t *testing.T) {
	instance := createRenderTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		Fields(instance.F("id"), instance.F("username")).
		Where(instance.C(instance.F("active"), "=", instance.P("is_active"))).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT "id", "username" FROM "users" WHERE "active" = :is_active`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}

	if len(result.RequiredParams) != 1 || result.RequiredParams[0] != "is_active" {
		t.Errorf("Expected params [is_active], got %v", result.RequiredParams)
	}
}

func TestRender_Select_MultipleWhere(t *testing.T) {
	instance := createRenderTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		Fields(instance.F("id")).
		Where(instance.C(instance.F("active"), "=", instance.P("is_active"))).
		Where(instance.C(instance.F("age"), ">", instance.P("min_age"))).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	// Multiple Where() calls combine with AND
	expected := `SELECT "id" FROM "users" WHERE ("active" = :is_active AND "age" > :min_age)`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}

	if len(result.RequiredParams) != 2 {
		t.Errorf("Expected 2 params, got %d", len(result.RequiredParams))
	}
}

func TestRender_Select_ComplexConditions(t *testing.T) {
	instance := createRenderTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		Fields(instance.F("id")).
		Where(instance.And(
			instance.C(instance.F("active"), "=", instance.P("is_active")),
			instance.Or(
				instance.C(instance.F("age"), ">", instance.P("min_age")),
				instance.C(instance.F("age"), "<", instance.P("max_age")),
			),
		)).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT "id" FROM "users" WHERE ("active" = :is_active AND ("age" > :min_age OR "age" < :max_age))`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}

	if len(result.RequiredParams) != 3 {
		t.Errorf("Expected 3 params, got %d", len(result.RequiredParams))
	}
}

func TestRender_Select_OrderBy(t *testing.T) {
	instance := createRenderTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		Fields(instance.F("id"), instance.F("username")).
		OrderBy(instance.F("username"), "ASC").
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT "id", "username" FROM "users" ORDER BY "username" ASC`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

func TestRender_Select_LimitOffset(t *testing.T) {
	instance := createRenderTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		Fields(instance.F("id")).
		Limit(10).
		Offset(20).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT "id" FROM "users" LIMIT 10 OFFSET 20`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

func TestRender_Select_Distinct(t *testing.T) {
	instance := createRenderTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		Fields(instance.F("username")).
		Distinct().
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT DISTINCT "username" FROM "users"`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

// Test JOIN queries.
func TestRender_Select_InnerJoin(t *testing.T) {
	instance := createRenderTestInstance(t)

	result, err := astql.Select(instance.T("users", "u")).
		Fields(
			instance.WithTable(instance.F("id"), "u"),
			instance.WithTable(instance.F("username"), "u"),
			instance.WithTable(instance.F("title"), "p"),
		).
		InnerJoin(
			instance.T("posts", "p"),
			astql.CF(
				instance.WithTable(instance.F("id"), "u"),
				"=",
				instance.WithTable(instance.F("user_id"), "p"),
			),
		).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT u."id", u."username", p."title" FROM "users" u INNER JOIN "posts" p ON u."id" = p."user_id"`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

func TestRender_Select_LeftJoin(t *testing.T) {
	instance := createRenderTestInstance(t)

	result, err := astql.Select(instance.T("users", "u")).
		Fields(instance.WithTable(instance.F("username"), "u")).
		LeftJoin(
			instance.T("posts", "p"),
			astql.CF(
				instance.WithTable(instance.F("id"), "u"),
				"=",
				instance.WithTable(instance.F("user_id"), "p"),
			),
		).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT u."username" FROM "users" u LEFT JOIN "posts" p ON u."id" = p."user_id"`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

func TestRender_Select_MultipleJoins(t *testing.T) {
	instance := createRenderTestInstance(t)

	result, err := astql.Select(instance.T("users", "u")).
		Fields(
			instance.WithTable(instance.F("username"), "u"),
			instance.WithTable(instance.F("title"), "p"),
			instance.WithTable(instance.F("body"), "c"),
		).
		InnerJoin(
			instance.T("posts", "p"),
			astql.CF(
				instance.WithTable(instance.F("id"), "u"),
				"=",
				instance.WithTable(instance.F("user_id"), "p"),
			),
		).
		InnerJoin(
			instance.T("comments", "c"),
			astql.CF(
				instance.WithTable(instance.F("id"), "p"),
				"=",
				instance.WithTable(instance.F("post_id"), "c"),
			),
		).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT u."username", p."title", c."body" FROM "users" u INNER JOIN "posts" p ON u."id" = p."user_id" INNER JOIN "comments" c ON p."id" = c."post_id"`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

// Test INSERT queries.
func TestRender_Insert_Basic(t *testing.T) {
	instance := createRenderTestInstance(t)

	vm := instance.ValueMap()
	vm[instance.F("username")] = instance.P("username")
	vm[instance.F("email")] = instance.P("email")

	result, err := astql.Insert(instance.T("users")).
		Values(vm).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	// Fields are sorted alphabetically for deterministic output
	expected := `INSERT INTO "users" ("email", "username") VALUES (:email, :username)`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}

	if len(result.RequiredParams) != 2 {
		t.Errorf("Expected 2 params, got %d", len(result.RequiredParams))
	}
}

func TestRender_Insert_WithReturning(t *testing.T) {
	instance := createRenderTestInstance(t)

	vm := instance.ValueMap()
	vm[instance.F("username")] = instance.P("username")

	result, err := astql.Insert(instance.T("users")).
		Values(vm).
		Returning(instance.F("id"), instance.F("created_at")).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `INSERT INTO "users" ("username") VALUES (:username) RETURNING "id", "created_at"`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

func TestRender_Insert_OnConflictDoNothing(t *testing.T) {
	instance := createRenderTestInstance(t)

	vm := instance.ValueMap()
	vm[instance.F("username")] = instance.P("username")
	vm[instance.F("email")] = instance.P("email")

	result, err := astql.Insert(instance.T("users")).
		Values(vm).
		OnConflict(instance.F("email")).DoNothing().
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `INSERT INTO "users" ("email", "username") VALUES (:email, :username) ON CONFLICT ("email") DO NOTHING`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

func TestRender_Insert_OnConflictDoUpdate(t *testing.T) {
	instance := createRenderTestInstance(t)

	vm := instance.ValueMap()
	vm[instance.F("username")] = instance.P("username")
	vm[instance.F("email")] = instance.P("email")

	result, err := astql.Insert(instance.T("users")).
		Values(vm).
		OnConflict(instance.F("email")).
		DoUpdate().
		Set(instance.F("username"), instance.P("new_username")).
		Build().
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `INSERT INTO "users" ("email", "username") VALUES (:email, :username) ON CONFLICT ("email") DO UPDATE SET "username" = :new_username`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

// Test UPDATE queries.
func TestRender_Update_Basic(t *testing.T) {
	instance := createRenderTestInstance(t)

	result, err := astql.Update(instance.T("users")).
		Set(instance.F("username"), instance.P("new_username")).
		Where(instance.C(instance.F("id"), "=", instance.P("user_id"))).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `UPDATE "users" SET "username" = :new_username WHERE "id" = :user_id`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}

	if len(result.RequiredParams) != 2 {
		t.Errorf("Expected 2 params, got %d", len(result.RequiredParams))
	}
}

func TestRender_Update_MultipleFields(t *testing.T) {
	instance := createRenderTestInstance(t)

	result, err := astql.Update(instance.T("users")).
		Set(instance.F("username"), instance.P("new_username")).
		Set(instance.F("email"), instance.P("new_email")).
		Set(instance.F("active"), instance.P("is_active")).
		Where(instance.C(instance.F("id"), "=", instance.P("user_id"))).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	// Fields are sorted alphabetically
	expected := `UPDATE "users" SET "active" = :is_active, "email" = :new_email, "username" = :new_username WHERE "id" = :user_id`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

func TestRender_Update_WithReturning(t *testing.T) {
	instance := createRenderTestInstance(t)

	result, err := astql.Update(instance.T("users")).
		Set(instance.F("active"), instance.P("is_active")).
		Where(instance.C(instance.F("id"), "=", instance.P("user_id"))).
		Returning(instance.F("id"), instance.F("active")).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `UPDATE "users" SET "active" = :is_active WHERE "id" = :user_id RETURNING "id", "active"`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

func TestRender_Update_SetExpr(t *testing.T) {
	instance := createRenderTestInstance(t)

	result, err := astql.Update(instance.T("users")).
		SetExpr(instance.F("age"), types.FieldExpression{
			Binary: &types.BinaryExpression{
				Field:    instance.F("age"),
				Operator: "+",
				Param:    instance.P("increment"),
			},
		}).
		Where(instance.C(instance.F("id"), "=", instance.P("user_id"))).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `UPDATE "users" SET "age" = "age" + :increment WHERE "id" = :user_id`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

func TestRender_Update_SetExpr_WithSet(t *testing.T) {
	instance := createRenderTestInstance(t)

	result, err := astql.Update(instance.T("users")).
		Set(instance.F("username"), instance.P("new_username")).
		SetExpr(instance.F("age"), types.FieldExpression{
			Binary: &types.BinaryExpression{
				Field:    instance.F("age"),
				Operator: "+",
				Param:    instance.P("increment"),
			},
		}).
		Where(instance.C(instance.F("id"), "=", instance.P("user_id"))).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `UPDATE "users" SET "username" = :new_username, "age" = "age" + :increment WHERE "id" = :user_id`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

func TestRender_Update_SetExpr_DuplicateField(t *testing.T) {
	instance := createRenderTestInstance(t)

	_, err := astql.Update(instance.T("users")).
		Set(instance.F("age"), instance.P("new_age")).
		SetExpr(instance.F("age"), types.FieldExpression{
			Binary: &types.BinaryExpression{
				Field:    instance.F("age"),
				Operator: "+",
				Param:    instance.P("increment"),
			},
		}).
		Render(postgres.New())
	if err == nil {
		t.Fatal("Expected error for duplicate field in Set and SetExpr")
	}
	if !strings.Contains(err.Error(), "age") {
		t.Errorf("Expected error to mention 'age', got: %v", err)
	}
}

// Test DELETE queries.
func TestRender_Delete_Basic(t *testing.T) {
	instance := createRenderTestInstance(t)

	result, err := astql.Delete(instance.T("users")).
		Where(instance.C(instance.F("id"), "=", instance.P("user_id"))).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `DELETE FROM "users" WHERE "id" = :user_id`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}

	if len(result.RequiredParams) != 1 {
		t.Errorf("Expected 1 param, got %d", len(result.RequiredParams))
	}
}

func TestRender_Delete_WithReturning(t *testing.T) {
	instance := createRenderTestInstance(t)

	result, err := astql.Delete(instance.T("users")).
		Where(instance.C(instance.F("active"), "=", instance.P("is_active"))).
		Returning(instance.F("id"), instance.F("username")).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `DELETE FROM "users" WHERE "active" = :is_active RETURNING "id", "username"`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

// Test COUNT queries.
func TestRender_Count_Basic(t *testing.T) {
	instance := createRenderTestInstance(t)

	result, err := astql.Count(instance.T("users")).Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT COUNT(*) FROM "users"`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

func TestRender_Count_WithWhere(t *testing.T) {
	instance := createRenderTestInstance(t)

	result, err := astql.Count(instance.T("users")).
		Where(instance.C(instance.F("active"), "=", instance.P("is_active"))).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT COUNT(*) FROM "users" WHERE "active" = :is_active`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

func TestRender_Count_WithInnerJoin(t *testing.T) {
	instance := createRenderTestInstance(t)

	result, err := astql.Count(instance.T("users", "u")).
		InnerJoin(
			instance.T("posts", "p"),
			astql.CF(
				instance.WithTable(instance.F("id"), "u"),
				"=",
				instance.WithTable(instance.F("user_id"), "p"),
			),
		).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT COUNT(*) FROM "users" u INNER JOIN "posts" p ON u."id" = p."user_id"`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

func TestRender_Count_WithCrossJoin(t *testing.T) {
	instance := createRenderTestInstance(t)

	result, err := astql.Count(instance.T("users")).
		CrossJoin(instance.T("posts")).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT COUNT(*) FROM "users" CROSS JOIN "posts"`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

func TestRender_Count_WithJoinAndWhere(t *testing.T) {
	instance := createRenderTestInstance(t)

	result, err := astql.Count(instance.T("users", "u")).
		LeftJoin(
			instance.T("posts", "p"),
			astql.CF(
				instance.WithTable(instance.F("id"), "u"),
				"=",
				instance.WithTable(instance.F("user_id"), "p"),
			),
		).
		Where(instance.C(instance.WithTable(instance.F("active"), "u"), "=", instance.P("is_active"))).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT COUNT(*) FROM "users" u LEFT JOIN "posts" p ON u."id" = p."user_id" WHERE u."active" = :is_active`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

// Test aggregate functions.
func TestRender_Select_Aggregates(t *testing.T) {
	instance := createRenderTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		SelectExpr(astql.Sum(instance.F("age"))).
		SelectExpr(astql.Avg(instance.F("age"))).
		SelectExpr(astql.Min(instance.F("age"))).
		SelectExpr(astql.Max(instance.F("age"))).
		SelectExpr(astql.CountField(instance.F("id"))).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT SUM("age"), AVG("age"), MIN("age"), MAX("age"), COUNT("id") FROM "users"`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

func TestRender_Select_AggregateWithAlias(t *testing.T) {
	instance := createRenderTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		SelectExpr(astql.As(astql.Sum(instance.F("age")), "total_age")).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT SUM("age") AS "total_age" FROM "users"`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

// Test GROUP BY and HAVING.
func TestRender_Select_GroupBy(t *testing.T) {
	instance := createRenderTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		Fields(instance.F("active")).
		SelectExpr(astql.CountField(instance.F("id"))).
		GroupBy(instance.F("active")).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT "active", COUNT("id") FROM "users" GROUP BY "active"`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

func TestRender_Select_GroupByHaving(t *testing.T) {
	instance := createRenderTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		Fields(instance.F("active")).
		SelectExpr(astql.CountField(instance.F("id"))).
		GroupBy(instance.F("active")).
		Having(instance.C(instance.F("active"), "=", instance.P("is_active"))).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT "active", COUNT("id") FROM "users" GROUP BY "active" HAVING "active" = :is_active`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

func TestRender_Select_HavingAgg_Count(t *testing.T) {
	instance := createRenderTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		Fields(instance.F("active")).
		SelectExpr(astql.CountField(instance.F("id"))).
		GroupBy(instance.F("active")).
		HavingAgg(astql.HavingCount(astql.GT, instance.P("min_count"))).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT "active", COUNT("id") FROM "users" GROUP BY "active" HAVING COUNT(*) > :min_count`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

func TestRender_Select_HavingAgg_Sum(t *testing.T) {
	instance := createRenderTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		Fields(instance.F("active")).
		SelectExpr(astql.Sum(instance.F("age"))).
		GroupBy(instance.F("active")).
		HavingAgg(astql.HavingSum(instance.F("age"), astql.GE, instance.P("min_total"))).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT "active", SUM("age") FROM "users" GROUP BY "active" HAVING SUM("age") >= :min_total`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

func TestRender_Select_HavingAgg_Avg(t *testing.T) {
	instance := createRenderTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		Fields(instance.F("active")).
		SelectExpr(astql.Avg(instance.F("age"))).
		GroupBy(instance.F("active")).
		HavingAgg(astql.HavingAvg(instance.F("age"), astql.LT, instance.P("max_avg"))).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT "active", AVG("age") FROM "users" GROUP BY "active" HAVING AVG("age") < :max_avg`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

func TestRender_Select_HavingAgg_CountField(t *testing.T) {
	instance := createRenderTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		Fields(instance.F("active")).
		SelectExpr(astql.CountField(instance.F("id"))).
		GroupBy(instance.F("active")).
		HavingAgg(astql.HavingCountField(instance.F("id"), astql.GT, instance.P("min_count"))).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT "active", COUNT("id") FROM "users" GROUP BY "active" HAVING COUNT("id") > :min_count`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

func TestRender_Select_HavingAgg_CountDistinct(t *testing.T) {
	instance := createRenderTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		Fields(instance.F("active")).
		SelectExpr(astql.CountDistinct(instance.F("email"))).
		GroupBy(instance.F("active")).
		HavingAgg(astql.HavingCountDistinct(instance.F("email"), astql.GE, instance.P("min_unique"))).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT "active", COUNT(DISTINCT "email") FROM "users" GROUP BY "active" HAVING COUNT(DISTINCT "email") >= :min_unique`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

func TestRender_Select_HavingAgg_MinMax(t *testing.T) {
	instance := createRenderTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		Fields(instance.F("active")).
		SelectExpr(astql.Min(instance.F("age"))).
		SelectExpr(astql.Max(instance.F("age"))).
		GroupBy(instance.F("active")).
		HavingAgg(
			astql.HavingMin(instance.F("age"), astql.GE, instance.P("min_age")),
			astql.HavingMax(instance.F("age"), astql.LE, instance.P("max_age")),
		).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT "active", MIN("age"), MAX("age") FROM "users" GROUP BY "active" HAVING MIN("age") >= :min_age AND MAX("age") <= :max_age`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

func TestRender_Select_HavingAgg_MixedConditions(t *testing.T) {
	instance := createRenderTestInstance(t)

	// Mix simple HAVING with aggregate HAVING
	result, err := astql.Select(instance.T("users")).
		Fields(instance.F("active")).
		SelectExpr(astql.CountField(instance.F("id"))).
		GroupBy(instance.F("active")).
		Having(instance.C(instance.F("active"), "=", instance.P("is_active"))).
		HavingAgg(astql.HavingCount(astql.GT, instance.P("min_count"))).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT "active", COUNT("id") FROM "users" GROUP BY "active" HAVING "active" = :is_active AND COUNT(*) > :min_count`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

// Test SQL injection prevention via schema validation.
func TestValidation_InvalidTable(t *testing.T) {
	instance := createRenderTestInstance(t)

	// Try to use a table that doesn't exist in schema
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for invalid table")
		}
	}()

	instance.T("malicious_table")
}

func TestValidation_InvalidField(t *testing.T) {
	instance := createRenderTestInstance(t)

	// Try to use a field that doesn't exist in schema
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for invalid field")
		}
	}()

	instance.F("malicious_field")
}

func TestValidation_SQLInjectionAttempt_Table(t *testing.T) {
	instance := createRenderTestInstance(t)

	// Try SQL injection via table name - should fail at validation
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for SQL injection attempt")
		}
	}()

	instance.T("users; DROP TABLE users--")
}

func TestValidation_SQLInjectionAttempt_Field(t *testing.T) {
	instance := createRenderTestInstance(t)

	// Try SQL injection via field name - should fail at validation
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for SQL injection attempt")
		}
	}()

	instance.F("id; DROP TABLE users--")
}

func TestValidation_SQLInjectionAttempt_Param(t *testing.T) {
	instance := createRenderTestInstance(t)

	// Try SQL injection via parameter name - should fail at validation
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for SQL injection attempt")
		}
	}()

	instance.P("id'; DROP TABLE users--")
}

// Test special characters are properly quoted.
func TestRender_SpecialCharacters_TableName(t *testing.T) {
	project := dbml.NewProject("test")
	// Table name with reserved keyword
	selectTable := dbml.NewTable("select")
	selectTable.AddColumn(dbml.NewColumn("id", "bigint"))
	project.AddTable(selectTable)

	instance, err := astql.NewFromDBML(project)
	if err != nil {
		t.Fatalf("Failed to create instance: %v", err)
	}

	result, err := astql.Select(instance.T("select")).
		Fields(instance.F("id")).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	// Reserved keyword should be quoted
	expected := `SELECT "id" FROM "select"`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

func TestRender_SpecialCharacters_FieldName(t *testing.T) {
	project := dbml.NewProject("test")
	users := dbml.NewTable("users")
	// Field name with reserved keyword
	users.AddColumn(dbml.NewColumn("order", "varchar"))
	project.AddTable(users)

	instance, err := astql.NewFromDBML(project)
	if err != nil {
		t.Fatalf("Failed to create instance: %v", err)
	}

	result, err := astql.Select(instance.T("users")).
		Fields(instance.F("order")).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	// Reserved keyword should be quoted
	expected := `SELECT "order" FROM "users"`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

// Test parameter uniqueness.
func TestRender_DuplicateParameters(t *testing.T) {
	instance := createRenderTestInstance(t)

	// Use same parameter multiple times
	result, err := astql.Select(instance.T("users")).
		Fields(instance.F("id")).
		Where(instance.And(
			instance.C(instance.F("age"), ">", instance.P("age_value")),
			instance.C(instance.F("age"), "<", instance.P("age_value")),
		)).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	// Parameter should only appear once in RequiredParams list
	if len(result.RequiredParams) != 1 {
		t.Errorf("Expected 1 unique param, got %d: %v", len(result.RequiredParams), result.RequiredParams)
	}

	// But should be used twice in SQL
	paramCount := strings.Count(result.SQL, ":age_value")
	if paramCount != 2 {
		t.Errorf("Expected parameter to be used 2 times in SQL, found %d", paramCount)
	}
}

// Test NULL conditions.
func TestRender_NullConditions(t *testing.T) {
	instance := createRenderTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		Fields(instance.F("id")).
		Where(instance.Null(instance.F("email"))).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT "id" FROM "users" WHERE "email" IS NULL`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}

	// NULL condition shouldn't require parameters
	if len(result.RequiredParams) != 0 {
		t.Errorf("Expected 0 params for NULL condition, got %d", len(result.RequiredParams))
	}
}

func TestRender_NotNullConditions(t *testing.T) {
	instance := createRenderTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		Fields(instance.F("id")).
		Where(instance.NotNull(instance.F("email"))).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT "id" FROM "users" WHERE "email" IS NOT NULL`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

// Test complex real-world query.
func TestRender_ComplexQuery(t *testing.T) {
	instance := createRenderTestInstance(t)

	result, err := astql.Select(instance.T("users", "u")).
		Fields(
			instance.WithTable(instance.F("username"), "u"),
			instance.WithTable(instance.F("email"), "u"),
		).
		SelectExpr(astql.As(astql.CountField(instance.WithTable(instance.F("id"), "p")), "post_count")).
		InnerJoin(
			instance.T("posts", "p"),
			astql.CF(
				instance.WithTable(instance.F("id"), "u"),
				"=",
				instance.WithTable(instance.F("user_id"), "p"),
			),
		).
		Where(instance.And(
			instance.C(instance.WithTable(instance.F("active"), "u"), "=", instance.P("is_active")),
			instance.C(instance.WithTable(instance.F("published"), "p"), "=", instance.P("is_published")),
		)).
		GroupBy(
			instance.WithTable(instance.F("id"), "u"),
			instance.WithTable(instance.F("username"), "u"),
			instance.WithTable(instance.F("email"), "u"),
		).
		OrderBy(instance.WithTable(instance.F("username"), "u"), "ASC").
		Limit(10).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT u."username", u."email", COUNT(p."id") AS "post_count" FROM "users" u INNER JOIN "posts" p ON u."id" = p."user_id" WHERE (u."active" = :is_active AND p."published" = :is_published) GROUP BY u."id", u."username", u."email" ORDER BY u."username" ASC LIMIT 10`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}

	if len(result.RequiredParams) != 2 {
		t.Errorf("Expected 2 params, got %d", len(result.RequiredParams))
	}
}

// Test RIGHT JOIN.
func TestRender_Select_RightJoin(t *testing.T) {
	instance := createRenderTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		Fields(instance.F("username"), instance.F("title")).
		RightJoin(
			instance.T("posts"),
			astql.CF(instance.F("id"), "=", instance.F("user_id")),
		).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT "username", "title" FROM "users" RIGHT JOIN "posts" ON "id" = "user_id"`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

// Test CROSS JOIN.
func TestRender_Select_CrossJoin(t *testing.T) {
	instance := createRenderTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		Fields(instance.F("username")).
		CrossJoin(instance.T("posts")).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT "username" FROM "users" CROSS JOIN "posts"`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

// Test COUNT(DISTINCT).
func TestRender_Select_CountDistinct(t *testing.T) {
	instance := createRenderTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		SelectExpr(astql.CountDistinct(instance.F("email"))).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT COUNT(DISTINCT "email") FROM "users"`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

// Test CASE expression.
func TestRender_Select_CaseExpression(t *testing.T) {
	instance := createRenderTestInstance(t)

	caseExpr := astql.Case().
		When(instance.C(instance.F("age"), "<", instance.P("young_age")), instance.P("young")).
		When(instance.C(instance.F("age"), ">=", instance.P("old_age")), instance.P("old")).
		Else(instance.P("middle")).
		As("age_group").
		Build()

	result, err := astql.Select(instance.T("users")).
		Fields(instance.F("username")).
		SelectExpr(caseExpr).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT "username", CASE WHEN "age" < :young_age THEN :young WHEN "age" >= :old_age THEN :old ELSE :middle END AS "age_group" FROM "users"`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}

	// Should have 5 parameters (young_age, young, old_age, old, middle)
	if len(result.RequiredParams) != 5 {
		t.Errorf("Expected 5 params, got %d: %v", len(result.RequiredParams), result.RequiredParams)
	}
}

// Test CASE expression without ELSE.
func TestRender_Select_CaseExpression_NoElse(t *testing.T) {
	instance := createRenderTestInstance(t)

	caseExpr := astql.Case().
		When(instance.C(instance.F("active"), "=", instance.P("is_active")), instance.P("status_active")).
		Build()

	result, err := astql.Select(instance.T("users")).
		SelectExpr(caseExpr).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT CASE WHEN "active" = :is_active THEN :status_active END FROM "users"`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

// Test COALESCE expression.
func TestRender_Select_Coalesce(t *testing.T) {
	instance := createRenderTestInstance(t)

	coalesceExpr := astql.As(
		astql.Coalesce(instance.P("email"), instance.P("username"), instance.P("default_value")),
		"contact",
	)

	result, err := astql.Select(instance.T("users")).
		SelectExpr(coalesceExpr).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT COALESCE(:email, :username, :default_value) AS "contact" FROM "users"`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}

	if len(result.RequiredParams) != 3 {
		t.Errorf("Expected 3 params, got %d", len(result.RequiredParams))
	}
}

// Test NULLIF expression.
func TestRender_Select_NullIf(t *testing.T) {
	instance := createRenderTestInstance(t)

	nullifExpr := astql.As(
		astql.NullIf(instance.P("value"), instance.P("null_value")),
		"safe_value",
	)

	result, err := astql.Select(instance.T("users")).
		SelectExpr(nullifExpr).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT NULLIF(:value, :null_value) AS "safe_value" FROM "users"`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}

	if len(result.RequiredParams) != 2 {
		t.Errorf("Expected 2 params, got %d", len(result.RequiredParams))
	}
}

// Test ROUND math function.
func TestRender_Select_Round(t *testing.T) {
	instance := createRenderTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		SelectExpr(astql.As(astql.Round(instance.F("age"), instance.P("precision")), "rounded_age")).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT ROUND("age", :precision) AS "rounded_age" FROM "users"`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}

	if len(result.RequiredParams) != 1 {
		t.Errorf("Expected 1 param, got %d", len(result.RequiredParams))
	}
}

// Test ROUND without precision.
func TestRender_Select_Round_NoPrecision(t *testing.T) {
	instance := createRenderTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		SelectExpr(astql.Round(instance.F("age"))).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT ROUND("age") FROM "users"`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

// Test FLOOR math function.
func TestRender_Select_Floor(t *testing.T) {
	instance := createRenderTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		SelectExpr(astql.As(astql.Floor(instance.F("age")), "floor_age")).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT FLOOR("age") AS "floor_age" FROM "users"`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

// Test CEIL math function.
func TestRender_Select_Ceil(t *testing.T) {
	instance := createRenderTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		SelectExpr(astql.As(astql.Ceil(instance.F("age")), "ceil_age")).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT CEIL("age") AS "ceil_age" FROM "users"`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

// Test ABS math function.
func TestRender_Select_Abs(t *testing.T) {
	instance := createRenderTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		SelectExpr(astql.As(astql.Abs(instance.F("age")), "abs_age")).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT ABS("age") AS "abs_age" FROM "users"`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

// Test POWER math function.
func TestRender_Select_Power(t *testing.T) {
	instance := createRenderTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		SelectExpr(astql.As(astql.Power(instance.F("age"), instance.P("exponent")), "power_age")).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT POWER("age", :exponent) AS "power_age" FROM "users"`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}

	if len(result.RequiredParams) != 1 {
		t.Errorf("Expected 1 param, got %d", len(result.RequiredParams))
	}
}

// Test SQRT math function.
func TestRender_Select_Sqrt(t *testing.T) {
	instance := createRenderTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		SelectExpr(astql.As(astql.Sqrt(instance.F("age")), "sqrt_age")).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT SQRT("age") AS "sqrt_age" FROM "users"`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

// Test combining multiple math expressions.
func TestRender_Select_MultipleMathExpressions(t *testing.T) {
	instance := createRenderTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		Fields(instance.F("username")).
		SelectExpr(astql.As(astql.Round(instance.F("age")), "rounded")).
		SelectExpr(astql.As(astql.Floor(instance.F("age")), "floored")).
		SelectExpr(astql.As(astql.Ceil(instance.F("age")), "ceiled")).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT "username", ROUND("age") AS "rounded", FLOOR("age") AS "floored", CEIL("age") AS "ceiled" FROM "users"`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

// createVectorTestInstance creates a test instance with vector columns.
func createVectorTestInstance(t *testing.T) *astql.ASTQL {
	t.Helper()

	project := dbml.NewProject("test")

	documents := dbml.NewTable("documents")
	documents.AddColumn(dbml.NewColumn("id", "bigint"))
	documents.AddColumn(dbml.NewColumn("content", "text"))
	documents.AddColumn(dbml.NewColumn("embedding", "vector(1536)"))
	project.AddTable(documents)

	instance, err := astql.NewFromDBML(project)
	if err != nil {
		t.Fatalf("Failed to create instance: %v", err)
	}
	return instance
}

// Test vector L2 distance operator (pgvector <->).
func TestRender_Select_VectorL2Distance(t *testing.T) {
	instance := createVectorTestInstance(t)

	result, err := astql.Select(instance.T("documents")).
		Fields(instance.F("id"), instance.F("content")).
		Where(instance.C(instance.F("embedding"), astql.VectorL2Distance, instance.P("query_embedding"))).
		OrderBy(instance.F("embedding"), astql.ASC).
		Limit(10).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT "id", "content" FROM "documents" WHERE "embedding" <-> :query_embedding ORDER BY "embedding" ASC LIMIT 10`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}

	if len(result.RequiredParams) != 1 || result.RequiredParams[0] != "query_embedding" {
		t.Errorf("Expected params [query_embedding], got %v", result.RequiredParams)
	}
}

// Test vector inner product operator (pgvector <#>).
func TestRender_Select_VectorInnerProduct(t *testing.T) {
	instance := createVectorTestInstance(t)

	result, err := astql.Select(instance.T("documents")).
		Fields(instance.F("id")).
		Where(instance.C(instance.F("embedding"), astql.VectorInnerProduct, instance.P("query"))).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT "id" FROM "documents" WHERE "embedding" <#> :query`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

// Test vector cosine distance operator (pgvector <=>).
func TestRender_Select_VectorCosineDistance(t *testing.T) {
	instance := createVectorTestInstance(t)

	result, err := astql.Select(instance.T("documents")).
		Fields(instance.F("id")).
		Where(instance.C(instance.F("embedding"), astql.VectorCosineDistance, instance.P("query"))).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT "id" FROM "documents" WHERE "embedding" <=> :query`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

// Test vector L1/Manhattan distance operator (pgvector <+>).
func TestRender_Select_VectorL1Distance(t *testing.T) {
	instance := createVectorTestInstance(t)

	result, err := astql.Select(instance.T("documents")).
		Fields(instance.F("id")).
		Where(instance.C(instance.F("embedding"), astql.VectorL1Distance, instance.P("query"))).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT "id" FROM "documents" WHERE "embedding" <+> :query`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

// Test OrderByExpr for vector distance ordering.
func TestRender_Select_OrderByExpr_VectorDistance(t *testing.T) {
	instance := createVectorTestInstance(t)

	result, err := astql.Select(instance.T("documents")).
		Fields(instance.F("id"), instance.F("content")).
		OrderByExpr(instance.F("embedding"), astql.VectorL2Distance, instance.P("query_embedding"), astql.ASC).
		Limit(10).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT "id", "content" FROM "documents" ORDER BY "embedding" <-> :query_embedding ASC LIMIT 10`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}

	if len(result.RequiredParams) != 1 || result.RequiredParams[0] != "query_embedding" {
		t.Errorf("Expected params [query_embedding], got %v", result.RequiredParams)
	}
}

// Test OrderByExpr combined with WHERE for semantic search.
func TestRender_Select_OrderByExpr_WithWhere(t *testing.T) {
	instance := createVectorTestInstance(t)

	result, err := astql.Select(instance.T("documents")).
		Fields(instance.F("id"), instance.F("content")).
		Where(instance.NotNull(instance.F("embedding"))).
		OrderByExpr(instance.F("embedding"), astql.VectorL2Distance, instance.P("query_embedding"), astql.ASC).
		Limit(10).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT "id", "content" FROM "documents" WHERE "embedding" IS NOT NULL ORDER BY "embedding" <-> :query_embedding ASC LIMIT 10`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

// Test OrderByExpr with cosine distance.
func TestRender_Select_OrderByExpr_CosineDistance(t *testing.T) {
	instance := createVectorTestInstance(t)

	result, err := astql.Select(instance.T("documents")).
		Fields(instance.F("id")).
		OrderByExpr(instance.F("embedding"), astql.VectorCosineDistance, instance.P("query"), astql.ASC).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT "id" FROM "documents" ORDER BY "embedding" <=> :query ASC`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

// Test IN operator with parameter (renders as = ANY for PostgreSQL).
func TestRender_Select_InWithParam(t *testing.T) {
	instance := createRenderTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		Fields(instance.F("id"), instance.F("username")).
		Where(instance.C(instance.F("age"), astql.IN, instance.P("ages"))).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	// IN with param renders as = ANY(:param) for PostgreSQL array support
	expected := `SELECT "id", "username" FROM "users" WHERE "age" = ANY(:ages)`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}

	if len(result.RequiredParams) != 1 || result.RequiredParams[0] != "ages" {
		t.Errorf("Expected params [ages], got %v", result.RequiredParams)
	}
}

// Test NOT IN operator with parameter (renders as != ALL for PostgreSQL).
func TestRender_Select_NotInWithParam(t *testing.T) {
	instance := createRenderTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		Fields(instance.F("id"), instance.F("username")).
		Where(instance.C(instance.F("age"), astql.NotIn, instance.P("excluded_ages"))).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	// NOT IN with param renders as != ALL(:param) for PostgreSQL array support
	expected := `SELECT "id", "username" FROM "users" WHERE "age" != ALL(:excluded_ages)`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}

	if len(result.RequiredParams) != 1 || result.RequiredParams[0] != "excluded_ages" {
		t.Errorf("Expected params [excluded_ages], got %v", result.RequiredParams)
	}
}

// Test IN operator combined with other conditions.
func TestRender_Select_InWithOtherConditions(t *testing.T) {
	instance := createRenderTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		Fields(instance.F("id")).
		Where(instance.And(
			instance.C(instance.F("active"), "=", instance.P("is_active")),
			instance.C(instance.F("age"), astql.IN, instance.P("ages")),
		)).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT "id" FROM "users" WHERE ("active" = :is_active AND "age" = ANY(:ages))`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}

	if len(result.RequiredParams) != 2 {
		t.Errorf("Expected 2 params, got %d: %v", len(result.RequiredParams), result.RequiredParams)
	}
}

// createJSONBTestInstance creates a test instance with JSONB columns.
func createJSONBTestInstance(t *testing.T) *astql.ASTQL {
	t.Helper()

	project := dbml.NewProject("test")

	documents := dbml.NewTable("documents")
	documents.AddColumn(dbml.NewColumn("id", "bigint"))
	documents.AddColumn(dbml.NewColumn("content", "text"))
	documents.AddColumn(dbml.NewColumn("embedding", "vector(1536)"))
	documents.AddColumn(dbml.NewColumn("metadata", "jsonb"))
	project.AddTable(documents)

	instance, err := astql.NewFromDBML(project)
	if err != nil {
		t.Fatalf("Failed to create instance: %v", err)
	}
	return instance
}

// Test BinaryExpr renders field <op> param AS alias.
func TestRender_Select_BinaryExpr(t *testing.T) {
	instance := createJSONBTestInstance(t)

	result, err := astql.Select(instance.T("documents")).
		Fields(instance.F("id"), instance.F("content")).
		SelectExpr(astql.As(
			astql.BinaryExpr(instance.F("embedding"), astql.VectorL2Distance, instance.P("query")),
			"distance",
		)).
		OrderByExpr(instance.F("embedding"), astql.VectorL2Distance, instance.P("query"), astql.ASC).
		Limit(10).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT "id", "content", "embedding" <-> :query AS "distance" FROM "documents" ORDER BY "embedding" <-> :query ASC LIMIT 10`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}

	if len(result.RequiredParams) != 1 || result.RequiredParams[0] != "query" {
		t.Errorf("Expected params [query], got %v", result.RequiredParams)
	}
}

// Test SelectBinaryExpr convenience method.
func TestRender_Select_SelectBinaryExpr(t *testing.T) {
	instance := createJSONBTestInstance(t)

	result, err := astql.Select(instance.T("documents")).
		Fields(instance.F("id"), instance.F("content")).
		SelectBinaryExpr(instance.F("embedding"), astql.VectorCosineDistance, instance.P("query_vec"), "score").
		OrderByExpr(instance.F("embedding"), astql.VectorCosineDistance, instance.P("query_vec"), astql.ASC).
		Limit(10).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT "id", "content", "embedding" <=> :query_vec AS "score" FROM "documents" ORDER BY "embedding" <=> :query_vec ASC LIMIT 10`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}

	if len(result.RequiredParams) != 1 || result.RequiredParams[0] != "query_vec" {
		t.Errorf("Expected params [query_vec], got %v", result.RequiredParams)
	}
}

// Test SelectBinaryExpr with multiple vector operators.
func TestRender_Select_SelectBinaryExpr_MultipleDistances(t *testing.T) {
	instance := createJSONBTestInstance(t)

	result, err := astql.Select(instance.T("documents")).
		Fields(instance.F("id")).
		SelectBinaryExpr(instance.F("embedding"), astql.VectorL2Distance, instance.P("query"), "l2_distance").
		SelectBinaryExpr(instance.F("embedding"), astql.VectorCosineDistance, instance.P("query"), "cosine_distance").
		SelectBinaryExpr(instance.F("embedding"), astql.VectorInnerProduct, instance.P("query"), "inner_product").
		Limit(5).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT "id", "embedding" <-> :query AS "l2_distance", "embedding" <=> :query AS "cosine_distance", "embedding" <#> :query AS "inner_product" FROM "documents" LIMIT 5`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

// Test SelectBinaryExpr fails on non-SELECT query.
func TestRender_SelectBinaryExpr_NonSelect(t *testing.T) {
	instance := createJSONBTestInstance(t)

	_, err := astql.Update(instance.T("documents")).
		Set(instance.F("content"), instance.P("new_content")).
		SelectBinaryExpr(instance.F("embedding"), astql.VectorL2Distance, instance.P("query"), "distance").
		Render(postgres.New())

	if err == nil {
		t.Fatal("Expected error for SelectBinaryExpr on UPDATE query")
	}
	if !strings.Contains(err.Error(), "SelectBinaryExpr can only be used with SELECT") {
		t.Errorf("Expected error about SELECT queries, got: %v", err)
	}
}

// Test SelectBinaryExpr fails with invalid alias.
func TestRender_SelectBinaryExpr_InvalidAlias(t *testing.T) {
	instance := createJSONBTestInstance(t)

	_, err := astql.Select(instance.T("documents")).
		Fields(instance.F("id")).
		SelectBinaryExpr(instance.F("embedding"), astql.VectorL2Distance, instance.P("query"), "invalid-alias").
		Render(postgres.New())

	if err == nil {
		t.Fatal("Expected error for invalid alias")
	}
	if !strings.Contains(err.Error(), "invalid alias") {
		t.Errorf("Expected error about invalid alias, got: %v", err)
	}
}

// Test JSONBText renders field->>:key_param (parameterized).
func TestRender_Select_JSONBText(t *testing.T) {
	instance := createJSONBTestInstance(t)

	statusField := instance.JSONBText(instance.F("metadata"), instance.P("status_key"))
	result, err := astql.Select(instance.T("documents")).
		Fields(instance.F("id"), statusField).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT "id", "metadata"->>:status_key FROM "documents"`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}

	if len(result.RequiredParams) != 1 || result.RequiredParams[0] != "status_key" {
		t.Errorf("Expected params [status_key], got %v", result.RequiredParams)
	}
}

// Test JSONBPath renders field->:key_param (parameterized).
func TestRender_Select_JSONBPath(t *testing.T) {
	instance := createJSONBTestInstance(t)

	tagsField := instance.JSONBPath(instance.F("metadata"), instance.P("tags_key"))
	result, err := astql.Select(instance.T("documents")).
		Fields(instance.F("id"), tagsField).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT "id", "metadata"->:tags_key FROM "documents"`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}

	if len(result.RequiredParams) != 1 || result.RequiredParams[0] != "tags_key" {
		t.Errorf("Expected params [tags_key], got %v", result.RequiredParams)
	}
}

// Test JSONBPath with ArrayContains operator.
func TestRender_Select_JSONBPath_ArrayContains(t *testing.T) {
	instance := createJSONBTestInstance(t)

	tagsField := instance.JSONBPath(instance.F("metadata"), instance.P("tags_key"))
	result, err := astql.Select(instance.T("documents")).
		Fields(instance.F("id")).
		Where(instance.C(tagsField, astql.ArrayContains, instance.P("tags_value"))).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT "id" FROM "documents" WHERE "metadata"->:tags_key @> :tags_value`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}

	if len(result.RequiredParams) != 2 {
		t.Errorf("Expected 2 params, got %d: %v", len(result.RequiredParams), result.RequiredParams)
	}
}

// Test JSONBText in WHERE clause.
func TestRender_Select_JSONBText_InWhere(t *testing.T) {
	instance := createJSONBTestInstance(t)

	statusField := instance.JSONBText(instance.F("metadata"), instance.P("status_key"))
	result, err := astql.Select(instance.T("documents")).
		Fields(instance.F("id")).
		Where(instance.C(statusField, "=", instance.P("status_value"))).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT "id" FROM "documents" WHERE "metadata"->>:status_key = :status_value`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}

	if len(result.RequiredParams) != 2 {
		t.Errorf("Expected 2 params, got %d: %v", len(result.RequiredParams), result.RequiredParams)
	}
}

// Test non-postgres renderers error on JSONB fields.
func TestRender_JSONB_NonPostgresError(t *testing.T) {
	instance := createJSONBTestInstance(t)

	statusField := instance.JSONBText(instance.F("metadata"), instance.P("status_key"))
	query := astql.Select(instance.T("documents")).
		Fields(instance.F("id"), statusField)

	// Test with MariaDB renderer
	t.Run("MariaDB", func(t *testing.T) {
		mariadbRenderer := createMariaDBRenderer()
		_, err := query.Render(mariadbRenderer)
		if err == nil {
			t.Fatal("Expected error for JSONB field with MariaDB renderer")
		}
		if !strings.Contains(err.Error(), "JSONB") {
			t.Errorf("Expected error to mention JSONB, got: %v", err)
		}
	})

	// Test with SQLite renderer
	t.Run("SQLite", func(t *testing.T) {
		sqliteRenderer := createSQLiteRenderer()
		_, err := query.Render(sqliteRenderer)
		if err == nil {
			t.Fatal("Expected error for JSONB field with SQLite renderer")
		}
		if !strings.Contains(err.Error(), "JSONB") {
			t.Errorf("Expected error to mention JSONB, got: %v", err)
		}
	})

	// Test with MSSQL renderer
	t.Run("MSSQL", func(t *testing.T) {
		mssqlRenderer := createMSSQLRenderer()
		_, err := query.Render(mssqlRenderer)
		if err == nil {
			t.Fatal("Expected error for JSONB field with MSSQL renderer")
		}
		if !strings.Contains(err.Error(), "JSONB") {
			t.Errorf("Expected error to mention JSONB, got: %v", err)
		}
	})
}

// Test BinaryExpr renders on non-postgres providers (without JSONB fields).
func TestRender_BinaryExpr_NonPostgres(t *testing.T) {
	instance := createJSONBTestInstance(t)

	// Build query with binary expression using a regular field (not JSONB)
	query := astql.Select(instance.T("documents")).
		Fields(instance.F("id")).
		SelectExpr(astql.As(
			astql.BinaryExpr(instance.F("embedding"), astql.EQ, instance.P("value")),
			"result",
		))

	t.Run("MariaDB", func(t *testing.T) {
		result, err := query.Render(createMariaDBRenderer())
		if err != nil {
			t.Fatalf("Render failed: %v", err)
		}
		expected := "SELECT `id`, `embedding` = :value AS `result` FROM `documents`"
		if result.SQL != expected {
			t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
		}
	})

	t.Run("SQLite", func(t *testing.T) {
		result, err := query.Render(createSQLiteRenderer())
		if err != nil {
			t.Fatalf("Render failed: %v", err)
		}
		expected := `SELECT "id", "embedding" = :value AS "result" FROM "documents"`
		if result.SQL != expected {
			t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
		}
	})

	t.Run("MSSQL", func(t *testing.T) {
		result, err := query.Render(createMSSQLRenderer())
		if err != nil {
			t.Fatalf("Render failed: %v", err)
		}
		expected := `SELECT [id], [embedding] = :value AS [result] FROM [documents]`
		if result.SQL != expected {
			t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
		}
	})
}

// Test BinaryExpr with JSONB field errors on non-postgres providers.
func TestRender_BinaryExpr_JSONBField_NonPostgresError(t *testing.T) {
	instance := createJSONBTestInstance(t)

	// Build query with binary expression using a JSONB field
	jsonbField := instance.JSONBText(instance.F("metadata"), instance.P("status_key"))
	query := astql.Select(instance.T("documents")).
		Fields(instance.F("id")).
		SelectExpr(astql.As(
			astql.BinaryExpr(jsonbField, astql.EQ, instance.P("value")),
			"result",
		))

	t.Run("MariaDB", func(t *testing.T) {
		_, err := query.Render(createMariaDBRenderer())
		if err == nil {
			t.Fatal("Expected error for JSONB field in binary expression with MariaDB")
		}
		if !strings.Contains(err.Error(), "JSONB") {
			t.Errorf("Expected error to mention JSONB, got: %v", err)
		}
	})

	t.Run("SQLite", func(t *testing.T) {
		_, err := query.Render(createSQLiteRenderer())
		if err == nil {
			t.Fatal("Expected error for JSONB field in binary expression with SQLite")
		}
		if !strings.Contains(err.Error(), "JSONB") {
			t.Errorf("Expected error to mention JSONB, got: %v", err)
		}
	})

	t.Run("MSSQL", func(t *testing.T) {
		_, err := query.Render(createMSSQLRenderer())
		if err == nil {
			t.Fatal("Expected error for JSONB field in binary expression with MSSQL")
		}
		if !strings.Contains(err.Error(), "JSONB") {
			t.Errorf("Expected error to mention JSONB, got: %v", err)
		}
	})
}

// Test BinaryExpr with unsupported operator errors on non-postgres providers.
func TestRender_BinaryExpr_UnsupportedOperator(t *testing.T) {
	instance := createJSONBTestInstance(t)

	// Build query with binary expression using a vector operator (unsupported on non-postgres)
	query := astql.Select(instance.T("documents")).
		Fields(instance.F("id")).
		SelectExpr(astql.As(
			astql.BinaryExpr(instance.F("embedding"), astql.VectorL2Distance, instance.P("query")),
			"distance",
		))

	t.Run("MariaDB", func(t *testing.T) {
		_, err := query.Render(createMariaDBRenderer())
		if err == nil {
			t.Fatal("Expected error for unsupported vector operator with MariaDB")
		}
	})

	t.Run("SQLite", func(t *testing.T) {
		_, err := query.Render(createSQLiteRenderer())
		if err == nil {
			t.Fatal("Expected error for unsupported vector operator with SQLite")
		}
	})

	t.Run("MSSQL", func(t *testing.T) {
		_, err := query.Render(createMSSQLRenderer())
		if err == nil {
			t.Fatal("Expected error for unsupported vector operator with MSSQL")
		}
	})
}

// Test JSONB in WHERE clause errors on non-postgres providers.
func TestRender_JSONB_InWhere_NonPostgresError(t *testing.T) {
	instance := createJSONBTestInstance(t)

	jsonbField := instance.JSONBText(instance.F("metadata"), instance.P("status_key"))
	query := astql.Select(instance.T("documents")).
		Fields(instance.F("id")).
		Where(instance.C(jsonbField, astql.EQ, instance.P("status_value")))

	t.Run("MariaDB", func(t *testing.T) {
		_, err := query.Render(createMariaDBRenderer())
		if err == nil {
			t.Fatal("Expected error for JSONB field in WHERE with MariaDB")
		}
		if !strings.Contains(err.Error(), "JSONB") {
			t.Errorf("Expected error to mention JSONB, got: %v", err)
		}
	})

	t.Run("SQLite", func(t *testing.T) {
		_, err := query.Render(createSQLiteRenderer())
		if err == nil {
			t.Fatal("Expected error for JSONB field in WHERE with SQLite")
		}
		if !strings.Contains(err.Error(), "JSONB") {
			t.Errorf("Expected error to mention JSONB, got: %v", err)
		}
	})

	t.Run("MSSQL", func(t *testing.T) {
		_, err := query.Render(createMSSQLRenderer())
		if err == nil {
			t.Fatal("Expected error for JSONB field in WHERE with MSSQL")
		}
		if !strings.Contains(err.Error(), "JSONB") {
			t.Errorf("Expected error to mention JSONB, got: %v", err)
		}
	})
}

// Test JSONB in ORDER BY errors on non-postgres providers.
func TestRender_JSONB_InOrderBy_NonPostgresError(t *testing.T) {
	instance := createJSONBTestInstance(t)

	jsonbField := instance.JSONBText(instance.F("metadata"), instance.P("sort_key"))
	query := astql.Select(instance.T("documents")).
		Fields(instance.F("id")).
		OrderBy(jsonbField, astql.ASC)

	t.Run("MariaDB", func(t *testing.T) {
		_, err := query.Render(createMariaDBRenderer())
		if err == nil {
			t.Fatal("Expected error for JSONB field in ORDER BY with MariaDB")
		}
		if !strings.Contains(err.Error(), "JSONB") {
			t.Errorf("Expected error to mention JSONB, got: %v", err)
		}
	})

	t.Run("SQLite", func(t *testing.T) {
		_, err := query.Render(createSQLiteRenderer())
		if err == nil {
			t.Fatal("Expected error for JSONB field in ORDER BY with SQLite")
		}
		if !strings.Contains(err.Error(), "JSONB") {
			t.Errorf("Expected error to mention JSONB, got: %v", err)
		}
	})

	t.Run("MSSQL", func(t *testing.T) {
		_, err := query.Render(createMSSQLRenderer())
		if err == nil {
			t.Fatal("Expected error for JSONB field in ORDER BY with MSSQL")
		}
		if !strings.Contains(err.Error(), "JSONB") {
			t.Errorf("Expected error to mention JSONB, got: %v", err)
		}
	})
}

// Test JSONB in GROUP BY errors on non-postgres providers.
func TestRender_JSONB_InGroupBy_NonPostgresError(t *testing.T) {
	instance := createJSONBTestInstance(t)

	jsonbField := instance.JSONBText(instance.F("metadata"), instance.P("group_key"))
	query := astql.Select(instance.T("documents")).
		Fields(jsonbField).
		GroupBy(jsonbField)

	t.Run("MariaDB", func(t *testing.T) {
		_, err := query.Render(createMariaDBRenderer())
		if err == nil {
			t.Fatal("Expected error for JSONB field in GROUP BY with MariaDB")
		}
		if !strings.Contains(err.Error(), "JSONB") {
			t.Errorf("Expected error to mention JSONB, got: %v", err)
		}
	})

	t.Run("SQLite", func(t *testing.T) {
		_, err := query.Render(createSQLiteRenderer())
		if err == nil {
			t.Fatal("Expected error for JSONB field in GROUP BY with SQLite")
		}
		if !strings.Contains(err.Error(), "JSONB") {
			t.Errorf("Expected error to mention JSONB, got: %v", err)
		}
	})

	t.Run("MSSQL", func(t *testing.T) {
		_, err := query.Render(createMSSQLRenderer())
		if err == nil {
			t.Fatal("Expected error for JSONB field in GROUP BY with MSSQL")
		}
		if !strings.Contains(err.Error(), "JSONB") {
			t.Errorf("Expected error to mention JSONB, got: %v", err)
		}
	})
}

// Test compound query ORDER BY with NULLS FIRST/LAST renders correctly.
func TestRender_CompoundQuery_OrderByNulls(t *testing.T) {
	instance := createRenderTestInstance(t)

	query1 := astql.Select(instance.T("users")).Fields(instance.F("username"))
	query2 := astql.Select(instance.T("users")).Fields(instance.F("username"))

	compound := astql.Union(query1, query2).
		OrderByNulls(instance.F("username"), astql.ASC, astql.NullsLast)

	result, err := compound.Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `(SELECT "username" FROM "users") UNION (SELECT "username" FROM "users") ORDER BY "username" ASC NULLS LAST`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

// Test JSONB field in RETURNING clause for UPDATE.
func TestRender_Update_JSONB_Returning(t *testing.T) {
	instance := createJSONBTestInstance(t)

	jsonbField := instance.JSONBText(instance.F("metadata"), instance.P("status_key"))
	query := astql.Update(instance.T("documents")).
		Set(instance.F("content"), instance.P("new_content")).
		Where(instance.C(instance.F("id"), astql.EQ, instance.P("id"))).
		Returning(instance.F("id"), jsonbField)

	result, err := query.Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `UPDATE "documents" SET "content" = :new_content WHERE "id" = :id RETURNING "id", "metadata"->>:status_key`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}

	// Verify params include the JSONB key
	expectedParams := []string{"new_content", "id", "status_key"}
	if len(result.RequiredParams) != len(expectedParams) {
		t.Errorf("Expected %d params, got %d: %v", len(expectedParams), len(result.RequiredParams), result.RequiredParams)
	}
	for i, p := range expectedParams {
		if result.RequiredParams[i] != p {
			t.Errorf("Expected param[%d] = %s, got %s", i, p, result.RequiredParams[i])
		}
	}
}

// Test JSONB field in RETURNING clause for DELETE.
func TestRender_Delete_JSONB_Returning(t *testing.T) {
	instance := createJSONBTestInstance(t)

	jsonbField := instance.JSONBPath(instance.F("metadata"), instance.P("tags_key"))
	query := astql.Delete(instance.T("documents")).
		Where(instance.C(instance.F("id"), astql.EQ, instance.P("id"))).
		Returning(jsonbField)

	result, err := query.Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `DELETE FROM "documents" WHERE "id" = :id RETURNING "metadata"->:tags_key`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}

	// Verify params include the JSONB key
	expectedParams := []string{"id", "tags_key"}
	if len(result.RequiredParams) != len(expectedParams) {
		t.Errorf("Expected %d params, got %d: %v", len(expectedParams), len(result.RequiredParams), result.RequiredParams)
	}
	for i, p := range expectedParams {
		if result.RequiredParams[i] != p {
			t.Errorf("Expected param[%d] = %s, got %s", i, p, result.RequiredParams[i])
		}
	}
}

// Test JSONB field in RETURNING clause for INSERT.
func TestRender_Insert_JSONB_Returning(t *testing.T) {
	instance := createJSONBTestInstance(t)

	jsonbField := instance.JSONBText(instance.F("metadata"), instance.P("created_key"))
	vm := instance.ValueMap()
	vm[instance.F("content")] = instance.P("content")
	query := astql.Insert(instance.T("documents")).
		Values(vm).
		Returning(instance.F("id"), jsonbField)

	result, err := query.Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `INSERT INTO "documents" ("content") VALUES (:content) RETURNING "id", "metadata"->>:created_key`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}

	// Verify params include the JSONB key
	expectedParams := []string{"content", "created_key"}
	if len(result.RequiredParams) != len(expectedParams) {
		t.Errorf("Expected %d params, got %d: %v", len(expectedParams), len(result.RequiredParams), result.RequiredParams)
	}
	for i, p := range expectedParams {
		if result.RequiredParams[i] != p {
			t.Errorf("Expected param[%d] = %s, got %s", i, p, result.RequiredParams[i])
		}
	}
}

// Test JSONB in Cast expression errors on non-postgres providers.
func TestRender_JSONB_InCast_NonPostgresError(t *testing.T) {
	instance := createJSONBTestInstance(t)

	jsonbField := instance.JSONBText(instance.F("metadata"), instance.P("num_key"))
	query := astql.Select(instance.T("documents")).
		Fields(instance.F("id")).
		SelectExpr(astql.As(
			astql.Cast(jsonbField, astql.CastInteger),
			"num_value",
		))

	t.Run("MariaDB", func(t *testing.T) {
		_, err := query.Render(createMariaDBRenderer())
		if err == nil {
			t.Fatal("Expected error for JSONB field in Cast with MariaDB")
		}
		if !strings.Contains(err.Error(), "JSONB") {
			t.Errorf("Expected error to mention JSONB, got: %v", err)
		}
	})

	t.Run("SQLite", func(t *testing.T) {
		_, err := query.Render(createSQLiteRenderer())
		if err == nil {
			t.Fatal("Expected error for JSONB field in Cast with SQLite")
		}
		if !strings.Contains(err.Error(), "JSONB") {
			t.Errorf("Expected error to mention JSONB, got: %v", err)
		}
	})

	t.Run("MSSQL", func(t *testing.T) {
		_, err := query.Render(createMSSQLRenderer())
		if err == nil {
			t.Fatal("Expected error for JSONB field in Cast with MSSQL")
		}
		if !strings.Contains(err.Error(), "JSONB") {
			t.Errorf("Expected error to mention JSONB, got: %v", err)
		}
	})
}

// Test JSONB in Math expression errors on non-postgres providers.
func TestRender_JSONB_InMath_NonPostgresError(t *testing.T) {
	instance := createJSONBTestInstance(t)

	jsonbField := instance.JSONBText(instance.F("metadata"), instance.P("num_key"))
	query := astql.Select(instance.T("documents")).
		Fields(instance.F("id")).
		SelectExpr(astql.As(
			astql.Round(jsonbField, instance.P("precision")),
			"rounded",
		))

	t.Run("MariaDB", func(t *testing.T) {
		_, err := query.Render(createMariaDBRenderer())
		if err == nil {
			t.Fatal("Expected error for JSONB field in Math with MariaDB")
		}
		if !strings.Contains(err.Error(), "JSONB") {
			t.Errorf("Expected error to mention JSONB, got: %v", err)
		}
	})

	t.Run("SQLite", func(t *testing.T) {
		_, err := query.Render(createSQLiteRenderer())
		if err == nil {
			t.Fatal("Expected error for JSONB field in Math with SQLite")
		}
		if !strings.Contains(err.Error(), "JSONB") {
			t.Errorf("Expected error to mention JSONB, got: %v", err)
		}
	})

	t.Run("MSSQL", func(t *testing.T) {
		_, err := query.Render(createMSSQLRenderer())
		if err == nil {
			t.Fatal("Expected error for JSONB field in Math with MSSQL")
		}
		if !strings.Contains(err.Error(), "JSONB") {
			t.Errorf("Expected error to mention JSONB, got: %v", err)
		}
	})
}

// Test JSONB in String expression errors on non-postgres providers.
func TestRender_JSONB_InString_NonPostgresError(t *testing.T) {
	instance := createJSONBTestInstance(t)

	jsonbField := instance.JSONBText(instance.F("metadata"), instance.P("str_key"))
	query := astql.Select(instance.T("documents")).
		Fields(instance.F("id")).
		SelectExpr(astql.As(
			astql.Upper(jsonbField),
			"upper_val",
		))

	t.Run("MariaDB", func(t *testing.T) {
		_, err := query.Render(createMariaDBRenderer())
		if err == nil {
			t.Fatal("Expected error for JSONB field in String with MariaDB")
		}
		if !strings.Contains(err.Error(), "JSONB") {
			t.Errorf("Expected error to mention JSONB, got: %v", err)
		}
	})

	t.Run("SQLite", func(t *testing.T) {
		_, err := query.Render(createSQLiteRenderer())
		if err == nil {
			t.Fatal("Expected error for JSONB field in String with SQLite")
		}
		if !strings.Contains(err.Error(), "JSONB") {
			t.Errorf("Expected error to mention JSONB, got: %v", err)
		}
	})

	t.Run("MSSQL", func(t *testing.T) {
		_, err := query.Render(createMSSQLRenderer())
		if err == nil {
			t.Fatal("Expected error for JSONB field in String with MSSQL")
		}
		if !strings.Contains(err.Error(), "JSONB") {
			t.Errorf("Expected error to mention JSONB, got: %v", err)
		}
	})
}

// Test JSONB in BETWEEN condition errors on non-postgres providers.
func TestRender_JSONB_InBetween_NonPostgresError(t *testing.T) {
	instance := createJSONBTestInstance(t)

	jsonbField := instance.JSONBText(instance.F("metadata"), instance.P("num_key"))
	query := astql.Select(instance.T("documents")).
		Fields(instance.F("id")).
		Where(astql.Between(jsonbField, instance.P("low"), instance.P("high")))

	t.Run("MariaDB", func(t *testing.T) {
		_, err := query.Render(createMariaDBRenderer())
		if err == nil {
			t.Fatal("Expected error for JSONB field in BETWEEN with MariaDB")
		}
		if !strings.Contains(err.Error(), "JSONB") {
			t.Errorf("Expected error to mention JSONB, got: %v", err)
		}
	})

	t.Run("SQLite", func(t *testing.T) {
		_, err := query.Render(createSQLiteRenderer())
		if err == nil {
			t.Fatal("Expected error for JSONB field in BETWEEN with SQLite")
		}
		if !strings.Contains(err.Error(), "JSONB") {
			t.Errorf("Expected error to mention JSONB, got: %v", err)
		}
	})

	t.Run("MSSQL", func(t *testing.T) {
		_, err := query.Render(createMSSQLRenderer())
		if err == nil {
			t.Fatal("Expected error for JSONB field in BETWEEN with MSSQL")
		}
		if !strings.Contains(err.Error(), "JSONB") {
			t.Errorf("Expected error to mention JSONB, got: %v", err)
		}
	})
}

// Test JSONB in field comparison errors on non-postgres providers.
func TestRender_JSONB_InFieldComparison_NonPostgresError(t *testing.T) {
	instance := createJSONBTestInstance(t)

	jsonbField := instance.JSONBText(instance.F("metadata"), instance.P("key"))
	query := astql.Select(instance.T("documents")).
		Fields(instance.F("id")).
		Where(astql.CF(jsonbField, astql.EQ, instance.F("content")))

	t.Run("MariaDB", func(t *testing.T) {
		_, err := query.Render(createMariaDBRenderer())
		if err == nil {
			t.Fatal("Expected error for JSONB field in FieldComparison with MariaDB")
		}
		if !strings.Contains(err.Error(), "JSONB") {
			t.Errorf("Expected error to mention JSONB, got: %v", err)
		}
	})

	t.Run("SQLite", func(t *testing.T) {
		_, err := query.Render(createSQLiteRenderer())
		if err == nil {
			t.Fatal("Expected error for JSONB field in FieldComparison with SQLite")
		}
		if !strings.Contains(err.Error(), "JSONB") {
			t.Errorf("Expected error to mention JSONB, got: %v", err)
		}
	})

	t.Run("MSSQL", func(t *testing.T) {
		_, err := query.Render(createMSSQLRenderer())
		if err == nil {
			t.Fatal("Expected error for JSONB field in FieldComparison with MSSQL")
		}
		if !strings.Contains(err.Error(), "JSONB") {
			t.Errorf("Expected error to mention JSONB, got: %v", err)
		}
	})
}

// Test JSONB in JOIN ON condition errors on non-postgres providers.
func TestRender_JSONB_InJoinOn_NonPostgresError(t *testing.T) {
	instance := createJSONBTestInstance(t)

	jsonbField := instance.JSONBText(instance.F("metadata"), instance.P("ref_key"))
	query := astql.Select(instance.T("documents")).
		Fields(instance.F("id")).
		LeftJoin(instance.T("documents"),
			instance.C(jsonbField, astql.EQ, instance.P("ref_value")))

	t.Run("MariaDB", func(t *testing.T) {
		_, err := query.Render(createMariaDBRenderer())
		if err == nil {
			t.Fatal("Expected error for JSONB field in JOIN ON with MariaDB")
		}
		if !strings.Contains(err.Error(), "JSONB") {
			t.Errorf("Expected error to mention JSONB, got: %v", err)
		}
	})

	t.Run("SQLite", func(t *testing.T) {
		_, err := query.Render(createSQLiteRenderer())
		if err == nil {
			t.Fatal("Expected error for JSONB field in JOIN ON with SQLite")
		}
		if !strings.Contains(err.Error(), "JSONB") {
			t.Errorf("Expected error to mention JSONB, got: %v", err)
		}
	})

	t.Run("MSSQL", func(t *testing.T) {
		_, err := query.Render(createMSSQLRenderer())
		if err == nil {
			t.Fatal("Expected error for JSONB field in JOIN ON with MSSQL")
		}
		if !strings.Contains(err.Error(), "JSONB") {
			t.Errorf("Expected error to mention JSONB, got: %v", err)
		}
	})
}

// Test JSONB in Window function Field errors on non-postgres providers.
func TestRender_JSONB_InWindowField_NonPostgresError(t *testing.T) {
	instance := createJSONBTestInstance(t)

	jsonbField := instance.JSONBText(instance.F("metadata"), instance.P("amount_key"))
	windowExpr := astql.SumOver(jsonbField).
		PartitionBy(instance.F("id")).
		As("running_total")

	query := astql.Select(instance.T("documents")).
		SelectExpr(windowExpr)

	t.Run("MariaDB", func(t *testing.T) {
		_, err := query.Render(createMariaDBRenderer())
		if err == nil {
			t.Fatal("Expected error for JSONB field in Window expression with MariaDB")
		}
		if !strings.Contains(err.Error(), "JSONB") {
			t.Errorf("Expected error to mention JSONB, got: %v", err)
		}
	})

	t.Run("SQLite", func(t *testing.T) {
		_, err := query.Render(createSQLiteRenderer())
		if err == nil {
			t.Fatal("Expected error for JSONB field in Window expression with SQLite")
		}
		if !strings.Contains(err.Error(), "JSONB") {
			t.Errorf("Expected error to mention JSONB, got: %v", err)
		}
	})

	t.Run("MSSQL", func(t *testing.T) {
		_, err := query.Render(createMSSQLRenderer())
		if err == nil {
			t.Fatal("Expected error for JSONB field in Window expression with MSSQL")
		}
		if !strings.Contains(err.Error(), "JSONB") {
			t.Errorf("Expected error to mention JSONB, got: %v", err)
		}
	})
}

// Test JSONB in Window PartitionBy errors on non-postgres providers.
func TestRender_JSONB_InWindowPartitionBy_NonPostgresError(t *testing.T) {
	instance := createJSONBTestInstance(t)

	jsonbField := instance.JSONBText(instance.F("metadata"), instance.P("group_key"))
	windowExpr := astql.RowNumber().
		PartitionBy(jsonbField).
		OrderBy(instance.F("id"), astql.DESC).
		As("row_num")

	query := astql.Select(instance.T("documents")).
		SelectExpr(windowExpr)

	t.Run("MariaDB", func(t *testing.T) {
		_, err := query.Render(createMariaDBRenderer())
		if err == nil {
			t.Fatal("Expected error for JSONB field in Window PartitionBy with MariaDB")
		}
		if !strings.Contains(err.Error(), "JSONB") {
			t.Errorf("Expected error to mention JSONB, got: %v", err)
		}
	})

	t.Run("SQLite", func(t *testing.T) {
		_, err := query.Render(createSQLiteRenderer())
		if err == nil {
			t.Fatal("Expected error for JSONB field in Window PartitionBy with SQLite")
		}
		if !strings.Contains(err.Error(), "JSONB") {
			t.Errorf("Expected error to mention JSONB, got: %v", err)
		}
	})

	t.Run("MSSQL", func(t *testing.T) {
		_, err := query.Render(createMSSQLRenderer())
		if err == nil {
			t.Fatal("Expected error for JSONB field in Window PartitionBy with MSSQL")
		}
		if !strings.Contains(err.Error(), "JSONB") {
			t.Errorf("Expected error to mention JSONB, got: %v", err)
		}
	})
}

// Test JSONB in Window OrderBy errors on non-postgres providers.
func TestRender_JSONB_InWindowOrderBy_NonPostgresError(t *testing.T) {
	instance := createJSONBTestInstance(t)

	jsonbField := instance.JSONBText(instance.F("metadata"), instance.P("sort_key"))
	windowExpr := astql.RowNumber().
		PartitionBy(instance.F("id")).
		OrderBy(jsonbField, astql.ASC).
		As("row_num")

	query := astql.Select(instance.T("documents")).
		SelectExpr(windowExpr)

	t.Run("MariaDB", func(t *testing.T) {
		_, err := query.Render(createMariaDBRenderer())
		if err == nil {
			t.Fatal("Expected error for JSONB field in Window OrderBy with MariaDB")
		}
		if !strings.Contains(err.Error(), "JSONB") {
			t.Errorf("Expected error to mention JSONB, got: %v", err)
		}
	})

	t.Run("SQLite", func(t *testing.T) {
		_, err := query.Render(createSQLiteRenderer())
		if err == nil {
			t.Fatal("Expected error for JSONB field in Window OrderBy with SQLite")
		}
		if !strings.Contains(err.Error(), "JSONB") {
			t.Errorf("Expected error to mention JSONB, got: %v", err)
		}
	})

	t.Run("MSSQL", func(t *testing.T) {
		_, err := query.Render(createMSSQLRenderer())
		if err == nil {
			t.Fatal("Expected error for JSONB field in Window OrderBy with MSSQL")
		}
		if !strings.Contains(err.Error(), "JSONB") {
			t.Errorf("Expected error to mention JSONB, got: %v", err)
		}
	})
}

// Test JSONB in ConditionGroup (OR) errors on non-postgres providers.
func TestRender_JSONB_InConditionGroup_NonPostgresError(t *testing.T) {
	instance := createJSONBTestInstance(t)

	jsonbField := instance.JSONBText(instance.F("metadata"), instance.P("status_key"))
	orGroup := instance.Or(
		instance.C(instance.F("id"), astql.EQ, instance.P("doc_id")),
		instance.C(jsonbField, astql.EQ, instance.P("status")),
	)

	query := astql.Select(instance.T("documents")).
		Where(orGroup)

	t.Run("MariaDB", func(t *testing.T) {
		_, err := query.Render(createMariaDBRenderer())
		if err == nil {
			t.Fatal("Expected error for JSONB field in ConditionGroup with MariaDB")
		}
		if !strings.Contains(err.Error(), "JSONB") {
			t.Errorf("Expected error to mention JSONB, got: %v", err)
		}
	})

	t.Run("SQLite", func(t *testing.T) {
		_, err := query.Render(createSQLiteRenderer())
		if err == nil {
			t.Fatal("Expected error for JSONB field in ConditionGroup with SQLite")
		}
		if !strings.Contains(err.Error(), "JSONB") {
			t.Errorf("Expected error to mention JSONB, got: %v", err)
		}
	})

	t.Run("MSSQL", func(t *testing.T) {
		_, err := query.Render(createMSSQLRenderer())
		if err == nil {
			t.Fatal("Expected error for JSONB field in ConditionGroup with MSSQL")
		}
		if !strings.Contains(err.Error(), "JSONB") {
			t.Errorf("Expected error to mention JSONB, got: %v", err)
		}
	})
}

// Test JSONB in AggregateCondition (HAVING) errors on non-postgres providers.
func TestRender_JSONB_InAggregateCondition_NonPostgresError(t *testing.T) {
	instance := createJSONBTestInstance(t)

	jsonbField := instance.JSONBText(instance.F("metadata"), instance.P("amount_key"))

	query := astql.Select(instance.T("documents")).
		Fields(instance.F("id")).
		GroupBy(instance.F("id")).
		HavingAgg(astql.AggregateCondition{
			Func:     astql.AggSum,
			Field:    &jsonbField,
			Operator: astql.GT,
			Value:    instance.P("min_total"),
		})

	t.Run("MariaDB", func(t *testing.T) {
		_, err := query.Render(createMariaDBRenderer())
		if err == nil {
			t.Fatal("Expected error for JSONB field in AggregateCondition with MariaDB")
		}
		if !strings.Contains(err.Error(), "JSONB") {
			t.Errorf("Expected error to mention JSONB, got: %v", err)
		}
	})

	t.Run("SQLite", func(t *testing.T) {
		_, err := query.Render(createSQLiteRenderer())
		if err == nil {
			t.Fatal("Expected error for JSONB field in AggregateCondition with SQLite")
		}
		if !strings.Contains(err.Error(), "JSONB") {
			t.Errorf("Expected error to mention JSONB, got: %v", err)
		}
	})

	t.Run("MSSQL", func(t *testing.T) {
		_, err := query.Render(createMSSQLRenderer())
		if err == nil {
			t.Fatal("Expected error for JSONB field in AggregateCondition with MSSQL")
		}
		if !strings.Contains(err.Error(), "JSONB") {
			t.Errorf("Expected error to mention JSONB, got: %v", err)
		}
	})
}
