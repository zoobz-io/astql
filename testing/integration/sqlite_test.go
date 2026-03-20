// Package integration provides integration tests for astql using real databases.
package integration

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/zoobz-io/astql"
	"github.com/zoobz-io/astql/sqlite"
	"github.com/zoobz-io/dbml"
)

// SQLiteDB wraps an in-memory SQLite database for testing.
type SQLiteDB struct {
	db *sql.DB
}

// NewSQLiteDB creates a new in-memory SQLite database.
func NewSQLiteDB(t *testing.T) *SQLiteDB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open SQLite: %v", err)
	}

	return &SQLiteDB{db: db}
}

// Close closes the SQLite database.
func (s *SQLiteDB) Close(t *testing.T) {
	t.Helper()
	if s.db != nil {
		if err := s.db.Close(); err != nil {
			t.Logf("Warning: failed to close database: %v", err)
		}
	}
}

// Exec executes a SQL statement.
func (s *SQLiteDB) Exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	_, err := s.db.Exec(sql, args...)
	if err != nil {
		t.Fatalf("Failed to execute SQL: %v\nSQL: %s", err, sql)
	}
}

// QueryRow executes a query and returns a single row.
func (s *SQLiteDB) QueryRow(t *testing.T, sql string, args ...any) *sql.Row {
	t.Helper()
	return s.db.QueryRow(sql, args...)
}

// Query executes a query and returns rows.
func (s *SQLiteDB) Query(t *testing.T, sql string, args ...any) *sql.Rows {
	t.Helper()
	rows, err := s.db.Query(sql, args...)
	if err != nil {
		t.Fatalf("Failed to execute query: %v\nSQL: %s", err, sql)
	}
	return rows
}

// createSQLiteTestInstance creates an ASTQL instance matching the SQLite test schema.
func createSQLiteTestInstance(t *testing.T) *astql.ASTQL {
	t.Helper()

	project := dbml.NewProject("test")

	users := dbml.NewTable("users")
	users.AddColumn(dbml.NewColumn("id", "integer"))
	users.AddColumn(dbml.NewColumn("username", "text"))
	users.AddColumn(dbml.NewColumn("email", "text"))
	users.AddColumn(dbml.NewColumn("age", "integer"))
	users.AddColumn(dbml.NewColumn("active", "integer"))
	project.AddTable(users)

	posts := dbml.NewTable("posts")
	posts.AddColumn(dbml.NewColumn("id", "integer"))
	posts.AddColumn(dbml.NewColumn("user_id", "integer"))
	posts.AddColumn(dbml.NewColumn("title", "text"))
	posts.AddColumn(dbml.NewColumn("views", "integer"))
	posts.AddColumn(dbml.NewColumn("published", "integer"))
	project.AddTable(posts)

	orders := dbml.NewTable("orders")
	orders.AddColumn(dbml.NewColumn("id", "integer"))
	orders.AddColumn(dbml.NewColumn("user_id", "integer"))
	orders.AddColumn(dbml.NewColumn("total", "real"))
	orders.AddColumn(dbml.NewColumn("status", "text"))
	project.AddTable(orders)

	instance, err := astql.NewFromDBML(project)
	if err != nil {
		t.Fatalf("Failed to create instance: %v", err)
	}
	return instance
}

// setupSQLiteSchema creates the test database schema.
func setupSQLiteSchema(t *testing.T, db *SQLiteDB) {
	t.Helper()

	db.Exec(t, `
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL,
			email TEXT NOT NULL,
			age INTEGER,
			active INTEGER DEFAULT 1
		)
	`)

	db.Exec(t, `
		CREATE TABLE posts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
			title TEXT NOT NULL,
			views INTEGER DEFAULT 0,
			published INTEGER DEFAULT 0
		)
	`)

	db.Exec(t, `
		CREATE TABLE orders (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
			total REAL NOT NULL,
			status TEXT DEFAULT 'pending'
		)
	`)
}

