// Package integration provides integration tests for astql using real PostgreSQL.
package integration

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/zoobz-io/astql"
	pgrenderer "github.com/zoobz-io/astql/postgres"
	"github.com/zoobz-io/dbml"
)

// PostgresContainer wraps a testcontainers PostgreSQL instance.
type PostgresContainer struct {
	container *postgres.PostgresContainer
	conn      *pgx.Conn
	connStr   string
}

// Exec executes a SQL statement.
func (pc *PostgresContainer) Exec(ctx context.Context, t *testing.T, sql string, args ...any) {
	t.Helper()
	_, err := pc.conn.Exec(ctx, sql, args...)
	if err != nil {
		t.Fatalf("Failed to execute SQL: %v\nSQL: %s", err, sql)
	}
}

// QueryRow executes a query and scans a single row.
func (pc *PostgresContainer) QueryRow(ctx context.Context, t *testing.T, sql string, args ...any) pgx.Row {
	t.Helper()
	return pc.conn.QueryRow(ctx, sql, args...)
}

// Query executes a query and returns rows.
func (pc *PostgresContainer) Query(ctx context.Context, t *testing.T, sql string, args ...any) pgx.Rows {
	t.Helper()
	rows, err := pc.conn.Query(ctx, sql, args...)
	if err != nil {
		t.Fatalf("Failed to execute query: %v\nSQL: %s", err, sql)
	}
	return rows
}

// createTestInstance creates an ASTQL instance matching the test database schema.
func createTestInstance(t *testing.T) *astql.ASTQL {
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
	orders.AddColumn(dbml.NewColumn("total", "numeric"))
	orders.AddColumn(dbml.NewColumn("status", "varchar"))
	project.AddTable(orders)

	instance, err := astql.NewFromDBML(project)
	if err != nil {
		t.Fatalf("Failed to create instance: %v", err)
	}
	return instance
}

// setupSchema creates the test database schema.
func setupSchema(ctx context.Context, t *testing.T, pc *PostgresContainer) {
	t.Helper()

	pc.Exec(ctx, t, `
		CREATE TABLE IF NOT EXISTS users (
			id BIGSERIAL PRIMARY KEY,
			username VARCHAR(255) NOT NULL,
			email VARCHAR(255) NOT NULL,
			age INT,
			active BOOLEAN DEFAULT true
		)
	`)

	pc.Exec(ctx, t, `
		CREATE TABLE IF NOT EXISTS posts (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
			title VARCHAR(255) NOT NULL,
			views INT DEFAULT 0,
			published BOOLEAN DEFAULT false
		)
	`)

	pc.Exec(ctx, t, `
		CREATE TABLE IF NOT EXISTS orders (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
			total NUMERIC(10,2) NOT NULL,
			status VARCHAR(50) DEFAULT 'pending'
		)
	`)
}

// seedData inserts test data.
func seedData(ctx context.Context, t *testing.T, pc *PostgresContainer) {
	t.Helper()

	// Insert users
	pc.Exec(ctx, t, `
		INSERT INTO users (id, username, email, age, active) VALUES
		(1, 'alice', 'alice@example.com', 30, true),
		(2, 'bob', 'bob@example.com', 25, true),
		(3, 'charlie', 'charlie@example.com', 35, false),
		(4, 'diana', 'diana@example.com', 28, true)
	`)

	// Insert posts
	pc.Exec(ctx, t, `
		INSERT INTO posts (id, user_id, title, views, published) VALUES
		(1, 1, 'First Post', 100, true),
		(2, 1, 'Second Post', 50, true),
		(3, 2, 'Bob''s Post', 75, true),
		(4, 3, 'Draft Post', 0, false)
	`)

	// Insert orders
	pc.Exec(ctx, t, `
		INSERT INTO orders (id, user_id, total, status) VALUES
		(1, 1, 99.99, 'completed'),
		(2, 1, 149.99, 'completed'),
		(3, 2, 49.99, 'pending'),
		(4, 4, 199.99, 'completed')
	`)
}

