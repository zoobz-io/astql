package astql_test

import (
	"strings"
	"testing"

	"github.com/zoobz-io/astql"
	"github.com/zoobz-io/astql/internal/types"
	"github.com/zoobz-io/astql/postgres"
	"github.com/zoobz-io/dbml"
)

func createExpressionsTestInstance(t *testing.T) *astql.ASTQL {
	t.Helper()

	project := dbml.NewProject("test")
	users := dbml.NewTable("users")
	users.AddColumn(dbml.NewColumn("id", "bigint"))
	users.AddColumn(dbml.NewColumn("age", "int"))
	users.AddColumn(dbml.NewColumn("active", "boolean"))
	project.AddTable(users)

	instance, err := astql.NewFromDBML(project)
	if err != nil {
		t.Fatalf("Failed to create instance: %v", err)
	}
	return instance
}

// Test valid alias.
func TestAs_ValidAlias(t *testing.T) {
	instance := createExpressionsTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		SelectExpr(astql.As(astql.Sum(instance.F("age")), "total_age")).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	// Alias should be quoted
	expected := `SELECT SUM("age") AS "total_age" FROM "users"`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

// Test alias with underscores.
func TestAs_AliasWithUnderscores(t *testing.T) {
	instance := createExpressionsTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		SelectExpr(astql.As(astql.Avg(instance.F("age")), "avg_user_age")).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT AVG("age") AS "avg_user_age" FROM "users"`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

// Test SQL injection attempt in alias - semicolon.
func TestAs_SQLInjection_Semicolon(t *testing.T) {
	instance := createExpressionsTestInstance(t)

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for SQL injection attempt in alias")
		} else {
			errMsg := r.(error).Error()
			if !strings.Contains(errMsg, "invalid alias") {
				t.Errorf("Expected 'invalid alias' error, got: %v", errMsg)
			}
		}
	}()

	// Should panic before rendering
	astql.Select(instance.T("users")).
		SelectExpr(astql.As(astql.Sum(instance.F("age")), "total; DROP TABLE users--"))
}

// Test SQL injection attempt in alias - OR clause.
func TestAs_SQLInjection_OrClause(t *testing.T) {
	instance := createExpressionsTestInstance(t)

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for SQL injection attempt in alias")
		}
	}()

	astql.Select(instance.T("users")).
		SelectExpr(astql.As(astql.Sum(instance.F("age")), "total or 1=1"))
}

// Test SQL injection attempt in alias - comment.
func TestAs_SQLInjection_Comment(t *testing.T) {
	instance := createExpressionsTestInstance(t)

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for SQL injection attempt in alias")
		}
	}()

	astql.Select(instance.T("users")).
		SelectExpr(astql.As(astql.Sum(instance.F("age")), "total--"))
}

// Test SQL injection attempt in alias - quote.
func TestAs_SQLInjection_Quote(t *testing.T) {
	instance := createExpressionsTestInstance(t)

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for SQL injection attempt in alias")
		}
	}()

	astql.Select(instance.T("users")).
		SelectExpr(astql.As(astql.Sum(instance.F("age")), "total'"))
}

// Test SQL injection attempt in alias - spaces.
func TestAs_SQLInjection_Spaces(t *testing.T) {
	instance := createExpressionsTestInstance(t)

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for SQL injection attempt with spaces")
		}
	}()

	astql.Select(instance.T("users")).
		SelectExpr(astql.As(astql.Sum(instance.F("age")), "total age"))
}

// Test invalid alias - starts with number.
func TestAs_InvalidAlias_StartsWithNumber(t *testing.T) {
	instance := createExpressionsTestInstance(t)

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for alias starting with number")
		}
	}()

	astql.Select(instance.T("users")).
		SelectExpr(astql.As(astql.Sum(instance.F("age")), "1total"))
}

// Test invalid alias - empty string.
func TestAs_InvalidAlias_Empty(t *testing.T) {
	instance := createExpressionsTestInstance(t)

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for empty alias")
		}
	}()

	astql.Select(instance.T("users")).
		SelectExpr(astql.As(astql.Sum(instance.F("age")), ""))
}

// Test CASE expression with valid alias.
func TestCaseBuilder_As_Valid(t *testing.T) {
	instance := createExpressionsTestInstance(t)

	caseExpr := astql.Case().
		When(instance.C(instance.F("age"), "<", instance.P("young")), instance.P("child")).
		Else(instance.P("adult")).
		As("age_group").
		Build()

	result, err := astql.Select(instance.T("users")).
		SelectExpr(caseExpr).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	// Alias should be quoted
	if !strings.Contains(result.SQL, `AS "age_group"`) {
		t.Errorf("Expected quoted alias in SQL: %s", result.SQL)
	}
}

// Test CASE expression with SQL injection attempt.
func TestCaseBuilder_As_SQLInjection(t *testing.T) {
	instance := createExpressionsTestInstance(t)

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for SQL injection in CASE alias")
		}
	}()

	astql.Case().
		When(instance.C(instance.F("age"), "<", instance.P("young")), instance.P("child")).
		As("group; DROP TABLE users--")
}

// Test COALESCE with valid alias.
func TestCoalesce_As_Valid(t *testing.T) {
	instance := createExpressionsTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		SelectExpr(astql.As(
			astql.Coalesce(instance.P("val1"), instance.P("val2")),
			"safe_value",
		)).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, `AS "safe_value"`) {
		t.Errorf("Expected quoted alias in SQL: %s", result.SQL)
	}
}

// Test multiple expressions with aliases.
func TestAs_MultipleAliases(t *testing.T) {
	instance := createExpressionsTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		SelectExpr(astql.As(astql.Sum(instance.F("age")), "total_age")).
		SelectExpr(astql.As(astql.Avg(instance.F("age")), "avg_age")).
		SelectExpr(astql.As(astql.Max(instance.F("age")), "max_age")).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	// All aliases should be quoted
	expected := `SELECT SUM("age") AS "total_age", AVG("age") AS "avg_age", MAX("age") AS "max_age" FROM "users"`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

func createSubqueryTestInstance(t *testing.T) *astql.ASTQL {
	t.Helper()

	project := dbml.NewProject("test")

	users := dbml.NewTable("users")
	users.AddColumn(dbml.NewColumn("id", "bigint"))
	users.AddColumn(dbml.NewColumn("username", "varchar"))
	users.AddColumn(dbml.NewColumn("email", "varchar"))
	users.AddColumn(dbml.NewColumn("active", "boolean"))
	project.AddTable(users)

	posts := dbml.NewTable("posts")
	posts.AddColumn(dbml.NewColumn("id", "bigint"))
	posts.AddColumn(dbml.NewColumn("user_id", "bigint"))
	posts.AddColumn(dbml.NewColumn("title", "varchar"))
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

// Test basic IN subquery.
func TestSubquery_IN_Basic(t *testing.T) {
	instance := createSubqueryTestInstance(t)

	// SELECT * FROM users WHERE id IN (SELECT user_id FROM posts WHERE published = :is_published)
	subquery := astql.Sub(
		astql.Select(instance.T("posts")).
			Fields(instance.F("user_id")).
			Where(instance.C(instance.F("published"), "=", instance.P("is_published"))),
	)

	result, err := astql.Select(instance.T("users")).
		Where(astql.CSub(instance.F("id"), astql.IN, subquery)).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT * FROM "users" WHERE "id" IN (SELECT "user_id" FROM "posts" WHERE "published" = :sq1_is_published)`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}

	// Should have the subquery parameter with prefix
	if len(result.RequiredParams) != 1 {
		t.Errorf("Expected 1 param, got %d", len(result.RequiredParams))
	}
	if !contains(result.RequiredParams, "sq1_is_published") {
		t.Errorf("Expected param 'sq1_is_published', got %v", result.RequiredParams)
	}
}

