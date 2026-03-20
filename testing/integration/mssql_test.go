// Package integration provides integration tests for astql using real databases.
package integration

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	"github.com/testcontainers/testcontainers-go/modules/mssql"
	"github.com/zoobz-io/astql"
	astqlmssql "github.com/zoobz-io/astql/mssql"
	"github.com/zoobz-io/dbml"
)

// MSSQLContainer wraps a testcontainers SQL Server instance.
type MSSQLContainer struct {
	container *mssql.MSSQLServerContainer
	db        *sql.DB
	connStr   string
}

// Exec executes a SQL statement.
func (mc *MSSQLContainer) Exec(ctx context.Context, t *testing.T, sql string, args ...any) {
	t.Helper()
	_, err := mc.db.ExecContext(ctx, sql, args...)
	if err != nil {
		t.Fatalf("Failed to execute SQL: %v\nSQL: %s", err, sql)
	}
}

// QueryRow executes a query and returns a single row.
func (mc *MSSQLContainer) QueryRow(ctx context.Context, t *testing.T, sql string, args ...any) *sql.Row {
	t.Helper()
	return mc.db.QueryRowContext(ctx, sql, args...)
}

// Query executes a query and returns rows.
func (mc *MSSQLContainer) Query(ctx context.Context, t *testing.T, sql string, args ...any) *sql.Rows {
	t.Helper()
	rows, err := mc.db.QueryContext(ctx, sql, args...)
	if err != nil {
		t.Fatalf("Failed to execute query: %v\nSQL: %s", err, sql)
	}
	return rows
}

// createMSSQLTestInstance creates an ASTQL instance matching the test database schema.
func createMSSQLTestInstance(t *testing.T) *astql.ASTQL {
	t.Helper()

	project := dbml.NewProject("test")

	users := dbml.NewTable("users")
	users.AddColumn(dbml.NewColumn("id", "bigint"))
	users.AddColumn(dbml.NewColumn("username", "varchar"))
	users.AddColumn(dbml.NewColumn("email", "varchar"))
	users.AddColumn(dbml.NewColumn("age", "int"))
	users.AddColumn(dbml.NewColumn("active", "bit"))
	project.AddTable(users)

	posts := dbml.NewTable("posts")
	posts.AddColumn(dbml.NewColumn("id", "bigint"))
	posts.AddColumn(dbml.NewColumn("user_id", "bigint"))
	posts.AddColumn(dbml.NewColumn("title", "varchar"))
	posts.AddColumn(dbml.NewColumn("views", "int"))
	posts.AddColumn(dbml.NewColumn("published", "bit"))
	project.AddTable(posts)

	orders := dbml.NewTable("orders")
	orders.AddColumn(dbml.NewColumn("id", "bigint"))
	orders.AddColumn(dbml.NewColumn("user_id", "bigint"))
	orders.AddColumn(dbml.NewColumn("total", "decimal"))
	orders.AddColumn(dbml.NewColumn("status", "varchar"))
	project.AddTable(orders)

	instance, err := astql.NewFromDBML(project)
	if err != nil {
		t.Fatalf("Failed to create instance: %v", err)
	}
	return instance
}

// setupMSSQLSchema creates the test database schema.
func setupMSSQLSchema(ctx context.Context, t *testing.T, mc *MSSQLContainer) {
	t.Helper()

	// Drop tables if they exist (in correct order due to foreign keys)
	mc.Exec(ctx, t, `
		IF OBJECT_ID('dbo.orders', 'U') IS NOT NULL DROP TABLE orders;
		IF OBJECT_ID('dbo.posts', 'U') IS NOT NULL DROP TABLE posts;
		IF OBJECT_ID('dbo.users', 'U') IS NOT NULL DROP TABLE users;
	`)

	mc.Exec(ctx, t, `
		CREATE TABLE users (
			id BIGINT IDENTITY(1,1) PRIMARY KEY,
			username VARCHAR(255) NOT NULL,
			email VARCHAR(255) NOT NULL UNIQUE,
			age INT,
			active BIT DEFAULT 1
		)
	`)

	mc.Exec(ctx, t, `
		CREATE TABLE posts (
			id BIGINT IDENTITY(1,1) PRIMARY KEY,
			user_id BIGINT,
			title VARCHAR(255) NOT NULL,
			views INT DEFAULT 0,
			published BIT DEFAULT 0,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)
	`)

	mc.Exec(ctx, t, `
		CREATE TABLE orders (
			id BIGINT IDENTITY(1,1) PRIMARY KEY,
			user_id BIGINT,
			total DECIMAL(10,2) NOT NULL,
			status VARCHAR(50) DEFAULT 'pending',
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)
	`)
}