// seedSQLiteData inserts test data.
func seedSQLiteData(t *testing.T, db *SQLiteDB) {
	t.Helper()

	db.Exec(t, `
		INSERT INTO users (id, username, email, age, active) VALUES
		(1, 'alice', 'alice@example.com', 30, 1),
		(2, 'bob', 'bob@example.com', 25, 1),
		(3, 'charlie', 'charlie@example.com', 35, 0),
		(4, 'diana', 'diana@example.com', 28, 1)
	`)

	db.Exec(t, `
		INSERT INTO posts (id, user_id, title, views, published) VALUES
		(1, 1, 'First Post', 100, 1),
		(2, 1, 'Second Post', 50, 1),
		(3, 2, 'Bobs Post', 75, 1),
		(4, 3, 'Draft Post', 0, 0)
	`)

	db.Exec(t, `
		INSERT INTO orders (id, user_id, total, status) VALUES
		(1, 1, 99.99, 'completed'),
		(2, 1, 149.99, 'completed'),
		(3, 2, 49.99, 'pending'),
		(4, 4, 199.99, 'completed')
	`)
}

// convertSQLiteParams converts astql named parameters to SQLite positional parameters.
func convertSQLiteParams(sqlStr string, params map[string]any) (string, []any) {
	args := make([]any, 0)
	paramNum := 1

	convertedSQL := sqlStr
	for name, value := range params {
		placeholder := ":" + name
		if strings.Contains(convertedSQL, placeholder) {
			convertedSQL = strings.Replace(convertedSQL, placeholder, fmt.Sprintf("?%d", paramNum), 1)
			args = append(args, value)
			paramNum++
		}
	}

	return convertedSQL, args
}

// TestSQLiteIntegration_BasicSelect tests basic SELECT queries against SQLite.
func TestSQLiteIntegration_BasicSelect(t *testing.T) {
	db := NewSQLiteDB(t)
	defer db.Close(t)

	setupSQLiteSchema(t, db)
	seedSQLiteData(t, db)

	instance := createSQLiteTestInstance(t)
	renderer := sqlite.New()

	// Test: Select all users
	result, err := astql.Select(instance.T("users")).Render(renderer)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	rows := db.Query(t, result.SQL)
	defer rows.Close()

	count := 0
	for rows.Next() {
		count++
	}
	if count != 4 {
		t.Errorf("Expected 4 users, got %d", count)
	}
}

// TestSQLiteIntegration_SelectWithWhere tests SELECT with WHERE clause.
func TestSQLiteIntegration_SelectWithWhere(t *testing.T) {
	db := NewSQLiteDB(t)
	defer db.Close(t)

	setupSQLiteSchema(t, db)
	seedSQLiteData(t, db)

	instance := createSQLiteTestInstance(t)
	renderer := sqlite.New()

	// Test: Select users with age > 27
	result, err := astql.Select(instance.T("users")).
		Fields(instance.F("username")).
		Where(instance.C(instance.F("age"), astql.GT, instance.P("min_age"))).
		Render(renderer)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	params := map[string]any{"min_age": 27}
	sql, args := convertSQLiteParams(result.SQL, params)

	rows := db.Query(t, sql, args...)
	defer rows.Close()

	var usernames []string
	for rows.Next() {
		var username string
		if err := rows.Scan(&username); err != nil {
			t.Fatalf("Scan failed: %v", err)
		}
		usernames = append(usernames, username)
	}

	// alice (30), charlie (35), diana (28) should match
	if len(usernames) != 3 {
		t.Errorf("Expected 3 users, got %d: %v", len(usernames), usernames)
	}
}

// TestSQLiteIntegration_Insert tests INSERT operations.
func TestSQLiteIntegration_Insert(t *testing.T) {
	db := NewSQLiteDB(t)
	defer db.Close(t)

	setupSQLiteSchema(t, db)

	instance := createSQLiteTestInstance(t)
	renderer := sqlite.New()

	vm := instance.ValueMap()
	vm[instance.F("username")] = instance.P("new_user")
	vm[instance.F("email")] = instance.P("new_email")
	vm[instance.F("age")] = instance.P("new_age")
	vm[instance.F("active")] = instance.P("new_active")

	result, err := astql.Insert(instance.T("users")).
		Values(vm).
		Render(renderer)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	params := map[string]any{
		"new_user":   "eve",
		"new_email":  "eve@example.com",
		"new_age":    22,
		"new_active": 1,
	}
	sql, args := convertSQLiteParams(result.SQL, params)

	db.Exec(t, sql, args...)

	// Verify insert
	var count int
	row := db.QueryRow(t, "SELECT COUNT(*) FROM users WHERE username = 'eve'")
	if err := row.Scan(&count); err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 user named 'eve', got %d", count)
	}
}

