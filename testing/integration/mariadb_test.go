// Package integration provides integration tests for astql using real databases.
package integration

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/testcontainers/testcontainers-go/modules/mariadb"
	"github.com/zoobz-io/astql"
	astqlmariadb "github.com/zoobz-io/astql/mariadb"
	"github.com/zoobz-io/dbml"
)

// MariaDBContainer wraps a testcontainers MariaDB instance.
type MariaDBContainer struct {
	container *mariadb.MariaDBContainer
	db        *sql.DB
	connStr   string
}

// Exec executes a SQL statement.
func (mc *MariaDBContainer) Exec(ctx context.Context, t *testing.T, sql string, args ...any) {
	t.Helper()
	_, err := mc.db.ExecContext(ctx, sql, args...)
	if err != nil {
		t.Fatalf("Failed to execute SQL: %v\nSQL: %s", err, sql)
	}
}

// QueryRow executes a query and returns a single row.
func (mc *MariaDBContainer) QueryRow(ctx context.Context, t *testing.T, sql string, args ...any) *sql.Row {
	t.Helper()
	return mc.db.QueryRowContext(ctx, sql, args...)
}

// Query executes a query and returns rows.
func (mc *MariaDBContainer) Query(ctx context.Context, t *testing.T, sql string, args ...any) *sql.Rows {
	t.Helper()
	rows, err := mc.db.QueryContext(ctx, sql, args...)
	if err != nil {
		t.Fatalf("Failed to execute query: %v\nSQL: %s", err, sql)
	}
	return rows
}