// seedMSSQLData inserts test data.
func seedMSSQLData(ctx context.Context, t *testing.T, mc *MSSQLContainer) {
	t.Helper()

	// SQL Server needs IDENTITY_INSERT ON to insert explicit IDs.
	// Must be in same batch since IDENTITY_INSERT is connection-scoped
	// and sql.DB uses a connection pool.
	mc.Exec(ctx, t, `
		SET IDENTITY_INSERT users ON;
		INSERT INTO users (id, username, email, age, active) VALUES
		(1, 'alice', 'alice@example.com', 30, 1),
		(2, 'bob', 'bob@example.com', 25, 1),
		(3, 'charlie', 'charlie@example.com', 35, 0),
		(4, 'diana', 'diana@example.com', 28, 1);
		SET IDENTITY_INSERT users OFF;
	`)

	mc.Exec(ctx, t, `
		SET IDENTITY_INSERT posts ON;
		INSERT INTO posts (id, user_id, title, views, published) VALUES
		(1, 1, 'First Post', 100, 1),
		(2, 1, 'Second Post', 50, 1),
		(3, 2, 'Bobs Post', 75, 1),
		(4, 3, 'Draft Post', 0, 0);
		SET IDENTITY_INSERT posts OFF;
	`)

	mc.Exec(ctx, t, `
		SET IDENTITY_INSERT orders ON;
		INSERT INTO orders (id, user_id, total, status) VALUES
		(1, 1, 99.99, 'completed'),
		(2, 1, 149.99, 'completed'),
		(3, 2, 49.99, 'pending'),
		(4, 4, 199.99, 'completed');
		SET IDENTITY_INSERT orders OFF;
	`)
}

// cleanupMSSQLData removes all test data to ensure test isolation.
func cleanupMSSQLData(ctx context.Context, t *testing.T, mc *MSSQLContainer) {
	t.Helper()
	mc.Exec(ctx, t, `DELETE FROM orders`)
	mc.Exec(ctx, t, `DELETE FROM posts`)
	mc.Exec(ctx, t, `DELETE FROM users`)
	mc.Exec(ctx, t, `DBCC CHECKIDENT ('users', RESEED, 0)`)
	mc.Exec(ctx, t, `DBCC CHECKIDENT ('posts', RESEED, 0)`)
	mc.Exec(ctx, t, `DBCC CHECKIDENT ('orders', RESEED, 0)`)
}

// convertMSSQLParams converts astql named parameters to SQL Server positional parameters.
// SQL Server go driver uses @p1, @p2, etc. for positional params.
// Parameters are extracted in the order they appear in the SQL string.
// astql uses : prefix for named params (sqlx converts : to @ for MSSQL).
func convertMSSQLParams(sqlStr string, params map[string]any) (string, []any) {
	args := make([]any, 0)
	result := strings.Builder{}
	paramNum := 1

	i := 0
	for i < len(sqlStr) {
		if sqlStr[i] == ':' {
			// Find end of parameter name
			j := i + 1
			for j < len(sqlStr) && (isAlphaNumericMSSQL(sqlStr[j]) || sqlStr[j] == '_') {
				j++
			}
			if j > i+1 {
				paramName := sqlStr[i+1 : j]
				if value, ok := params[paramName]; ok {
					result.WriteString(fmt.Sprintf("@p%d", paramNum))
					args = append(args, value)
					paramNum++
					i = j
					continue
				}
			}
		}
		result.WriteByte(sqlStr[i])
		i++
	}

	return result.String(), args
}