// Test NOT IN subquery.
func TestSubquery_NOT_IN(t *testing.T) {
	instance := createSubqueryTestInstance(t)

	// SELECT * FROM users WHERE id NOT IN (SELECT user_id FROM posts)
	subquery := astql.Sub(
		astql.Select(instance.T("posts")).
			Fields(instance.F("user_id")),
	)

	result, err := astql.Select(instance.T("users")).
		Where(astql.CSub(instance.F("id"), astql.NotIn, subquery)).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT * FROM "users" WHERE "id" NOT IN (SELECT "user_id" FROM "posts")`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

// Test EXISTS subquery.
func TestSubquery_EXISTS(t *testing.T) {
	instance := createSubqueryTestInstance(t)

	// SELECT * FROM users WHERE EXISTS (SELECT 1 FROM posts WHERE posts.user_id = users.id)
	subquery := astql.Sub(
		astql.Select(instance.T("posts", "p")).
			Fields(instance.WithTable(instance.F("user_id"), "p")).
			Where(instance.C(
				instance.WithTable(instance.F("user_id"), "p"),
				"=",
				instance.P("user_id"),
			)),
	)

	result, err := astql.Select(instance.T("users", "u")).
		Where(astql.CSubExists(astql.EXISTS, subquery)).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT * FROM "users" u WHERE EXISTS (SELECT p."user_id" FROM "posts" p WHERE p."user_id" = :sq1_user_id)`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

// Test NOT EXISTS subquery.
func TestSubquery_NOT_EXISTS(t *testing.T) {
	instance := createSubqueryTestInstance(t)

	// SELECT * FROM users WHERE NOT EXISTS (SELECT 1 FROM posts WHERE posts.user_id = users.id)
	subquery := astql.Sub(
		astql.Select(instance.T("posts")).
			Fields(instance.F("user_id")),
	)

	result, err := astql.Select(instance.T("users")).
		Where(astql.CSubExists(astql.NotExists, subquery)).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT * FROM "users" WHERE NOT EXISTS (SELECT "user_id" FROM "posts")`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

// Test nested subqueries (depth 2).
func TestSubquery_Nested_Depth2(t *testing.T) {
	instance := createSubqueryTestInstance(t)

	// SELECT * FROM users WHERE id IN (
	//   SELECT user_id FROM posts WHERE id IN (
	//     SELECT post_id FROM comments WHERE body = :search_term
	//   )
	// )
	innerSubquery := astql.Sub(
		astql.Select(instance.T("comments")).
			Fields(instance.F("post_id")).
			Where(instance.C(instance.F("body"), "=", instance.P("search_term"))),
	)

	middleSubquery := astql.Sub(
		astql.Select(instance.T("posts")).
			Fields(instance.F("user_id")).
			Where(astql.CSub(instance.F("id"), astql.IN, innerSubquery)),
	)

	result, err := astql.Select(instance.T("users")).
		Where(astql.CSub(instance.F("id"), astql.IN, middleSubquery)).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	// Parameters should be namespaced by depth: sq1_, sq2_
	expected := `SELECT * FROM "users" WHERE "id" IN (SELECT "user_id" FROM "posts" WHERE "id" IN (SELECT "post_id" FROM "comments" WHERE "body" = :sq2_search_term))`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}

	// Should have parameter with sq2_ prefix (depth 2)
	if len(result.RequiredParams) != 1 {
		t.Errorf("Expected 1 param, got %d", len(result.RequiredParams))
	}
	if !contains(result.RequiredParams, "sq2_search_term") {
		t.Errorf("Expected param 'sq2_search_term', got %v", result.RequiredParams)
	}
}

// Test nested subqueries (depth 3 - max depth).
func TestSubquery_Nested_MaxDepth(t *testing.T) {
	instance := createSubqueryTestInstance(t)

	// Build a query with max depth (3 levels of nesting)
	level3 := astql.Sub(
		astql.Select(instance.T("comments")).
			Fields(instance.F("post_id")).
			Where(instance.C(instance.F("body"), "=", instance.P("term"))),
	)

	level2 := astql.Sub(
		astql.Select(instance.T("posts")).
			Fields(instance.F("user_id")).
			Where(astql.CSub(instance.F("id"), astql.IN, level3)),
	)

	level1 := astql.Sub(
		astql.Select(instance.T("users")).
			Fields(instance.F("id")).
			Where(astql.CSub(instance.F("id"), astql.IN, level2)),
	)

	result, err := astql.Select(instance.T("users")).
		Where(astql.CSub(instance.F("id"), astql.IN, level1)).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	// Should succeed at max depth
	if !strings.Contains(result.SQL, "sq3_term") {
		t.Errorf("Expected parameter with sq3_ prefix at depth 3, got: %s", result.SQL)
	}
}

// Test subquery depth limit exceeded.
func TestSubquery_DepthLimitExceeded(t *testing.T) {
	instance := createSubqueryTestInstance(t)

	// Try to build a query that exceeds max depth (4 levels)
	level4 := astql.Sub(
		astql.Select(instance.T("comments")).
			Fields(instance.F("post_id")),
	)

	level3 := astql.Sub(
		astql.Select(instance.T("posts")).
			Fields(instance.F("user_id")).
			Where(astql.CSub(instance.F("id"), astql.IN, level4)),
	)

	level2 := astql.Sub(
		astql.Select(instance.T("users")).
			Fields(instance.F("id")).
			Where(astql.CSub(instance.F("id"), astql.IN, level3)),
	)

	level1 := astql.Sub(
		astql.Select(instance.T("users")).
			Fields(instance.F("id")).
			Where(astql.CSub(instance.F("id"), astql.IN, level2)),
	)

	_, err := astql.Select(instance.T("users")).
		Where(astql.CSub(instance.F("id"), astql.IN, level1)).
		Render(postgres.New())

	// Should fail with depth exceeded error
	if err == nil {
		t.Fatal("Expected error for exceeding max subquery depth")
	}
	if !strings.Contains(err.Error(), "maximum subquery depth") {
		t.Errorf("Expected 'maximum subquery depth' error, got: %v", err)
	}
}

// Test subquery with multiple parameters.
func TestSubquery_MultipleParameters(t *testing.T) {
	instance := createSubqueryTestInstance(t)

	// SELECT * FROM users WHERE id IN (
	//   SELECT user_id FROM posts WHERE published = :is_published AND title LIKE :title_pattern
	// )
	subquery := astql.Sub(
		astql.Select(instance.T("posts")).
			Fields(instance.F("user_id")).
			Where(instance.And(
				instance.C(instance.F("published"), "=", instance.P("is_published")),
				instance.C(instance.F("title"), "LIKE", instance.P("title_pattern")),
			)),
	)

	result, err := astql.Select(instance.T("users")).
		Where(astql.CSub(instance.F("id"), astql.IN, subquery)).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	// Should have both parameters with sq1_ prefix
	if len(result.RequiredParams) != 2 {
		t.Errorf("Expected 2 params, got %d", len(result.RequiredParams))
	}
	if !contains(result.RequiredParams, "sq1_is_published") {
		t.Errorf("Missing param 'sq1_is_published': %v", result.RequiredParams)
	}
	if !contains(result.RequiredParams, "sq1_title_pattern") {
		t.Errorf("Missing param 'sq1_title_pattern': %v", result.RequiredParams)
	}
}

// Test mixing outer query params with subquery params.
func TestSubquery_MixedParameters(t *testing.T) {
	instance := createSubqueryTestInstance(t)

	// SELECT * FROM users
	// WHERE active = :is_active
	// AND id IN (SELECT user_id FROM posts WHERE published = :is_published)
	subquery := astql.Sub(
		astql.Select(instance.T("posts")).
			Fields(instance.F("user_id")).
			Where(instance.C(instance.F("published"), "=", instance.P("is_published"))),
	)

	result, err := astql.Select(instance.T("users")).
		Where(instance.And(
			instance.C(instance.F("active"), "=", instance.P("is_active")),
			astql.CSub(instance.F("id"), astql.IN, subquery),
		)).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT * FROM "users" WHERE ("active" = :is_active AND "id" IN (SELECT "user_id" FROM "posts" WHERE "published" = :sq1_is_published))`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}

	// Should have both outer and subquery params
	if len(result.RequiredParams) != 2 {
		t.Errorf("Expected 2 params, got %d", len(result.RequiredParams))
	}
	if !contains(result.RequiredParams, "is_active") {
		t.Errorf("Missing outer param 'is_active': %v", result.RequiredParams)
	}
	if !contains(result.RequiredParams, "sq1_is_published") {
		t.Errorf("Missing subquery param 'sq1_is_published': %v", result.RequiredParams)
	}
}