// createMariaDBTestInstance creates an ASTQL instance matching the test database schema.
func createMariaDBTestInstance(t *testing.T) *astql.ASTQL {
	t.Helper()

	project := dbml.NewProject("test")

	users := dbml.NewTable("users")
	users.AddColumn(dbml.NewColumn("id", "bigint"))
	users.AddColumn(dbml.NewColumn("username", "varchar"))
	users.AddColumn(dbml.NewColumn("email", "varchar"))
	users.AddColumn(dbml.NewColumn("age", "int"))
	users.AddColumn(dbml.NewColumn("active", "boolean"))
	project.AddTable(users)

	posts := dbml.NewTable("posts")
	posts.AddColumn(dbml.NewColumn("id", "bigint"))
	posts.AddColumn(dbml.NewColumn("user_id", "bigint"))
	posts.AddColumn(dbml.NewColumn("title", "varchar"))
	posts.AddColumn(dbml.NewColumn("views", "int"))
	posts.AddColumn(dbml.NewColumn("published", "boolean"))
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

// setupMariaDBSchema creates the test database schema.
func setupMariaDBSchema(ctx context.Context, t *testing.T, mc *MariaDBContainer) {
	t.Helper()

	mc.Exec(ctx, t, `
		CREATE TABLE IF NOT EXISTS users (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			username VARCHAR(255) NOT NULL,
			email VARCHAR(255) NOT NULL UNIQUE,
			age INT,
			active BOOLEAN DEFAULT true
		)
	`)

	mc.Exec(ctx, t, `
		CREATE TABLE IF NOT EXISTS posts (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			user_id BIGINT,
			title VARCHAR(255) NOT NULL,
			views INT DEFAULT 0,
			published BOOLEAN DEFAULT false,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)
	`)

	mc.Exec(ctx, t, `
		CREATE TABLE IF NOT EXISTS orders (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			user_id BIGINT,
			total DECIMAL(10,2) NOT NULL,
			status VARCHAR(50) DEFAULT 'pending',
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)
	`)
}

// seedMariaDBData inserts test data.
func seedMariaDBData(ctx context.Context, t *testing.T, mc *MariaDBContainer) {
	t.Helper()

	mc.Exec(ctx, t, `
		INSERT INTO users (id, username, email, age, active) VALUES
		(1, 'alice', 'alice@example.com', 30, true),
		(2, 'bob', 'bob@example.com', 25, true),
		(3, 'charlie', 'charlie@example.com', 35, false),
		(4, 'diana', 'diana@example.com', 28, true)
	`)

	mc.Exec(ctx, t, `
		INSERT INTO posts (id, user_id, title, views, published) VALUES
		(1, 1, 'First Post', 100, true),
		(2, 1, 'Second Post', 50, true),
		(3, 2, 'Bobs Post', 75, true),
		(4, 3, 'Draft Post', 0, false)
	`)

	mc.Exec(ctx, t, `
		INSERT INTO orders (id, user_id, total, status) VALUES
		(1, 1, 99.99, 'completed'),
		(2, 1, 149.99, 'completed'),
		(3, 2, 49.99, 'pending'),
		(4, 4, 199.99, 'completed')
	`)
}

// cleanupMariaDBData removes all test data to ensure test isolation.
func cleanupMariaDBData(ctx context.Context, t *testing.T, mc *MariaDBContainer) {
	t.Helper()
	mc.Exec(ctx, t, `SET FOREIGN_KEY_CHECKS = 0`)
	mc.Exec(ctx, t, `TRUNCATE TABLE orders`)
	mc.Exec(ctx, t, `TRUNCATE TABLE posts`)
	mc.Exec(ctx, t, `TRUNCATE TABLE users`)
	mc.Exec(ctx, t, `SET FOREIGN_KEY_CHECKS = 1`)
}

// convertMariaDBParams converts astql named parameters to MariaDB positional parameters.
// Parameters are extracted in the order they appear in the SQL string.
func convertMariaDBParams(sqlStr string, params map[string]any) (string, []any) {
	args := make([]any, 0)
	result := strings.Builder{}

	i := 0
	for i < len(sqlStr) {
		if sqlStr[i] == ':' {
			// Find end of parameter name
			j := i + 1
			for j < len(sqlStr) && (isAlphaNumeric(sqlStr[j]) || sqlStr[j] == '_') {
				j++
			}
			if j > i+1 {
				paramName := sqlStr[i+1 : j]
				if value, ok := params[paramName]; ok {
					result.WriteByte('?')
					args = append(args, value)
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

func isAlphaNumeric(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// TestMariaDBIntegration_BasicSelect tests basic SELECT queries against MariaDB.
func TestMariaDBIntegration_BasicSelect(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	mc := getMariaDBContainer(t)

	setupMariaDBSchema(ctx, t, mc)
	seedMariaDBData(ctx, t, mc)
	t.Cleanup(func() { cleanupMariaDBData(ctx, t, mc) })

	instance := createMariaDBTestInstance(t)
	renderer := astqlmariadb.New()

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

// TestMariaDBIntegration_SelectWithWhere tests WHERE clause against MariaDB.
func TestMariaDBIntegration_SelectWithWhere(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	mc := getMariaDBContainer(t)

	setupMariaDBSchema(ctx, t, mc)
	seedMariaDBData(ctx, t, mc)
	t.Cleanup(func() { cleanupMariaDBData(ctx, t, mc) })

	instance := createMariaDBTestInstance(t)
	renderer := astqlmariadb.New()

	result, err := astql.Select(instance.T("users")).
		Fields(instance.F("username")).
		Where(instance.C(instance.F("active"), astql.EQ, instance.P("is_active"))).
		Render(renderer)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	sql, args := convertMariaDBParams(result.SQL, map[string]any{"is_active": true})
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

// TestMariaDBIntegration_Insert tests INSERT operations against MariaDB.
func TestMariaDBIntegration_Insert(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	mc := getMariaDBContainer(t)

	setupMariaDBSchema(ctx, t, mc)
	t.Cleanup(func() { cleanupMariaDBData(ctx, t, mc) })

	instance := createMariaDBTestInstance(t)
	renderer := astqlmariadb.New()

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

	sql, args := convertMariaDBParams(result.SQL, map[string]any{
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

// TestMariaDBIntegration_OnDuplicateKeyUpdate tests upsert against MariaDB.
func TestMariaDBIntegration_OnDuplicateKeyUpdate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	mc := getMariaDBContainer(t)

	setupMariaDBSchema(ctx, t, mc)
	seedMariaDBData(ctx, t, mc)
	t.Cleanup(func() { cleanupMariaDBData(ctx, t, mc) })

	instance := createMariaDBTestInstance(t)
	renderer := astqlmariadb.New()

	vm := instance.ValueMap()
	vm[instance.F("username")] = instance.P("username")
	vm[instance.F("email")] = instance.P("email")
	vm[instance.F("age")] = instance.P("age")

	result, err := astql.Insert(instance.T("users")).
		Values(vm).
		OnConflict(instance.F("email")).
		DoUpdate().
		Set(instance.F("age"), instance.P("new_age")).
		Build().
		Render(renderer)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	// Try to insert with existing email, should update age
	sql, args := convertMariaDBParams(result.SQL, map[string]any{
		"username": "alice_dup",
		"email":    "alice@example.com", // existing email
		"age":      99,
		"new_age":  99,
	})

	mc.Exec(ctx, t, sql, args...)

	// Verify update happened
	var age int
	row := mc.QueryRow(ctx, t, "SELECT age FROM users WHERE email = 'alice@example.com'")
	if err := row.Scan(&age); err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if age != 99 {
		t.Errorf("Expected age 99, got %d", age)
	}
}

// TestMariaDBIntegration_Update tests UPDATE operations against MariaDB.
func TestMariaDBIntegration_Update(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	mc := getMariaDBContainer(t)

	setupMariaDBSchema(ctx, t, mc)
	seedMariaDBData(ctx, t, mc)
	t.Cleanup(func() { cleanupMariaDBData(ctx, t, mc) })

	instance := createMariaDBTestInstance(t)
	renderer := astqlmariadb.New()

	result, err := astql.Update(instance.T("users")).
		Set(instance.F("age"), instance.P("new_age")).
		Where(instance.C(instance.F("username"), astql.EQ, instance.P("username"))).
		Render(renderer)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	sql, args := convertMariaDBParams(result.SQL, map[string]any{
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

// TestMariaDBIntegration_Delete tests DELETE operations against MariaDB.
func TestMariaDBIntegration_Delete(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	mc := getMariaDBContainer(t)

	setupMariaDBSchema(ctx, t, mc)
	seedMariaDBData(ctx, t, mc)
	t.Cleanup(func() { cleanupMariaDBData(ctx, t, mc) })

	instance := createMariaDBTestInstance(t)
	renderer := astqlmariadb.New()

	result, err := astql.Delete(instance.T("users")).
		Where(instance.C(instance.F("active"), astql.EQ, instance.P("is_active"))).
		Render(renderer)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	sql, args := convertMariaDBParams(result.SQL, map[string]any{"is_active": false})
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

// TestMariaDBIntegration_Join tests JOIN operations against MariaDB.
func TestMariaDBIntegration_Join(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	mc := getMariaDBContainer(t)

	setupMariaDBSchema(ctx, t, mc)
	seedMariaDBData(ctx, t, mc)
	t.Cleanup(func() { cleanupMariaDBData(ctx, t, mc) })

	instance := createMariaDBTestInstance(t)
	renderer := astqlmariadb.New()

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

	sql, args := convertMariaDBParams(result.SQL, map[string]any{"is_published": true})
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

// TestMariaDBIntegration_Aggregates tests aggregate functions against MariaDB.
func TestMariaDBIntegration_Aggregates(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	mc := getMariaDBContainer(t)

	setupMariaDBSchema(ctx, t, mc)
	seedMariaDBData(ctx, t, mc)
	t.Cleanup(func() { cleanupMariaDBData(ctx, t, mc) })

	instance := createMariaDBTestInstance(t)
	renderer := astqlmariadb.New()

	result, err := astql.Select(instance.T("posts")).
		Fields(instance.F("user_id")).
		SelectExpr(astql.As(astql.CountStar(), "post_count")).
		GroupBy(instance.F("user_id")).
		HavingAgg(astql.HavingCount(astql.GT, instance.P("min_count"))).
		Render(renderer)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	sql, args := convertMariaDBParams(result.SQL, map[string]any{"min_count": 1})
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

// TestMariaDBIntegration_OrderByLimit tests ORDER BY and LIMIT against MariaDB.
func TestMariaDBIntegration_OrderByLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	mc := getMariaDBContainer(t)

	setupMariaDBSchema(ctx, t, mc)
	seedMariaDBData(ctx, t, mc)
	t.Cleanup(func() { cleanupMariaDBData(ctx, t, mc) })

	instance := createMariaDBTestInstance(t)
	renderer := astqlmariadb.New()

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

// TestMariaDBIntegration_ForUpdate tests row locking against MariaDB.
func TestMariaDBIntegration_ForUpdate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	mc := getMariaDBContainer(t)

	setupMariaDBSchema(ctx, t, mc)
	seedMariaDBData(ctx, t, mc)
	t.Cleanup(func() { cleanupMariaDBData(ctx, t, mc) })

	instance := createMariaDBTestInstance(t)
	renderer := astqlmariadb.New()

	// Start a transaction
	tx, err := mc.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("Begin transaction failed: %v", err)
	}
	defer tx.Rollback()

	result, err := astql.Select(instance.T("users")).
		Fields(instance.F("username")).
		Where(instance.C(instance.F("id"), astql.EQ, instance.P("user_id"))).
		ForUpdate().
		Render(renderer)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	sql, args := convertMariaDBParams(result.SQL, map[string]any{"user_id": 1})

	var username string
	err = tx.QueryRowContext(ctx, sql, args...).Scan(&username)
	if err != nil {
		t.Fatalf("Select FOR UPDATE failed: %v", err)
	}

	if username != "alice" {
		t.Errorf("Expected username 'alice', got '%s'", username)
	}
}

// TestMariaDBIntegration_WindowFunction tests window functions against MariaDB.
func TestMariaDBIntegration_WindowFunction(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	mc := getMariaDBContainer(t)

	setupMariaDBSchema(ctx, t, mc)
	seedMariaDBData(ctx, t, mc)
	t.Cleanup(func() { cleanupMariaDBData(ctx, t, mc) })

	instance := createMariaDBTestInstance(t)
	renderer := astqlmariadb.New()

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

// TestMariaDBIntegration_CaseExpression tests CASE expressions against MariaDB.
func TestMariaDBIntegration_CaseExpression(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	mc := getMariaDBContainer(t)

	setupMariaDBSchema(ctx, t, mc)
	seedMariaDBData(ctx, t, mc)
	t.Cleanup(func() { cleanupMariaDBData(ctx, t, mc) })

	instance := createMariaDBTestInstance(t)
	renderer := astqlmariadb.New()

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

	sql, args := convertMariaDBParams(result.SQL, map[string]any{
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

// TestMariaDBIntegration_Between tests BETWEEN condition against MariaDB.
func TestMariaDBIntegration_Between(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	mc := getMariaDBContainer(t)

	setupMariaDBSchema(ctx, t, mc)
	seedMariaDBData(ctx, t, mc)
	t.Cleanup(func() { cleanupMariaDBData(ctx, t, mc) })

	instance := createMariaDBTestInstance(t)
	renderer := astqlmariadb.New()

	result, err := astql.Select(instance.T("users")).
		Fields(instance.F("username")).
		Where(astql.Between(instance.F("age"), instance.P("min_age"), instance.P("max_age"))).
		OrderBy(instance.F("age"), astql.ASC).
		Render(renderer)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	sql, args := convertMariaDBParams(result.SQL, map[string]any{
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

// TestMariaDBIntegration_DateNow tests NOW() function against MariaDB.
func TestMariaDBIntegration_DateNow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	mc := getMariaDBContainer(t)

	setupMariaDBSchema(ctx, t, mc)
	t.Cleanup(func() { cleanupMariaDBData(ctx, t, mc) })

	instance := createMariaDBTestInstance(t)
	renderer := astqlmariadb.New()

	result, err := astql.Select(instance.T("users")).
		SelectExpr(astql.As(astql.Now(), "current_time")).
		Limit(1).
		Render(renderer)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	// Just verify the query executes without error
	row := mc.QueryRow(ctx, t, result.SQL)
	var timestamp string
	if err := row.Scan(&timestamp); err != nil {
		// This might fail if no users - that's ok, just checking SQL validity
		if err != sql.ErrNoRows {
			t.Fatalf("Query failed: %v", err)
		}
	}
}

// TestMariaDBIntegration_Union tests UNION operations against MariaDB.
func TestMariaDBIntegration_Union(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	mc := getMariaDBContainer(t)

	setupMariaDBSchema(ctx, t, mc)
	seedMariaDBData(ctx, t, mc)
	t.Cleanup(func() { cleanupMariaDBData(ctx, t, mc) })

	instance := createMariaDBTestInstance(t)
	renderer := astqlmariadb.New()

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
		"q1_is_active": false,
	}
	sql, args := convertMariaDBParams(result.SQL, params)

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