func isAlphaNumericMSSQL(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// TestMSSQLIntegration_BasicSelect tests basic SELECT queries against SQL Server.
func TestMSSQLIntegration_BasicSelect(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	mc := getMSSQLContainer(t)

	setupMSSQLSchema(ctx, t, mc)
	seedMSSQLData(ctx, t, mc)
	t.Cleanup(func() { cleanupMSSQLData(ctx, t, mc) })

	instance := createMSSQLTestInstance(t)
	renderer := astqlmssql.New()

	result, err := astql.Select(instance.T("users")).Render(renderer)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	rows := mc.Query(ctx, t, result.SQL)
	defer rows.Close()

	count := 0
	for rows.Next() {
		count++
	}
	if count != 4 {
		t.Errorf("Expected 4 users, got %d", count)
	}
}

// TestMSSQLIntegration_SelectWithWhere tests WHERE clause against SQL Server.
func TestMSSQLIntegration_SelectWithWhere(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	mc := getMSSQLContainer(t)

	setupMSSQLSchema(ctx, t, mc)
	seedMSSQLData(ctx, t, mc)
	t.Cleanup(func() { cleanupMSSQLData(ctx, t, mc) })

	instance := createMSSQLTestInstance(t)
	renderer := astqlmssql.New()

	result, err := astql.Select(instance.T("users")).
		Fields(instance.F("username")).
		Where(instance.C(instance.F("active"), astql.EQ, instance.P("is_active"))).
		Render(renderer)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	sql, args := convertMSSQLParams(result.SQL, map[string]any{"is_active": 1})
	rows := mc.Query(ctx, t, sql, args...)
	defer rows.Close()

	var usernames []string
	for rows.Next() {
		var username string
		if err := rows.Scan(&username); err != nil {
			t.Fatalf("Scan failed: %v", err)
		}
		usernames = append(usernames, username)
	}

	if len(usernames) != 3 {
		t.Errorf("Expected 3 active users, got %d: %v", len(usernames), usernames)
	}
}

// TestMSSQLIntegration_Insert tests INSERT operations against SQL Server.
func TestMSSQLIntegration_Insert(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	mc := getMSSQLContainer(t)

	setupMSSQLSchema(ctx, t, mc)
	t.Cleanup(func() { cleanupMSSQLData(ctx, t, mc) })

	instance := createMSSQLTestInstance(t)
	renderer := astqlmssql.New()

	vm := instance.ValueMap()
	vm[instance.F("username")] = instance.P("username")
	vm[instance.F("email")] = instance.P("email")
	vm[instance.F("age")] = instance.P("age")

	result, err := astql.Insert(instance.T("users")).
		Values(vm).
		Render(renderer)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	sql, args := convertMSSQLParams(result.SQL, map[string]any{
		"username": "newuser",
		"email":    "newuser@example.com",
		"age":      42,
	})

	mc.Exec(ctx, t, sql, args...)

	// Verify insert
	var count int
	row := mc.QueryRow(ctx, t, "SELECT COUNT(*) FROM users WHERE username = 'newuser'")
	if err := row.Scan(&count); err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 user, got %d", count)
	}
}

// TestMSSQLIntegration_InsertWithOutput tests INSERT with OUTPUT clause against SQL Server.
func TestMSSQLIntegration_InsertWithOutput(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	mc := getMSSQLContainer(t)

	setupMSSQLSchema(ctx, t, mc)
	t.Cleanup(func() { cleanupMSSQLData(ctx, t, mc) })

	instance := createMSSQLTestInstance(t)
	renderer := astqlmssql.New()

	vm := instance.ValueMap()
	vm[instance.F("username")] = instance.P("username")
	vm[instance.F("email")] = instance.P("email")
	vm[instance.F("age")] = instance.P("age")

	result, err := astql.Insert(instance.T("users")).
		Values(vm).
		Returning(instance.F("id")).
		Render(renderer)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	sql, args := convertMSSQLParams(result.SQL, map[string]any{
		"username": "outputuser",
		"email":    "output@example.com",
		"age":      33,
	})

	var id int64
	err = mc.db.QueryRowContext(ctx, sql, args...).Scan(&id)
	if err != nil {
		t.Fatalf("Insert with OUTPUT failed: %v", err)
	}

	if id <= 0 {
		t.Errorf("Expected positive ID, got %d", id)
	}
}

// TestMSSQLIntegration_Update tests UPDATE operations against SQL Server.
func TestMSSQLIntegration_Update(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	mc := getMSSQLContainer(t)

	setupMSSQLSchema(ctx, t, mc)
	seedMSSQLData(ctx, t, mc)
	t.Cleanup(func() { cleanupMSSQLData(ctx, t, mc) })

	instance := createMSSQLTestInstance(t)
	renderer := astqlmssql.New()

	result, err := astql.Update(instance.T("users")).
		Set(instance.F("age"), instance.P("new_age")).
		Where(instance.C(instance.F("username"), astql.EQ, instance.P("username"))).
		Render(renderer)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	sql, args := convertMSSQLParams(result.SQL, map[string]any{
		"new_age":  99,
		"username": "alice",
	})

	mc.Exec(ctx, t, sql, args...)

	// Verify update
	var age int
	row := mc.QueryRow(ctx, t, "SELECT age FROM users WHERE username = 'alice'")
	if err := row.Scan(&age); err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if age != 99 {
		t.Errorf("Expected age 99, got %d", age)
	}
}