// Test CSub with wrong operator (should panic).
func TestSubquery_CSub_WrongOperator(t *testing.T) {
	instance := createSubqueryTestInstance(t)

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for using = operator with CSub")
		}
	}()

	subquery := astql.Sub(
		astql.Select(instance.T("posts")).
			Fields(instance.F("user_id")),
	)

	// Should panic - CSub only accepts IN/NOT IN
	astql.CSub(instance.F("id"), astql.EQ, subquery)
}

// Test CSubExists with wrong operator (should panic).
func TestSubquery_CSubExists_WrongOperator(t *testing.T) {
	instance := createSubqueryTestInstance(t)

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for using IN operator with CSubExists")
		}
	}()

	subquery := astql.Sub(
		astql.Select(instance.T("posts")).
			Fields(instance.F("user_id")),
	)

	// Should panic - CSubExists only accepts EXISTS/NOT EXISTS
	astql.CSubExists(astql.IN, subquery)
}

// Test Sub with invalid builder (should panic).
func TestSubquery_Sub_InvalidBuilder(t *testing.T) {
	instance := createSubqueryTestInstance(t)

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for building subquery with invalid AST")
		}
	}()

	// Create an invalid builder (Fields on INSERT)
	invalidBuilder := astql.Insert(instance.T("users")).
		Fields(instance.F("id"))

	// Should panic when trying to build
	astql.Sub(invalidBuilder)
}

// Test subquery with complex conditions.
func TestSubquery_ComplexConditions(t *testing.T) {
	instance := createSubqueryTestInstance(t)

	// SELECT * FROM users WHERE id IN (
	//   SELECT user_id FROM posts
	//   WHERE (published = :is_published OR title LIKE :title_pattern)
	//   AND user_id != :excluded_user
	// )
	subquery := astql.Sub(
		astql.Select(instance.T("posts")).
			Fields(instance.F("user_id")).
			Where(instance.And(
				instance.Or(
					instance.C(instance.F("published"), "=", instance.P("is_published")),
					instance.C(instance.F("title"), "LIKE", instance.P("title_pattern")),
				),
				instance.C(instance.F("user_id"), "!=", instance.P("excluded_user")),
			)),
	)

	result, err := astql.Select(instance.T("users")).
		Where(astql.CSub(instance.F("id"), astql.IN, subquery)).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	// All subquery parameters should have sq1_ prefix
	if len(result.RequiredParams) != 3 {
		t.Errorf("Expected 3 params, got %d", len(result.RequiredParams))
	}
	for _, param := range result.RequiredParams {
		if !strings.HasPrefix(param, "sq1_") {
			t.Errorf("Expected all params to have sq1_ prefix, got: %s", param)
		}
	}
}

// Helper function to check if slice contains string.
func contains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

// =============================================================================
// BETWEEN Expression Tests
// =============================================================================

func createBetweenTestInstance(t *testing.T) *astql.ASTQL {
	t.Helper()
	project := dbml.NewProject("test")
	users := dbml.NewTable("users")
	users.AddColumn(dbml.NewColumn("id", "bigint"))
	users.AddColumn(dbml.NewColumn("age", "int"))
	users.AddColumn(dbml.NewColumn("score", "numeric"))
	project.AddTable(users)

	instance, err := astql.NewFromDBML(project)
	if err != nil {
		t.Fatalf("Failed to create instance: %v", err)
	}
	return instance
}

func TestBetween_Basic(t *testing.T) {
	instance := createBetweenTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		Where(astql.Between(instance.F("age"), instance.P("min_age"), instance.P("max_age"))).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT * FROM "users" WHERE "age" BETWEEN :min_age AND :max_age`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}

	if len(result.RequiredParams) != 2 {
		t.Errorf("Expected 2 params, got %d", len(result.RequiredParams))
	}
}

func TestNotBetween_Basic(t *testing.T) {
	instance := createBetweenTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		Where(astql.NotBetween(instance.F("score"), instance.P("min"), instance.P("max"))).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT * FROM "users" WHERE "score" NOT BETWEEN :min AND :max`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

func TestBetween_WithOtherConditions(t *testing.T) {
	instance := createBetweenTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		Where(instance.And(
			astql.Between(instance.F("age"), instance.P("min_age"), instance.P("max_age")),
			instance.C(instance.F("score"), ">", instance.P("min_score")),
		)).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "BETWEEN") {
		t.Errorf("Expected BETWEEN in SQL: %s", result.SQL)
	}
	if len(result.RequiredParams) != 3 {
		t.Errorf("Expected 3 params, got %d", len(result.RequiredParams))
	}
}

// =============================================================================
// Cast Expression Tests
// =============================================================================