// cleanupData removes all test data to ensure test isolation.
func cleanupData(ctx context.Context, t *testing.T, pc *PostgresContainer) {
	t.Helper()
	pc.Exec(ctx, t, `TRUNCATE TABLE orders, posts, users RESTART IDENTITY CASCADE`)
}

// convertParams converts astql named parameters to pgx positional parameters.
// Returns the modified SQL and ordered arguments.
func convertParams(sql string, params map[string]any) (convertedSQL string, args []any) {
	args = make([]any, 0)
	paramNum := 1

	convertedSQL = sql
	for name, value := range params {
		placeholder := ":" + name
		if strings.Contains(convertedSQL, placeholder) {
			convertedSQL = strings.Replace(convertedSQL, placeholder, fmt.Sprintf("$%d", paramNum), 1)
			args = append(args, value)
			paramNum++
		}
	}

	return convertedSQL, args
}

// TestIntegration_BasicSelect tests basic SELECT queries against real PostgreSQL.
func TestIntegration_BasicSelect(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	pc := getPostgresContainer(t)
	setupSchema(ctx, t, pc)
	seedData(ctx, t, pc)
	t.Cleanup(func() { cleanupData(ctx, t, pc) })

	instance := createTestInstance(t)

	// Test: Select all users
	result, err := astql.Select(instance.T("users")).Render(pgrenderer.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	rows := pc.Query(ctx, t, result.SQL)
	defer rows.Close()

	count := 0
	for rows.Next() {
		count++
	}
	if count != 4 {
		t.Errorf("Expected 4 users, got %d", count)
	}
}

// TestIntegration_SelectWithWhere tests WHERE clause against real PostgreSQL.
func TestIntegration_SelectWithWhere(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	pc := getPostgresContainer(t)
	setupSchema(ctx, t, pc)
	seedData(ctx, t, pc)
	t.Cleanup(func() { cleanupData(ctx, t, pc) })

	instance := createTestInstance(t)

	// Test: Select active users
	result, err := astql.Select(instance.T("users")).
		Fields(instance.F("username")).
		Where(instance.C(instance.F("active"), "=", instance.P("is_active"))).
		Render(pgrenderer.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	sql, args := convertParams(result.SQL, map[string]any{"is_active": true})
	rows := pc.Query(ctx, t, sql, args...)
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

// TestIntegration_Join tests JOIN operations against real PostgreSQL.
func TestIntegration_Join(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	pc := getPostgresContainer(t)
	setupSchema(ctx, t, pc)
	seedData(ctx, t, pc)
	t.Cleanup(func() { cleanupData(ctx, t, pc) })

	instance := createTestInstance(t)

	// Test: Join users with posts
	result, err := astql.Select(instance.T("users", "u")).
		Fields(
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
		Where(instance.C(
			instance.WithTable(instance.F("published"), "p"),
			"=",
			instance.P("is_published"),
		)).
		Render(pgrenderer.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	sql, args := convertParams(result.SQL, map[string]any{"is_published": true})
	rows := pc.Query(ctx, t, sql, args...)
	defer rows.Close()

	count := 0
	for rows.Next() {
		var username, title string
		if err := rows.Scan(&username, &title); err != nil {
			t.Fatalf("Scan failed: %v", err)
		}
		count++
	}

	if count != 3 {
		t.Errorf("Expected 3 published posts, got %d", count)
	}
}

// TestIntegration_Aggregates tests aggregate functions against real PostgreSQL.
func TestIntegration_Aggregates(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	pc := getPostgresContainer(t)
	setupSchema(ctx, t, pc)
	seedData(ctx, t, pc)
	t.Cleanup(func() { cleanupData(ctx, t, pc) })

	instance := createTestInstance(t)

	// Test: Count posts per user with HAVING
	result, err := astql.Select(instance.T("posts")).
		Fields(instance.F("user_id")).
		SelectExpr(astql.As(astql.CountStar(), "post_count")).
		GroupBy(instance.F("user_id")).
		HavingAgg(astql.HavingCount(astql.GT, instance.P("min_count"))).
		Render(pgrenderer.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	sql, args := convertParams(result.SQL, map[string]any{"min_count": 1})
	rows := pc.Query(ctx, t, sql, args...)
	defer rows.Close()

	count := 0
	for rows.Next() {
		var userID int64
		var postCount int
		if err := rows.Scan(&userID, &postCount); err != nil {
			t.Fatalf("Scan failed: %v", err)
		}
		if postCount <= 1 {
			t.Errorf("Expected post_count > 1, got %d for user %d", postCount, userID)
		}
		count++
	}

	// Only alice has more than 1 post
	if count != 1 {
		t.Errorf("Expected 1 user with >1 posts, got %d", count)
	}
}

// TestIntegration_OrderByLimit tests ORDER BY and LIMIT against real PostgreSQL.
func TestIntegration_OrderByLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	pc := getPostgresContainer(t)
	setupSchema(ctx, t, pc)
	seedData(ctx, t, pc)
	t.Cleanup(func() { cleanupData(ctx, t, pc) })

	instance := createTestInstance(t)

	// Test: Get top 2 users by age
	result, err := astql.Select(instance.T("users")).
		Fields(instance.F("username"), instance.F("age")).
		OrderBy(instance.F("age"), astql.DESC).
		Limit(2).
		Render(pgrenderer.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	rows := pc.Query(ctx, t, result.SQL)
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

// TestIntegration_Insert tests INSERT operations against real PostgreSQL.
func TestIntegration_Insert(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	pc := getPostgresContainer(t)
	setupSchema(ctx, t, pc)
	t.Cleanup(func() { cleanupData(ctx, t, pc) })

	instance := createTestInstance(t)

	// Test: Insert a new user
	vm := instance.ValueMap()
	vm[instance.F("username")] = instance.P("username")
	vm[instance.F("email")] = instance.P("email")
	vm[instance.F("age")] = instance.P("age")

	result, err := astql.Insert(instance.T("users")).
		Values(vm).
		Returning(instance.F("id")).
		Render(pgrenderer.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	sql, args := convertParams(result.SQL, map[string]any{
		"username": "newuser",
		"email":    "newuser@example.com",
		"age":      42,
	})

	var id int64
	err = pc.conn.QueryRow(ctx, sql, args...).Scan(&id)
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	if id <= 0 {
		t.Errorf("Expected positive ID, got %d", id)
	}
}

// TestIntegration_Update tests UPDATE operations against real PostgreSQL.
func TestIntegration_Update(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	pc := getPostgresContainer(t)
	setupSchema(ctx, t, pc)
	seedData(ctx, t, pc)
	t.Cleanup(func() { cleanupData(ctx, t, pc) })

	instance := createTestInstance(t)

	// Test: Update user's age
	result, err := astql.Update(instance.T("users")).
		Set(instance.F("age"), instance.P("new_age")).
		Where(instance.C(instance.F("username"), "=", instance.P("username"))).
		Returning(instance.F("age")).
		Render(pgrenderer.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	sql, args := convertParams(result.SQL, map[string]any{
		"new_age":  99,
		"username": "alice",
	})

	var age int
	err = pc.conn.QueryRow(ctx, sql, args...).Scan(&age)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	if age != 99 {
		t.Errorf("Expected age 99, got %d", age)
	}
}

// TestIntegration_Delete tests DELETE operations against real PostgreSQL.
func TestIntegration_Delete(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	pc := getPostgresContainer(t)
	setupSchema(ctx, t, pc)
	seedData(ctx, t, pc)
	t.Cleanup(func() { cleanupData(ctx, t, pc) })

	instance := createTestInstance(t)

	// Test: Delete inactive users
	result, err := astql.Delete(instance.T("users")).
		Where(instance.C(instance.F("active"), "=", instance.P("is_active"))).
		Render(pgrenderer.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	sql, args := convertParams(result.SQL, map[string]any{"is_active": false})
	_, err = pc.conn.Exec(ctx, sql, args...)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify deletion
	var count int
	err = pc.conn.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}

	if count != 3 {
		t.Errorf("Expected 3 users after delete, got %d", count)
	}
}

// TestIntegration_Between tests BETWEEN condition against real PostgreSQL.
func TestIntegration_Between(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	pc := getPostgresContainer(t)
	setupSchema(ctx, t, pc)
	seedData(ctx, t, pc)
	t.Cleanup(func() { cleanupData(ctx, t, pc) })

	instance := createTestInstance(t)

	// Test: Select users with age between 25 and 30
	result, err := astql.Select(instance.T("users")).
		Fields(instance.F("username")).
		Where(astql.Between(instance.F("age"), instance.P("min_age"), instance.P("max_age"))).
		OrderBy(instance.F("age"), astql.ASC).
		Render(pgrenderer.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	sql, args := convertParams(result.SQL, map[string]any{
		"min_age": 25,
		"max_age": 30,
	})
	rows := pc.Query(ctx, t, sql, args...)
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

// TestIntegration_DistinctOn tests DISTINCT ON against real PostgreSQL.
func TestIntegration_DistinctOn(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	pc := getPostgresContainer(t)
	setupSchema(ctx, t, pc)
	seedData(ctx, t, pc)
	t.Cleanup(func() { cleanupData(ctx, t, pc) })

	instance := createTestInstance(t)

	// Test: Get one post per user (the one with most views)
	result, err := astql.Select(instance.T("posts")).
		DistinctOn(instance.F("user_id")).
		Fields(instance.F("user_id"), instance.F("title"), instance.F("views")).
		OrderBy(instance.F("user_id"), astql.ASC).
		OrderBy(instance.F("views"), astql.DESC).
		Render(pgrenderer.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	rows := pc.Query(ctx, t, result.SQL)
	defer rows.Close()

	count := 0
	for rows.Next() {
		count++
	}

	// Should get one post per unique user_id
	if count != 3 {
		t.Errorf("Expected 3 distinct user posts, got %d", count)
	}
}

// TestIntegration_ForUpdate tests FOR UPDATE locking against real PostgreSQL.
func TestIntegration_ForUpdate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	pc := getPostgresContainer(t)
	setupSchema(ctx, t, pc)
	seedData(ctx, t, pc)
	t.Cleanup(func() { cleanupData(ctx, t, pc) })

	instance := createTestInstance(t)

	// Start a transaction
	tx, err := pc.conn.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin transaction failed: %v", err)
	}
	defer tx.Rollback(ctx)

	// Test: Select with FOR UPDATE
	result, err := astql.Select(instance.T("users")).
		Fields(instance.F("username")).
		Where(instance.C(instance.F("id"), "=", instance.P("user_id"))).
		ForUpdate().
		Render(pgrenderer.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	sql, args := convertParams(result.SQL, map[string]any{"user_id": 1})

	var username string
	err = tx.QueryRow(ctx, sql, args...).Scan(&username)
	if err != nil {
		t.Fatalf("Select FOR UPDATE failed: %v", err)
	}

	if username != "alice" {
		t.Errorf("Expected username 'alice', got '%s'", username)
	}
}

// TestIntegration_NullsOrdering tests NULLS FIRST/LAST against real PostgreSQL.
func TestIntegration_NullsOrdering(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	pc := getPostgresContainer(t)
	setupSchema(ctx, t, pc)
	t.Cleanup(func() { cleanupData(ctx, t, pc) })

	// Insert users with NULL ages
	pc.Exec(ctx, t, `
		INSERT INTO users (id, username, email, age, active) VALUES
		(1, 'alice', 'alice@example.com', 30, true),
		(2, 'bob', 'bob@example.com', NULL, true),
		(3, 'charlie', 'charlie@example.com', 25, true)
	`)

	instance := createTestInstance(t)

	// Test: NULLS FIRST
	result, err := astql.Select(instance.T("users")).
		Fields(instance.F("username")).
		OrderByNulls(instance.F("age"), astql.ASC, astql.NullsFirst).
		Render(pgrenderer.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	rows := pc.Query(ctx, t, result.SQL)
	defer rows.Close()

	var usernames []string
	for rows.Next() {
		var username string
		if err := rows.Scan(&username); err != nil {
			t.Fatalf("Scan failed: %v", err)
		}
		usernames = append(usernames, username)
	}

	// With NULLS FIRST, bob should be first
	if len(usernames) != 3 || usernames[0] != "bob" {
		t.Errorf("Expected bob first with NULLS FIRST, got %v", usernames)
	}
}

// TestIntegration_CaseExpression tests CASE expressions against real PostgreSQL.
func TestIntegration_CaseExpression(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	pc := getPostgresContainer(t)
	setupSchema(ctx, t, pc)
	seedData(ctx, t, pc)
	t.Cleanup(func() { cleanupData(ctx, t, pc) })

	instance := createTestInstance(t)

	// Test: CASE expression for age groups
	caseExpr := astql.Case().
		When(instance.C(instance.F("age"), "<", instance.P("young_age")), instance.P("young_label")).
		When(instance.C(instance.F("age"), "<", instance.P("mid_age")), instance.P("mid_label")).
		Else(instance.P("senior_label")).
		As("age_group").
		Build()

	result, err := astql.Select(instance.T("users")).
		Fields(instance.F("username")).
		SelectExpr(caseExpr).
		OrderBy(instance.F("age"), astql.ASC).
		Render(pgrenderer.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	sql, args := convertParams(result.SQL, map[string]any{
		"young_age":    27,
		"young_label":  "young",
		"mid_age":      32,
		"mid_label":    "middle",
		"senior_label": "senior",
	})

	rows := pc.Query(ctx, t, sql, args...)
	defer rows.Close()

	results := make(map[string]string)
	for rows.Next() {
		var username, ageGroup string
		if err := rows.Scan(&username, &ageGroup); err != nil {
			t.Fatalf("Scan failed: %v", err)
		}
		results[username] = ageGroup
	}

	// bob (25) = young, diana (28) = middle, alice (30) = middle, charlie (35) = senior
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

// TestIntegration_WindowFunction tests window functions against real PostgreSQL.
func TestIntegration_WindowFunction(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	pc := getPostgresContainer(t)
	setupSchema(ctx, t, pc)
	seedData(ctx, t, pc)
	t.Cleanup(func() { cleanupData(ctx, t, pc) })

	instance := createTestInstance(t)

	// Test: ROW_NUMBER() over users ordered by age
	winExpr := astql.RowNumber().
		OrderBy(instance.F("age"), astql.DESC).
		As("rank")

	result, err := astql.Select(instance.T("users")).
		Fields(instance.F("username"), instance.F("age")).
		SelectExpr(winExpr).
		Render(pgrenderer.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	rows := pc.Query(ctx, t, result.SQL)
	defer rows.Close()

	type userRank struct {
		username string
		age      int
		rank     int
	}
	var results []userRank

	for rows.Next() {
		var ur userRank
		if err := rows.Scan(&ur.username, &ur.age, &ur.rank); err != nil {
			t.Fatalf("Scan failed: %v", err)
		}
		results = append(results, ur)
	}

	// Verify ranking is correct (oldest first)
	if len(results) != 4 {
		t.Fatalf("Expected 4 users, got %d", len(results))
	}

	// Find charlie (age 35) - should be rank 1
	for _, r := range results {
		if r.username == "charlie" && r.rank != 1 {
			t.Errorf("Expected charlie to be rank 1, got %d", r.rank)
		}
	}
}