// TestMSSQLIntegration_Delete tests DELETE operations against SQL Server.
func TestMSSQLIntegration_Delete(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	mc := getMSSQLContainer(t)

	setupMSSQLSchema(ctx, t, mc)
	seedMSSQLData(ctx, t, mc)
	t.Cleanup(func() { cleanupMSSQLData(ctx, t, mc) })

	instance := createMSSQLTestInstance(t)
	renderer := astqlmssql.New()

	result, err := astql.Delete(instance.T("users")).
		Where(instance.C(instance.F("active"), astql.EQ, instance.P("is_active"))).
		Render(renderer)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	sql, args := convertMSSQLParams(result.SQL, map[string]any{"is_active": 0})
	mc.Exec(ctx, t, sql, args...)

	// Verify deletion
	var count int
	row := mc.QueryRow(ctx, t, "SELECT COUNT(*) FROM users")
	if err := row.Scan(&count); err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if count != 3 {
		t.Errorf("Expected 3 users after delete, got %d", count)
	}
}

// TestMSSQLIntegration_Join tests JOIN operations against SQL Server.
func TestMSSQLIntegration_Join(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	mc := getMSSQLContainer(t)

	setupMSSQLSchema(ctx, t, mc)
	seedMSSQLData(ctx, t, mc)
	t.Cleanup(func() { cleanupMSSQLData(ctx, t, mc) })

	instance := createMSSQLTestInstance(t)
	renderer := astqlmssql.New()

	result, err := astql.Select(instance.T("users", "u")).
		Fields(
			instance.WithTable(instance.F("username"), "u"),
			instance.WithTable(instance.F("title"), "p"),
		).
		InnerJoin(
			instance.T("posts", "p"),
			astql.CF(
				instance.WithTable(instance.F("id"), "u"),
				astql.EQ,
				instance.WithTable(instance.F("user_id"), "p"),
			),
		).
		Where(instance.C(
			instance.WithTable(instance.F("published"), "p"),
			astql.EQ,
			instance.P("is_published"),
		)).
		Render(renderer)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	sql, args := convertMSSQLParams(result.SQL, map[string]any{"is_published": 1})
	rows := mc.Query(ctx, t, sql, args...)
	defer rows.Close()

	count := 0
	for rows.Next() {
		count++
	}
	if count != 3 {
		t.Errorf("Expected 3 published posts, got %d", count)
	}
}

// TestMSSQLIntegration_Aggregates tests aggregate functions against SQL Server.
func TestMSSQLIntegration_Aggregates(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	mc := getMSSQLContainer(t)

	setupMSSQLSchema(ctx, t, mc)
	seedMSSQLData(ctx, t, mc)
	t.Cleanup(func() { cleanupMSSQLData(ctx, t, mc) })

	instance := createMSSQLTestInstance(t)
	renderer := astqlmssql.New()

	result, err := astql.Select(instance.T("posts")).
		Fields(instance.F("user_id")).
		SelectExpr(astql.As(astql.CountStar(), "post_count")).
		GroupBy(instance.F("user_id")).
		HavingAgg(astql.HavingCount(astql.GT, instance.P("min_count"))).
		Render(renderer)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	sql, args := convertMSSQLParams(result.SQL, map[string]any{"min_count": 1})
	rows := mc.Query(ctx, t, sql, args...)
	defer rows.Close()

	count := 0
	for rows.Next() {
		count++
	}
	// Only alice has more than 1 post
	if count != 1 {
		t.Errorf("Expected 1 user with >1 posts, got %d", count)
	}
}