// TestSQLiteIntegration_Update tests UPDATE operations.
func TestSQLiteIntegration_Update(t *testing.T) {
	db := NewSQLiteDB(t)
	defer db.Close(t)

	setupSQLiteSchema(t, db)
	seedSQLiteData(t, db)

	instance := createSQLiteTestInstance(t)
	renderer := sqlite.New()

	result, err := astql.Update(instance.T("users")).
		Set(instance.F("age"), instance.P("new_age")).
		Where(instance.C(instance.F("username"), astql.EQ, instance.P("target_user"))).
		Render(renderer)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	params := map[string]any{
		"new_age":     99,
		"target_user": "alice",
	}
	sql, args := convertSQLiteParams(result.SQL, params)

	db.Exec(t, sql, args...)

	// Verify update
	var age int
	row := db.QueryRow(t, "SELECT age FROM users WHERE username = 'alice'")
	if err := row.Scan(&age); err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if age != 99 {
		t.Errorf("Expected age 99, got %d", age)
	}
}

// TestSQLiteIntegration_Delete tests DELETE operations.
func TestSQLiteIntegration_Delete(t *testing.T) {
	db := NewSQLiteDB(t)
	defer db.Close(t)

	setupSQLiteSchema(t, db)
	seedSQLiteData(t, db)

	instance := createSQLiteTestInstance(t)
	renderer := sqlite.New()

	result, err := astql.Delete(instance.T("users")).
		Where(instance.C(instance.F("username"), astql.EQ, instance.P("target_user"))).
		Render(renderer)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	params := map[string]any{"target_user": "charlie"}
	sql, args := convertSQLiteParams(result.SQL, params)

	db.Exec(t, sql, args...)

	// Verify delete
	var count int
	row := db.QueryRow(t, "SELECT COUNT(*) FROM users WHERE username = 'charlie'")
	if err := row.Scan(&count); err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 users named 'charlie', got %d", count)
	}
}

// TestSQLiteIntegration_Count tests COUNT operations.
func TestSQLiteIntegration_Count(t *testing.T) {
	db := NewSQLiteDB(t)
	defer db.Close(t)

	setupSQLiteSchema(t, db)
	seedSQLiteData(t, db)

	instance := createSQLiteTestInstance(t)
	renderer := sqlite.New()

	result, err := astql.Count(instance.T("users")).
		Where(instance.C(instance.F("active"), astql.EQ, instance.P("is_active"))).
		Render(renderer)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	params := map[string]any{"is_active": 1}
	sql, args := convertSQLiteParams(result.SQL, params)

	var count int
	row := db.QueryRow(t, sql, args...)
	if err := row.Scan(&count); err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if count != 3 {
		t.Errorf("Expected 3 active users, got %d", count)
	}
}

// TestSQLiteIntegration_Join tests JOIN operations.
func TestSQLiteIntegration_Join(t *testing.T) {
	db := NewSQLiteDB(t)
	defer db.Close(t)

	setupSQLiteSchema(t, db)
	seedSQLiteData(t, db)

	instance := createSQLiteTestInstance(t)
	renderer := sqlite.New()

	users := instance.T("users", "u")
	posts := instance.T("posts", "p")

	result, err := astql.Select(users).
		Fields(
			instance.WithTable(instance.F("username"), "u"),
			instance.WithTable(instance.F("title"), "p"),
		).
		InnerJoin(posts,
			astql.CF(
				instance.WithTable(instance.F("id"), "u"),
				astql.EQ,
				instance.WithTable(instance.F("user_id"), "p"),
			),
		).
		Render(renderer)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	rows := db.Query(t, result.SQL)
	defer rows.Close()

	count := 0
	for rows.Next() {
		var username, title string
		if err := rows.Scan(&username, &title); err != nil {
			t.Fatalf("Scan failed: %v", err)
		}
		count++
	}
	if count != 4 {
		t.Errorf("Expected 4 rows from join, got %d", count)
	}
}

// TestSQLiteIntegration_GroupByHaving tests GROUP BY with HAVING.
func TestSQLiteIntegration_GroupByHaving(t *testing.T) {
	db := NewSQLiteDB(t)
	defer db.Close(t)

	setupSQLiteSchema(t, db)
	seedSQLiteData(t, db)

	instance := createSQLiteTestInstance(t)
	renderer := sqlite.New()

	result, err := astql.Select(instance.T("posts")).
		Fields(instance.F("user_id")).
		SelectExpr(astql.As(astql.CountStar(), "post_count")).
		GroupBy(instance.F("user_id")).
		HavingAgg(astql.HavingCount(astql.GT, instance.P("min_posts"))).
		Render(renderer)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	params := map[string]any{"min_posts": 1}
	sql, args := convertSQLiteParams(result.SQL, params)

	rows := db.Query(t, sql, args...)
	defer rows.Close()

	count := 0
	for rows.Next() {
		count++
	}
	// Only user_id=1 has more than 1 post
	if count != 1 {
		t.Errorf("Expected 1 user with >1 posts, got %d", count)
	}
}