func TestCast_ToText(t *testing.T) {
	instance := createBetweenTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		SelectExpr(astql.As(astql.Cast(instance.F("age"), astql.CastText), "age_text")).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT CAST("age" AS TEXT) AS "age_text" FROM "users"`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

func TestCast_ToInteger(t *testing.T) {
	instance := createBetweenTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		SelectExpr(astql.Cast(instance.F("score"), astql.CastInteger)).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, `CAST("score" AS INTEGER)`) {
		t.Errorf("Expected CAST to INTEGER in SQL: %s", result.SQL)
	}
}

func TestCast_ToTimestamp(t *testing.T) {
	project := dbml.NewProject("test")
	users := dbml.NewTable("users")
	users.AddColumn(dbml.NewColumn("created", "varchar"))
	project.AddTable(users)
	instance, _ := astql.NewFromDBML(project)

	result, err := astql.Select(instance.T("users")).
		SelectExpr(astql.Cast(instance.F("created"), astql.CastTimestamp)).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "CAST") && !strings.Contains(result.SQL, "TIMESTAMP") {
		t.Errorf("Expected CAST to TIMESTAMP in SQL: %s", result.SQL)
	}
}

// =============================================================================
// Window Function Tests
// =============================================================================

func createWindowTestInstance(t *testing.T) *astql.ASTQL {
	t.Helper()
	project := dbml.NewProject("test")

	orders := dbml.NewTable("orders")
	orders.AddColumn(dbml.NewColumn("id", "bigint"))
	orders.AddColumn(dbml.NewColumn("user_id", "bigint"))
	orders.AddColumn(dbml.NewColumn("total", "numeric"))
	orders.AddColumn(dbml.NewColumn("created_at", "timestamp"))
	project.AddTable(orders)

	instance, err := astql.NewFromDBML(project)
	if err != nil {
		t.Fatalf("Failed to create instance: %v", err)
	}
	return instance
}

func TestWindowFunction_RowNumber(t *testing.T) {
	instance := createWindowTestInstance(t)

	winExpr := astql.RowNumber().
		OrderBy(instance.F("total"), astql.DESC).
		As("rank")

	result, err := astql.Select(instance.T("orders")).
		Fields(instance.F("id")).
		SelectExpr(winExpr).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "ROW_NUMBER()") {
		t.Errorf("Expected ROW_NUMBER() in SQL: %s", result.SQL)
	}
	if !strings.Contains(result.SQL, "OVER") {
		t.Errorf("Expected OVER in SQL: %s", result.SQL)
	}
	if !strings.Contains(result.SQL, `ORDER BY "total" DESC`) {
		t.Errorf("Expected ORDER BY in window: %s", result.SQL)
	}
}

func TestWindowFunction_Rank(t *testing.T) {
	instance := createWindowTestInstance(t)

	winExpr := astql.Rank().
		PartitionBy(instance.F("user_id")).
		OrderBy(instance.F("total"), astql.DESC).
		As("user_rank")

	result, err := astql.Select(instance.T("orders")).
		SelectExpr(winExpr).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "RANK()") {
		t.Errorf("Expected RANK() in SQL: %s", result.SQL)
	}
	if !strings.Contains(result.SQL, `PARTITION BY "user_id"`) {
		t.Errorf("Expected PARTITION BY in SQL: %s", result.SQL)
	}
}

func TestWindowFunction_DenseRank(t *testing.T) {
	instance := createWindowTestInstance(t)

	winExpr := astql.DenseRank().
		OrderBy(instance.F("total"), astql.ASC).
		Build()

	result, err := astql.Select(instance.T("orders")).
		SelectExpr(winExpr).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "DENSE_RANK()") {
		t.Errorf("Expected DENSE_RANK() in SQL: %s", result.SQL)
	}
}

func TestWindowFunction_Ntile(t *testing.T) {
	instance := createWindowTestInstance(t)

	winExpr := astql.Ntile(instance.P("buckets")).
		OrderBy(instance.F("total"), astql.DESC).
		As("quartile")

	result, err := astql.Select(instance.T("orders")).
		SelectExpr(winExpr).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "NTILE(:buckets)") {
		t.Errorf("Expected NTILE(:buckets) in SQL: %s", result.SQL)
	}
}

func TestWindowFunction_Lag(t *testing.T) {
	instance := createWindowTestInstance(t)

	winExpr := astql.Lag(instance.F("total"), instance.P("offset"), instance.P("default")).
		OrderBy(instance.F("created_at"), astql.ASC).
		As("prev_total")

	result, err := astql.Select(instance.T("orders")).
		SelectExpr(winExpr).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "LAG") {
		t.Errorf("Expected LAG in SQL: %s", result.SQL)
	}
}

func TestWindowFunction_Lead(t *testing.T) {
	instance := createWindowTestInstance(t)

	winExpr := astql.Lead(instance.F("total"), instance.P("offset")).
		OrderBy(instance.F("created_at"), astql.ASC).
		Build()

	result, err := astql.Select(instance.T("orders")).
		SelectExpr(winExpr).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "LEAD") {
		t.Errorf("Expected LEAD in SQL: %s", result.SQL)
	}
}

func TestWindowFunction_FirstValue(t *testing.T) {
	instance := createWindowTestInstance(t)

	winExpr := astql.FirstValue(instance.F("total")).
		PartitionBy(instance.F("user_id")).
		OrderBy(instance.F("created_at"), astql.ASC).
		Build()

	result, err := astql.Select(instance.T("orders")).
		SelectExpr(winExpr).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "FIRST_VALUE") {
		t.Errorf("Expected FIRST_VALUE in SQL: %s", result.SQL)
	}
}

func TestWindowFunction_LastValue(t *testing.T) {
	instance := createWindowTestInstance(t)

	winExpr := astql.LastValue(instance.F("total")).
		PartitionBy(instance.F("user_id")).
		OrderBy(instance.F("created_at"), astql.ASC).
		Frame(astql.FrameUnboundedPreceding, astql.FrameUnboundedFollowing).
		Build()

	result, err := astql.Select(instance.T("orders")).
		SelectExpr(winExpr).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "LAST_VALUE") {
		t.Errorf("Expected LAST_VALUE in SQL: %s", result.SQL)
	}
	if !strings.Contains(result.SQL, "ROWS BETWEEN UNBOUNDED PRECEDING AND UNBOUNDED FOLLOWING") {
		t.Errorf("Expected frame clause in SQL: %s", result.SQL)
	}
}

func TestWindowFunction_SumOver(t *testing.T) {
	instance := createWindowTestInstance(t)

	winExpr := astql.SumOver(instance.F("total")).
		PartitionBy(instance.F("user_id")).
		As("running_total")

	result, err := astql.Select(instance.T("orders")).
		SelectExpr(winExpr).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, `SUM("total")`) {
		t.Errorf("Expected SUM in SQL: %s", result.SQL)
	}
	if !strings.Contains(result.SQL, "OVER") {
		t.Errorf("Expected OVER in SQL: %s", result.SQL)
	}
}

func TestWindowFunction_CountOver(t *testing.T) {
	instance := createWindowTestInstance(t)

	winExpr := astql.CountOver().
		PartitionBy(instance.F("user_id")).
		As("order_count")

	result, err := astql.Select(instance.T("orders")).
		SelectExpr(winExpr).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "COUNT(*)") {
		t.Errorf("Expected COUNT(*) in SQL: %s", result.SQL)
	}
}

// =============================================================================
// Filter Clause Tests
// =============================================================================

func TestSumFilter(t *testing.T) {
	instance := createWindowTestInstance(t)

	filterExpr := astql.SumFilter(
		instance.F("total"),
		instance.C(instance.F("user_id"), "=", instance.P("target_user")),
	)

	result, err := astql.Select(instance.T("orders")).
		SelectExpr(astql.As(filterExpr, "user_total")).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "FILTER") {
		t.Errorf("Expected FILTER in SQL: %s", result.SQL)
	}
	if !strings.Contains(result.SQL, "WHERE") {
		t.Errorf("Expected WHERE inside FILTER: %s", result.SQL)
	}
}

func TestCountFilter(t *testing.T) {
	instance := createWindowTestInstance(t)

	filterExpr := astql.CountFieldFilter(
		instance.F("id"),
		instance.C(instance.F("total"), ">", instance.P("min_total")),
	)

	result, err := astql.Select(instance.T("orders")).
		SelectExpr(astql.As(filterExpr, "high_value_count")).
		GroupBy(instance.F("user_id")).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "COUNT") {
		t.Errorf("Expected COUNT in SQL: %s", result.SQL)
	}
	if !strings.Contains(result.SQL, "FILTER") {
		t.Errorf("Expected FILTER in SQL: %s", result.SQL)
	}
}

// =============================================================================
// ILIKE Operator Tests
// =============================================================================

func TestILIKE_Basic(t *testing.T) {
	project := dbml.NewProject("test")
	users := dbml.NewTable("users")
	users.AddColumn(dbml.NewColumn("username", "varchar"))
	project.AddTable(users)
	instance, _ := astql.NewFromDBML(project)

	result, err := astql.Select(instance.T("users")).
		Where(instance.C(instance.F("username"), "ILIKE", instance.P("pattern"))).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "ILIKE") {
		t.Errorf("Expected ILIKE in SQL: %s", result.SQL)
	}
}

func TestNotILIKE_Basic(t *testing.T) {
	project := dbml.NewProject("test")
	users := dbml.NewTable("users")
	users.AddColumn(dbml.NewColumn("username", "varchar"))
	project.AddTable(users)
	instance, _ := astql.NewFromDBML(project)

	result, err := astql.Select(instance.T("users")).
		Where(instance.C(instance.F("username"), "NOT ILIKE", instance.P("pattern"))).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "NOT ILIKE") {
		t.Errorf("Expected NOT ILIKE in SQL: %s", result.SQL)
	}
}

// =============================================================================
// Regex Operator Tests
// =============================================================================

func TestRegexMatch(t *testing.T) {
	project := dbml.NewProject("test")
	users := dbml.NewTable("users")
	users.AddColumn(dbml.NewColumn("email", "varchar"))
	project.AddTable(users)
	instance, _ := astql.NewFromDBML(project)

	result, err := astql.Select(instance.T("users")).
		Where(instance.C(instance.F("email"), "~", instance.P("pattern"))).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "~") {
		t.Errorf("Expected ~ in SQL: %s", result.SQL)
	}
}

func TestRegexIMatch(t *testing.T) {
	project := dbml.NewProject("test")
	users := dbml.NewTable("users")
	users.AddColumn(dbml.NewColumn("email", "varchar"))
	project.AddTable(users)
	instance, _ := astql.NewFromDBML(project)

	result, err := astql.Select(instance.T("users")).
		Where(instance.C(instance.F("email"), "~*", instance.P("pattern"))).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "~*") {
		t.Errorf("Expected ~* in SQL: %s", result.SQL)
	}
}

// =============================================================================
// Array Operator Tests
// =============================================================================

func TestArrayContains(t *testing.T) {
	project := dbml.NewProject("test")
	users := dbml.NewTable("users")
	users.AddColumn(dbml.NewColumn("tags", "text[]"))
	project.AddTable(users)
	instance, _ := astql.NewFromDBML(project)

	result, err := astql.Select(instance.T("users")).
		Where(instance.C(instance.F("tags"), "@>", instance.P("required_tags"))).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "@>") {
		t.Errorf("Expected @> in SQL: %s", result.SQL)
	}
}

func TestArrayOverlap(t *testing.T) {
	project := dbml.NewProject("test")
	users := dbml.NewTable("users")
	users.AddColumn(dbml.NewColumn("tags", "text[]"))
	project.AddTable(users)
	instance, _ := astql.NewFromDBML(project)

	result, err := astql.Select(instance.T("users")).
		Where(instance.C(instance.F("tags"), "&&", instance.P("any_tags"))).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "&&") {
		t.Errorf("Expected && in SQL: %s", result.SQL)
	}
}

func TestArrayContainedBy(t *testing.T) {
	project := dbml.NewProject("test")
	users := dbml.NewTable("users")
	users.AddColumn(dbml.NewColumn("tags", "text[]"))
	project.AddTable(users)
	instance, _ := astql.NewFromDBML(project)

	result, err := astql.Select(instance.T("users")).
		Where(instance.C(instance.F("tags"), "<@", instance.P("allowed_tags"))).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "<@") {
		t.Errorf("Expected <@ in SQL: %s", result.SQL)
	}
}

func TestNotLike(t *testing.T) {
	project := dbml.NewProject("test")
	users := dbml.NewTable("users")
	users.AddColumn(dbml.NewColumn("username", "varchar"))
	project.AddTable(users)
	instance, _ := astql.NewFromDBML(project)

	result, err := astql.Select(instance.T("users")).
		Where(instance.C(instance.F("username"), "NOT LIKE", instance.P("pattern"))).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "NOT LIKE") {
		t.Errorf("Expected NOT LIKE in SQL: %s", result.SQL)
	}
}

func TestNotRegexMatch(t *testing.T) {
	project := dbml.NewProject("test")
	users := dbml.NewTable("users")
	users.AddColumn(dbml.NewColumn("email", "varchar"))
	project.AddTable(users)
	instance, _ := astql.NewFromDBML(project)

	result, err := astql.Select(instance.T("users")).
		Where(instance.C(instance.F("email"), "!~", instance.P("pattern"))).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "!~") {
		t.Errorf("Expected !~ in SQL: %s", result.SQL)
	}
}

func TestNotRegexIMatch(t *testing.T) {
	project := dbml.NewProject("test")
	users := dbml.NewTable("users")
	users.AddColumn(dbml.NewColumn("email", "varchar"))
	project.AddTable(users)
	instance, _ := astql.NewFromDBML(project)

	result, err := astql.Select(instance.T("users")).
		Where(instance.C(instance.F("email"), "!~*", instance.P("pattern"))).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "!~*") {
		t.Errorf("Expected !~* in SQL: %s", result.SQL)
	}
}

// =============================================================================
// Aggregate Filter Function Tests
// =============================================================================

func TestAvgFilter(t *testing.T) {
	project := dbml.NewProject("test")
	users := dbml.NewTable("users")
	users.AddColumn(dbml.NewColumn("age", "int"))
	users.AddColumn(dbml.NewColumn("active", "boolean"))
	project.AddTable(users)
	instance, _ := astql.NewFromDBML(project)

	result, err := astql.Select(instance.T("users")).
		SelectExpr(astql.AvgFilter(instance.F("age"), instance.C(instance.F("active"), "=", instance.P("is_active")))).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "AVG") && !strings.Contains(result.SQL, "FILTER") {
		t.Errorf("Expected AVG with FILTER in SQL: %s", result.SQL)
	}
}

func TestMinFilter(t *testing.T) {
	project := dbml.NewProject("test")
	users := dbml.NewTable("users")
	users.AddColumn(dbml.NewColumn("age", "int"))
	users.AddColumn(dbml.NewColumn("active", "boolean"))
	project.AddTable(users)
	instance, _ := astql.NewFromDBML(project)

	result, err := astql.Select(instance.T("users")).
		SelectExpr(astql.MinFilter(instance.F("age"), instance.C(instance.F("active"), "=", instance.P("is_active")))).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "MIN") {
		t.Errorf("Expected MIN in SQL: %s", result.SQL)
	}
}

func TestMaxFilter(t *testing.T) {
	project := dbml.NewProject("test")
	users := dbml.NewTable("users")
	users.AddColumn(dbml.NewColumn("age", "int"))
	users.AddColumn(dbml.NewColumn("active", "boolean"))
	project.AddTable(users)
	instance, _ := astql.NewFromDBML(project)

	result, err := astql.Select(instance.T("users")).
		SelectExpr(astql.MaxFilter(instance.F("age"), instance.C(instance.F("active"), "=", instance.P("is_active")))).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "MAX") {
		t.Errorf("Expected MAX in SQL: %s", result.SQL)
	}
}

func TestCountDistinctFilter(t *testing.T) {
	project := dbml.NewProject("test")
	users := dbml.NewTable("users")
	users.AddColumn(dbml.NewColumn("id", "bigint"))
	users.AddColumn(dbml.NewColumn("active", "boolean"))
	project.AddTable(users)
	instance, _ := astql.NewFromDBML(project)

	result, err := astql.Select(instance.T("users")).
		SelectExpr(astql.CountDistinctFilter(instance.F("id"), instance.C(instance.F("active"), "=", instance.P("is_active")))).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "COUNT") && !strings.Contains(result.SQL, "DISTINCT") {
		t.Errorf("Expected COUNT DISTINCT in SQL: %s", result.SQL)
	}
}

// =============================================================================
// Window Spec Builder Tests
// =============================================================================

func TestWindowSpecBuilder(t *testing.T) {
	project := dbml.NewProject("test")
	users := dbml.NewTable("users")
	users.AddColumn(dbml.NewColumn("id", "bigint"))
	users.AddColumn(dbml.NewColumn("age", "int"))
	users.AddColumn(dbml.NewColumn("department", "varchar"))
	project.AddTable(users)
	instance, _ := astql.NewFromDBML(project)

	// Use RowNumber with the window spec builder pattern
	winExpr := astql.RowNumber().
		PartitionBy(instance.F("department")).
		OrderBy(instance.F("age"), astql.DESC).
		As("row_num")

	result, err := astql.Select(instance.T("users")).
		Fields(instance.F("id")).
		SelectExpr(winExpr).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "ROW_NUMBER()") {
		t.Errorf("Expected ROW_NUMBER() in SQL: %s", result.SQL)
	}
	if !strings.Contains(result.SQL, "PARTITION BY") {
		t.Errorf("Expected PARTITION BY in SQL: %s", result.SQL)
	}
	if !strings.Contains(result.SQL, "ORDER BY") {
		t.Errorf("Expected ORDER BY in SQL: %s", result.SQL)
	}
}

func TestWindowSpecBuilder_Build(t *testing.T) {
	project := dbml.NewProject("test")
	users := dbml.NewTable("users")
	users.AddColumn(dbml.NewColumn("age", "int"))
	project.AddTable(users)
	instance, _ := astql.NewFromDBML(project)

	// Test the Window() -> WindowSpecBuilder -> Build() path
	spec := astql.Window().
		OrderBy(instance.F("age"), astql.ASC).
		Rows(astql.FrameUnboundedPreceding, astql.FrameCurrentRow).
		Build()

	// Verify spec was built (it's a value type)
	if spec.FrameStart != astql.FrameUnboundedPreceding {
		t.Error("Expected FrameStart to be set")
	}
}

func TestWindowSpecBuilder_OrderByNulls(t *testing.T) {
	project := dbml.NewProject("test")
	users := dbml.NewTable("users")
	users.AddColumn(dbml.NewColumn("age", "int"))
	project.AddTable(users)
	instance, _ := astql.NewFromDBML(project)

	spec := astql.Window().
		OrderByNulls(instance.F("age"), astql.ASC, astql.NullsLast).
		Build()

	// Verify ordering was set
	if len(spec.OrderBy) == 0 {
		t.Error("Expected OrderBy to be set")
	}
	if spec.OrderBy[0].Nulls != astql.NullsLast {
		t.Errorf("Expected NullsLast, got %v", spec.OrderBy[0].Nulls)
	}
}

func TestAvgOver(t *testing.T) {
	project := dbml.NewProject("test")
	users := dbml.NewTable("users")
	users.AddColumn(dbml.NewColumn("id", "bigint"))
	users.AddColumn(dbml.NewColumn("age", "int"))
	project.AddTable(users)
	instance, _ := astql.NewFromDBML(project)

	winExpr := astql.AvgOver(instance.F("age")).As("avg_age")

	result, err := astql.Select(instance.T("users")).
		Fields(instance.F("id")).
		SelectExpr(winExpr).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "AVG") {
		t.Errorf("Expected AVG in SQL: %s", result.SQL)
	}
}

func TestMinOver(t *testing.T) {
	project := dbml.NewProject("test")
	users := dbml.NewTable("users")
	users.AddColumn(dbml.NewColumn("id", "bigint"))
	users.AddColumn(dbml.NewColumn("age", "int"))
	project.AddTable(users)
	instance, _ := astql.NewFromDBML(project)

	winExpr := astql.MinOver(instance.F("age")).As("min_age")

	result, err := astql.Select(instance.T("users")).
		Fields(instance.F("id")).
		SelectExpr(winExpr).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "MIN") {
		t.Errorf("Expected MIN in SQL: %s", result.SQL)
	}
}

func TestMaxOver(t *testing.T) {
	project := dbml.NewProject("test")
	users := dbml.NewTable("users")
	users.AddColumn(dbml.NewColumn("id", "bigint"))
	users.AddColumn(dbml.NewColumn("age", "int"))
	project.AddTable(users)
	instance, _ := astql.NewFromDBML(project)

	winExpr := astql.MaxOver(instance.F("age")).As("max_age")

	result, err := astql.Select(instance.T("users")).
		Fields(instance.F("id")).
		SelectExpr(winExpr).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "MAX") {
		t.Errorf("Expected MAX in SQL: %s", result.SQL)
	}
}

// =============================================================================
// Math Function Tests
// =============================================================================

func createMathTestInstance(t *testing.T) *astql.ASTQL {
	t.Helper()
	project := dbml.NewProject("test")
	users := dbml.NewTable("users")
	users.AddColumn(dbml.NewColumn("id", "bigint"))
	users.AddColumn(dbml.NewColumn("score", "numeric"))
	users.AddColumn(dbml.NewColumn("balance", "numeric"))
	project.AddTable(users)

	instance, err := astql.NewFromDBML(project)
	if err != nil {
		t.Fatalf("Failed to create instance: %v", err)
	}
	return instance
}

func TestRound_Basic(t *testing.T) {
	instance := createMathTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		SelectExpr(astql.As(astql.Round(instance.F("score")), "rounded")).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "ROUND") {
		t.Errorf("Expected ROUND in SQL: %s", result.SQL)
	}
}

func TestRound_WithPrecision(t *testing.T) {
	instance := createMathTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		SelectExpr(astql.As(astql.Round(instance.F("score"), instance.P("precision")), "rounded")).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "ROUND") {
		t.Errorf("Expected ROUND in SQL: %s", result.SQL)
	}
	if !contains(result.RequiredParams, "precision") {
		t.Errorf("Expected precision param: %v", result.RequiredParams)
	}
}

func TestFloor_Basic(t *testing.T) {
	instance := createMathTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		SelectExpr(astql.As(astql.Floor(instance.F("score")), "floored")).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "FLOOR") {
		t.Errorf("Expected FLOOR in SQL: %s", result.SQL)
	}
}

func TestCeil_Basic(t *testing.T) {
	instance := createMathTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		SelectExpr(astql.As(astql.Ceil(instance.F("score")), "ceiled")).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "CEIL") {
		t.Errorf("Expected CEIL in SQL: %s", result.SQL)
	}
}

func TestAbs_Basic(t *testing.T) {
	instance := createMathTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		SelectExpr(astql.As(astql.Abs(instance.F("balance")), "absolute")).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "ABS") {
		t.Errorf("Expected ABS in SQL: %s", result.SQL)
	}
}

func TestPower_Basic(t *testing.T) {
	instance := createMathTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		SelectExpr(astql.As(astql.Power(instance.F("score"), instance.P("exponent")), "powered")).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "POWER") {
		t.Errorf("Expected POWER in SQL: %s", result.SQL)
	}
	if !contains(result.RequiredParams, "exponent") {
		t.Errorf("Expected exponent param: %v", result.RequiredParams)
	}
}

func TestSqrt_Basic(t *testing.T) {
	instance := createMathTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		SelectExpr(astql.As(astql.Sqrt(instance.F("score")), "root")).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "SQRT") {
		t.Errorf("Expected SQRT in SQL: %s", result.SQL)
	}
}

// =============================================================================
// String Function Tests
// =============================================================================

func createStringTestInstance(t *testing.T) *astql.ASTQL {
	t.Helper()
	project := dbml.NewProject("test")
	users := dbml.NewTable("users")
	users.AddColumn(dbml.NewColumn("id", "bigint"))
	users.AddColumn(dbml.NewColumn("name", "varchar"))
	users.AddColumn(dbml.NewColumn("email", "varchar"))
	project.AddTable(users)

	instance, err := astql.NewFromDBML(project)
	if err != nil {
		t.Fatalf("Failed to create instance: %v", err)
	}
	return instance
}

func TestUpper_Basic(t *testing.T) {
	instance := createStringTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		SelectExpr(astql.As(astql.Upper(instance.F("name")), "upper_name")).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "UPPER") {
		t.Errorf("Expected UPPER in SQL: %s", result.SQL)
	}
}

func TestLower_Basic(t *testing.T) {
	instance := createStringTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		SelectExpr(astql.As(astql.Lower(instance.F("name")), "lower_name")).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "LOWER") {
		t.Errorf("Expected LOWER in SQL: %s", result.SQL)
	}
}

func TestTrim_Basic(t *testing.T) {
	instance := createStringTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		SelectExpr(astql.As(astql.Trim(instance.F("name")), "trimmed")).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "TRIM") {
		t.Errorf("Expected TRIM in SQL: %s", result.SQL)
	}
}

func TestLTrim_Basic(t *testing.T) {
	instance := createStringTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		SelectExpr(astql.As(astql.LTrim(instance.F("name")), "ltrimmed")).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "LTRIM") {
		t.Errorf("Expected LTRIM in SQL: %s", result.SQL)
	}
}

func TestRTrim_Basic(t *testing.T) {
	instance := createStringTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		SelectExpr(astql.As(astql.RTrim(instance.F("name")), "rtrimmed")).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "RTRIM") {
		t.Errorf("Expected RTRIM in SQL: %s", result.SQL)
	}
}

func TestLength_Basic(t *testing.T) {
	instance := createStringTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		SelectExpr(astql.As(astql.Length(instance.F("name")), "name_len")).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "LENGTH") {
		t.Errorf("Expected LENGTH in SQL: %s", result.SQL)
	}
}

func TestSubstring_Basic(t *testing.T) {
	instance := createStringTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		SelectExpr(astql.As(astql.Substring(instance.F("name"), instance.P("start"), instance.P("len")), "substr")).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "SUBSTRING") {
		t.Errorf("Expected SUBSTRING in SQL: %s", result.SQL)
	}
	if len(result.RequiredParams) != 2 {
		t.Errorf("Expected 2 params, got %d: %v", len(result.RequiredParams), result.RequiredParams)
	}
}

func TestReplace_Basic(t *testing.T) {
	instance := createStringTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		SelectExpr(astql.As(astql.Replace(instance.F("name"), instance.P("search"), instance.P("replacement")), "replaced")).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "REPLACE") {
		t.Errorf("Expected REPLACE in SQL: %s", result.SQL)
	}
	if len(result.RequiredParams) != 2 {
		t.Errorf("Expected 2 params, got %d: %v", len(result.RequiredParams), result.RequiredParams)
	}
}

func TestConcat_Basic(t *testing.T) {
	instance := createStringTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		SelectExpr(astql.As(astql.Concat(instance.F("name"), instance.F("email")), "combined")).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "CONCAT") {
		t.Errorf("Expected CONCAT in SQL: %s", result.SQL)
	}
}

// =============================================================================
// Date Function Tests
// =============================================================================

func createDateTestInstance(t *testing.T) *astql.ASTQL {
	t.Helper()
	project := dbml.NewProject("test")
	events := dbml.NewTable("events")
	events.AddColumn(dbml.NewColumn("id", "bigint"))
	events.AddColumn(dbml.NewColumn("created_at", "timestamp"))
	events.AddColumn(dbml.NewColumn("updated_at", "timestamp"))
	project.AddTable(events)

	instance, err := astql.NewFromDBML(project)
	if err != nil {
		t.Fatalf("Failed to create instance: %v", err)
	}
	return instance
}

func TestNow_Basic(t *testing.T) {
	instance := createDateTestInstance(t)

	result, err := astql.Select(instance.T("events")).
		SelectExpr(astql.As(astql.Now(), "current_time")).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "NOW()") {
		t.Errorf("Expected NOW() in SQL: %s", result.SQL)
	}
}

func TestCurrentDate_Basic(t *testing.T) {
	instance := createDateTestInstance(t)

	result, err := astql.Select(instance.T("events")).
		SelectExpr(astql.As(astql.CurrentDate(), "today")).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "CURRENT_DATE") {
		t.Errorf("Expected CURRENT_DATE in SQL: %s", result.SQL)
	}
}

func TestCurrentTime_Basic(t *testing.T) {
	instance := createDateTestInstance(t)

	result, err := astql.Select(instance.T("events")).
		SelectExpr(astql.As(astql.CurrentTime(), "now_time")).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "CURRENT_TIME") {
		t.Errorf("Expected CURRENT_TIME in SQL: %s", result.SQL)
	}
}

func TestCurrentTimestamp_Basic(t *testing.T) {
	instance := createDateTestInstance(t)

	result, err := astql.Select(instance.T("events")).
		SelectExpr(astql.As(astql.CurrentTimestamp(), "ts")).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "CURRENT_TIMESTAMP") {
		t.Errorf("Expected CURRENT_TIMESTAMP in SQL: %s", result.SQL)
	}
}

func TestExtract_Year(t *testing.T) {
	instance := createDateTestInstance(t)

	result, err := astql.Select(instance.T("events")).
		SelectExpr(astql.As(astql.Extract(types.PartYear, instance.F("created_at")), "year")).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "EXTRACT") && !strings.Contains(result.SQL, "YEAR") {
		t.Errorf("Expected EXTRACT with YEAR in SQL: %s", result.SQL)
	}
}

func TestExtract_Month(t *testing.T) {
	instance := createDateTestInstance(t)

	result, err := astql.Select(instance.T("events")).
		SelectExpr(astql.As(astql.Extract(types.PartMonth, instance.F("created_at")), "month")).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "EXTRACT") {
		t.Errorf("Expected EXTRACT in SQL: %s", result.SQL)
	}
}

func TestDateTrunc_Month(t *testing.T) {
	instance := createDateTestInstance(t)

	result, err := astql.Select(instance.T("events")).
		SelectExpr(astql.As(astql.DateTrunc(types.PartMonth, instance.F("created_at")), "month_start")).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "DATE_TRUNC") {
		t.Errorf("Expected DATE_TRUNC in SQL: %s", result.SQL)
	}
}

func TestDateTrunc_Day(t *testing.T) {
	instance := createDateTestInstance(t)

	result, err := astql.Select(instance.T("events")).
		SelectExpr(astql.As(astql.DateTrunc(types.PartDay, instance.F("created_at")), "day_start")).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "DATE_TRUNC") {
		t.Errorf("Expected DATE_TRUNC in SQL: %s", result.SQL)
	}
}

// =============================================================================
// HAVING Aggregate Tests
// =============================================================================

func createHavingTestInstance(t *testing.T) *astql.ASTQL {
	t.Helper()
	project := dbml.NewProject("test")
	orders := dbml.NewTable("orders")
	orders.AddColumn(dbml.NewColumn("id", "bigint"))
	orders.AddColumn(dbml.NewColumn("user_id", "bigint"))
	orders.AddColumn(dbml.NewColumn("total", "numeric"))
	orders.AddColumn(dbml.NewColumn("quantity", "int"))
	project.AddTable(orders)

	instance, err := astql.NewFromDBML(project)
	if err != nil {
		t.Fatalf("Failed to create instance: %v", err)
	}
	return instance
}

func TestHavingSum_Basic(t *testing.T) {
	instance := createHavingTestInstance(t)

	result, err := astql.Select(instance.T("orders")).
		Fields(instance.F("user_id")).
		SelectExpr(astql.As(astql.Sum(instance.F("total")), "total_amount")).
		GroupBy(instance.F("user_id")).
		HavingAgg(astql.HavingSum(instance.F("total"), astql.GT, instance.P("min_total"))).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "HAVING") {
		t.Errorf("Expected HAVING in SQL: %s", result.SQL)
	}
	if !strings.Contains(result.SQL, "SUM") {
		t.Errorf("Expected SUM in SQL: %s", result.SQL)
	}
}

func TestHavingAvg_Basic(t *testing.T) {
	instance := createHavingTestInstance(t)

	result, err := astql.Select(instance.T("orders")).
		Fields(instance.F("user_id")).
		GroupBy(instance.F("user_id")).
		HavingAgg(astql.HavingAvg(instance.F("total"), astql.GE, instance.P("min_avg"))).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "HAVING") {
		t.Errorf("Expected HAVING in SQL: %s", result.SQL)
	}
	if !strings.Contains(result.SQL, "AVG") {
		t.Errorf("Expected AVG in SQL: %s", result.SQL)
	}
}

func TestHavingMin_Basic(t *testing.T) {
	instance := createHavingTestInstance(t)

	result, err := astql.Select(instance.T("orders")).
		Fields(instance.F("user_id")).
		GroupBy(instance.F("user_id")).
		HavingAgg(astql.HavingMin(instance.F("total"), astql.GT, instance.P("threshold"))).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "HAVING") {
		t.Errorf("Expected HAVING in SQL: %s", result.SQL)
	}
	if !strings.Contains(result.SQL, "MIN") {
		t.Errorf("Expected MIN in SQL: %s", result.SQL)
	}
}

func TestHavingMax_Basic(t *testing.T) {
	instance := createHavingTestInstance(t)

	result, err := astql.Select(instance.T("orders")).
		Fields(instance.F("user_id")).
		GroupBy(instance.F("user_id")).
		HavingAgg(astql.HavingMax(instance.F("total"), astql.LT, instance.P("max_limit"))).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "HAVING") {
		t.Errorf("Expected HAVING in SQL: %s", result.SQL)
	}
	if !strings.Contains(result.SQL, "MAX") {
		t.Errorf("Expected MAX in SQL: %s", result.SQL)
	}
}

func TestHavingCountDistinct_Basic(t *testing.T) {
	instance := createHavingTestInstance(t)

	result, err := astql.Select(instance.T("orders")).
		Fields(instance.F("user_id")).
		GroupBy(instance.F("user_id")).
		HavingAgg(astql.HavingCountDistinct(instance.F("id"), astql.GE, instance.P("min_orders"))).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "HAVING") {
		t.Errorf("Expected HAVING in SQL: %s", result.SQL)
	}
	if !strings.Contains(result.SQL, "COUNT") {
		t.Errorf("Expected COUNT in SQL: %s", result.SQL)
	}
}

// =============================================================================
// Coalesce and NullIf Tests
// =============================================================================

func TestCoalesce_TwoValues(t *testing.T) {
	instance := createMathTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		SelectExpr(astql.As(astql.Coalesce(instance.P("val1"), instance.P("val2")), "result")).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "COALESCE") {
		t.Errorf("Expected COALESCE in SQL: %s", result.SQL)
	}
}

func TestCoalesce_ThreeValues(t *testing.T) {
	instance := createMathTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		SelectExpr(astql.As(astql.Coalesce(instance.P("val1"), instance.P("val2"), instance.P("val3")), "result")).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "COALESCE") {
		t.Errorf("Expected COALESCE in SQL: %s", result.SQL)
	}
	if len(result.RequiredParams) != 3 {
		t.Errorf("Expected 3 params, got %d: %v", len(result.RequiredParams), result.RequiredParams)
	}
}

func TestCoalesce_PanicOnSingleValue(t *testing.T) {
	instance := createMathTestInstance(t)

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for Coalesce with single value")
		}
	}()

	astql.Coalesce(instance.P("val1"))
}

func TestNullIf_Basic(t *testing.T) {
	instance := createMathTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		SelectExpr(astql.As(astql.NullIf(instance.P("val1"), instance.P("val2")), "result")).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "NULLIF") {
		t.Errorf("Expected NULLIF in SQL: %s", result.SQL)
	}
}

// =============================================================================
// Cast Variant Tests
// =============================================================================

func TestCast_ToNumeric(t *testing.T) {
	instance := createStringTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		SelectExpr(astql.Cast(instance.F("id"), astql.CastNumeric)).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "CAST") && !strings.Contains(result.SQL, "NUMERIC") {
		t.Errorf("Expected CAST to NUMERIC in SQL: %s", result.SQL)
	}
}

func TestCast_ToBoolean(t *testing.T) {
	instance := createStringTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		SelectExpr(astql.Cast(instance.F("id"), astql.CastBoolean)).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "CAST") {
		t.Errorf("Expected CAST in SQL: %s", result.SQL)
	}
}

func TestCast_ToDate(t *testing.T) {
	instance := createStringTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		SelectExpr(astql.Cast(instance.F("name"), astql.CastDate)).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "CAST") && !strings.Contains(result.SQL, "DATE") {
		t.Errorf("Expected CAST to DATE in SQL: %s", result.SQL)
	}
}

func TestCast_ToReal(t *testing.T) {
	instance := createStringTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		SelectExpr(astql.Cast(instance.F("id"), astql.CastReal)).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "CAST") {
		t.Errorf("Expected CAST in SQL: %s", result.SQL)
	}
}

func TestCast_ToText_FromNumeric(t *testing.T) {
	instance := createMathTestInstance(t)

	result, err := astql.Select(instance.T("users")).
		SelectExpr(astql.Cast(instance.F("score"), astql.CastText)).
		Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result.SQL, "CAST") {
		t.Errorf("Expected CAST in SQL: %s", result.SQL)
	}
}