// TestMSSQLIntegration_OrderByOffsetFetch tests ORDER BY with OFFSET/FETCH against SQL Server.
func TestMSSQLIntegration_OrderByOffsetFetch(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	mc := getMSSQLContainer(t)

	setupMSSQLSchema(ctx, t, mc)
	seedMSSQLData(ctx, t, mc)
	t.Cleanup(func() { cleanupMSSQLData(ctx, t, mc) })

	instance := createMSSQLTestInstance(t)
	renderer := astqlmssql.New()

	// SQL Server requires ORDER BY for OFFSET/FETCH
	result, err := astql.Select(instance.T("users")).
		Fields(instance.F("username"), instance.F("age")).
		OrderBy(instance.F("age"), astql.DESC).
		Limit(2).
		Render(renderer)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	rows := mc.Query(ctx, t, result.SQL)
	defer rows.Close()

	var ages []int
	for rows.Next() {
		var username string
		var age int
		if err := rows.Scan(&username, &age); err != nil {
			t.Fatalf("Scan failed: %v", err)
		}
		ages = append(ages, age)
	}

	if len(ages) != 2 {
		t.Fatalf("Expected 2 users, got %d", len(ages))
	}
	if ages[0] != 35 || ages[1] != 30 {
		t.Errorf("Expected ages [35, 30], got %v", ages)
	}
}

// TestMSSQLIntegration_WindowFunction tests window functions against SQL Server.
func TestMSSQLIntegration_WindowFunction(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	mc := getMSSQLContainer(t)

	setupMSSQLSchema(ctx, t, mc)
	seedMSSQLData(ctx, t, mc)
	t.Cleanup(func() { cleanupMSSQLData(ctx, t, mc) })

	instance := createMSSQLTestInstance(t)
	renderer := astqlmssql.New()

	winExpr := astql.RowNumber().
		OrderBy(instance.F("age"), astql.DESC).
		As("rank")

	result, err := astql.Select(instance.T("users")).
		Fields(instance.F("username"), instance.F("age")).
		SelectExpr(winExpr).
		Render(renderer)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	rows := mc.Query(ctx, t, result.SQL)
	defer rows.Close()

	results := make(map[string]int)
	for rows.Next() {
		var username string
		var age, rank int
		if err := rows.Scan(&username, &age, &rank); err != nil {
			t.Fatalf("Scan failed: %v", err)
		}
		results[username] = rank
	}

	if len(results) != 4 {
		t.Fatalf("Expected 4 users, got %d", len(results))
	}

	// charlie (35) should be rank 1
	if results["charlie"] != 1 {
		t.Errorf("Expected charlie to be rank 1, got %d", results["charlie"])
	}
}

// TestMSSQLIntegration_CaseExpression tests CASE expressions against SQL Server.
func TestMSSQLIntegration_CaseExpression(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	mc := getMSSQLContainer(t)

	setupMSSQLSchema(ctx, t, mc)
	seedMSSQLData(ctx, t, mc)
	t.Cleanup(func() { cleanupMSSQLData(ctx, t, mc) })

	instance := createMSSQLTestInstance(t)
	renderer := astqlmssql.New()

	caseExpr := astql.Case().
		When(instance.C(instance.F("age"), astql.LT, instance.P("young_age")), instance.P("young_label")).
		When(instance.C(instance.F("age"), astql.LT, instance.P("mid_age")), instance.P("mid_label")).
		Else(instance.P("senior_label")).
		As("age_group").
		Build()

	result, err := astql.Select(instance.T("users")).
		Fields(instance.F("username")).
		SelectExpr(caseExpr).
		Render(renderer)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	sql, args := convertMSSQLParams(result.SQL, map[string]any{
		"young_age":    27,
		"young_label":  "young",
		"mid_age":      32,
		"mid_label":    "middle",
		"senior_label": "senior",
	})

	rows := mc.Query(ctx, t, sql, args...)
	defer rows.Close()

	results := make(map[string]string)
	for rows.Next() {
		var username, ageGroup string
		if err := rows.Scan(&username, &ageGroup); err != nil {
			t.Fatalf("Scan failed: %v", err)
		}
		results[username] = ageGroup
	}

	expected := map[string]string{
		"bob":     "young",
		"diana":   "middle",
		"alice":   "middle",
		"charlie": "senior",
	}

	for user, expectedGroup := range expected {
		if results[user] != expectedGroup {
			t.Errorf("Expected %s to be '%s', got '%s'", user, expectedGroup, results[user])
		}
	}
}