// TestSQLiteIntegration_OrderByLimit tests ORDER BY with LIMIT.
func TestSQLiteIntegration_OrderByLimit(t *testing.T) {
	db := NewSQLiteDB(t)
	defer db.Close(t)

	setupSQLiteSchema(t, db)
	seedSQLiteData(t, db)

	instance := createSQLiteTestInstance(t)
	renderer := sqlite.New()

	result, err := astql.Select(instance.T("users")).
		Fields(instance.F("username")).
		OrderBy(instance.F("age"), astql.DESC).
		Limit(2).
		Render(renderer)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	rows := db.Query(t, result.SQL)
	defer rows.Close()

	var usernames []string
	for rows.Next() {
		var username string
		if err := rows.Scan(&username); err != nil {
			t.Fatalf("Scan failed: %v", err)
		}
		usernames = append(usernames, username)
	}

	if len(usernames) != 2 {
		t.Errorf("Expected 2 users, got %d", len(usernames))
	}
	// charlie (35), alice (30) should be first two by age DESC
	if usernames[0] != "charlie" {
		t.Errorf("Expected first user 'charlie', got %q", usernames[0])
	}
}

// TestSQLiteIntegration_Union tests UNION operations.
func TestSQLiteIntegration_Union(t *testing.T) {
	db := NewSQLiteDB(t)
	defer db.Close(t)

	setupSQLiteSchema(t, db)
	seedSQLiteData(t, db)

	instance := createSQLiteTestInstance(t)
	renderer := sqlite.New()

	// Union: users with age > 30 UNION users who are inactive
	// Note: renderer adds q0_, q1_ prefixes automatically to compound query params
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
	sql, args := convertSQLiteParams(result.SQL, params)

	rows := db.Query(t, sql, args...)
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

// TestSQLiteIntegration_WindowFunction tests window functions.
func TestSQLiteIntegration_WindowFunction(t *testing.T) {
	db := NewSQLiteDB(t)
	defer db.Close(t)

	setupSQLiteSchema(t, db)
	seedSQLiteData(t, db)

	instance := createSQLiteTestInstance(t)
	renderer := sqlite.New()

	result, err := astql.Select(instance.T("posts")).
		Fields(instance.F("title")).
		SelectExpr(
			astql.RowNumber().
				PartitionBy(instance.F("user_id")).
				OrderBy(instance.F("views"), astql.DESC).
				As("rank"),
		).
		Render(renderer)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	rows := db.Query(t, result.SQL)
	defer rows.Close()

	count := 0
	for rows.Next() {
		var title string
		var rank int
		if err := rows.Scan(&title, &rank); err != nil {
			t.Fatalf("Scan failed: %v", err)
		}
		count++
	}
	if count != 4 {
		t.Errorf("Expected 4 rows, got %d", count)
	}
}

// TestSQLiteIntegration_Between tests BETWEEN condition.
func TestSQLiteIntegration_Between(t *testing.T) {
	db := NewSQLiteDB(t)
	defer db.Close(t)

	setupSQLiteSchema(t, db)
	seedSQLiteData(t, db)

	instance := createSQLiteTestInstance(t)
	renderer := sqlite.New()

	result, err := astql.Select(instance.T("users")).
		Fields(instance.F("username")).
		Where(astql.Between(instance.F("age"), instance.P("min_age"), instance.P("max_age"))).
		Render(renderer)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	params := map[string]any{
		"min_age": 25,
		"max_age": 30,
	}
	sql, args := convertSQLiteParams(result.SQL, params)

	rows := db.Query(t, sql, args...)
	defer rows.Close()

	var usernames []string
	for rows.Next() {
		var username string
		if err := rows.Scan(&username); err != nil {
			t.Fatalf("Scan failed: %v", err)
		}
		usernames = append(usernames, username)
	}

	// bob (25), alice (30), diana (28) - all between 25 and 30
	if len(usernames) != 3 {
		t.Errorf("Expected 3 users, got %d: %v", len(usernames), usernames)
	}
}