// TestMSSQLIntegration_Between tests BETWEEN condition against SQL Server.
func TestMSSQLIntegration_Between(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	mc := getMSSQLContainer(t)

	setupMSSQLSchema(ctx, t, mc)
	seedMSSQLData(ctx, t, mc)
	t.Cleanup(func() { cleanupMSSQLData(ctx, t, mc) })

	instance := createMSSQLTestInstance(t)
	renderer := astqlmssql.New()

	// Need ORDER BY for pagination in SQL Server
	result, err := astql.Select(instance.T("users")).
		Fields(instance.F("username")).
		Where(astql.Between(instance.F("age"), instance.P("min_age"), instance.P("max_age"))).
		Render(renderer)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	sql, args := convertMSSQLParams(result.SQL, map[string]any{
		"min_age": 25,
		"max_age": 30,
	})
	rows := mc.Query(ctx, t, sql, args...)
	defer rows.Close()

	var usernames []string
	for rows.Next() {
		var username string
		if err := rows.Scan(&username); err != nil {
			t.Fatalf("Scan failed: %v", err)
		}
		usernames = append(usernames, username)
	}

	// bob (25), diana (28), alice (30)
	if len(usernames) != 3 {
		t.Errorf("Expected 3 users, got %d: %v", len(usernames), usernames)
	}
}

// TestMSSQLIntegration_GetDate tests GETDATE() function against SQL Server.
func TestMSSQLIntegration_GetDate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	mc := getMSSQLContainer(t)

	setupMSSQLSchema(ctx, t, mc)
	seedMSSQLData(ctx, t, mc)
	t.Cleanup(func() { cleanupMSSQLData(ctx, t, mc) })

	instance := createMSSQLTestInstance(t)
	renderer := astqlmssql.New()

	result, err := astql.Select(instance.T("users")).
		SelectExpr(astql.As(astql.Now(), "current_time")).
		Render(renderer)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	// SQL Server should render NOW() as GETDATE()
	if !strings.Contains(result.SQL, "GETDATE()") {
		t.Errorf("Expected GETDATE() in SQL, got: %s", result.SQL)
	}

	rows := mc.Query(ctx, t, result.SQL)
	defer rows.Close()

	count := 0
	for rows.Next() {
		count++
	}
	if count != 4 {
		t.Errorf("Expected 4 rows, got %d", count)
	}
}

// TestMSSQLIntegration_Union tests UNION operations against SQL Server.
func TestMSSQLIntegration_Union(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	mc := getMSSQLContainer(t)

	setupMSSQLSchema(ctx, t, mc)
	seedMSSQLData(ctx, t, mc)
	t.Cleanup(func() { cleanupMSSQLData(ctx, t, mc) })

	instance := createMSSQLTestInstance(t)
	renderer := astqlmssql.New()

	query1 := astql.Select(instance.T("users")).
		Fields(instance.F("username")).
		Where(instance.C(instance.F("age"), astql.GT, instance.P("min_age")))

	query2 := astql.Select(instance.T("users")).
		Fields(instance.F("username")).
		Where(instance.C(instance.F("active"), astql.EQ, instance.P("is_active")))

	result, err := query1.Union(query2).Render(renderer)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	params := map[string]any{
		"q0_min_age":   30,
		"q1_is_active": 0,
	}
	sql, args := convertMSSQLParams(result.SQL, params)

	rows := mc.Query(ctx, t, sql, args...)
	defer rows.Close()

	var usernames []string
	for rows.Next() {
		var username string
		if err := rows.Scan(&username); err != nil {
			t.Fatalf("Scan failed: %v", err)
		}
		usernames = append(usernames, username)
	}

	// charlie (35, inactive) should appear once due to UNION dedup
	if len(usernames) != 1 {
		t.Errorf("Expected 1 unique user, got %d: %v", len(usernames), usernames)
	}
}

// TestMSSQLIntegration_NotEqual tests <> operator against SQL Server.
func TestMSSQLIntegration_NotEqual(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	mc := getMSSQLContainer(t)

	setupMSSQLSchema(ctx, t, mc)
	seedMSSQLData(ctx, t, mc)
	t.Cleanup(func() { cleanupMSSQLData(ctx, t, mc) })

	instance := createMSSQLTestInstance(t)
	renderer := astqlmssql.New()

	result, err := astql.Select(instance.T("users")).
		Fields(instance.F("username")).
		Where(instance.C(instance.F("username"), astql.NE, instance.P("excluded"))).
		Render(renderer)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	// SQL Server should use <> for not equal
	if !strings.Contains(result.SQL, "<>") {
		t.Errorf("Expected <> in SQL, got: %s", result.SQL)
	}

	sql, args := convertMSSQLParams(result.SQL, map[string]any{"excluded": "charlie"})
	rows := mc.Query(ctx, t, sql, args...)
	defer rows.Close()

	count := 0
	for rows.Next() {
		count++
	}
	if count != 3 {
		t.Errorf("Expected 3 users, got %d", count)
	}
}
