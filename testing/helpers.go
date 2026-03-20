// Package testing provides test utilities for astql.
package testing

import (
	"testing"

	"github.com/zoobz-io/astql"
	"github.com/zoobz-io/dbml"
)

// TestInstance creates a fully-featured ASTQL instance for testing.
// Includes users, posts, comments, orders, and products tables.
func TestInstance(t *testing.T) *astql.ASTQL {
	t.Helper()

	project := dbml.NewProject("test")

	// Users table
	users := dbml.NewTable("users")
	users.AddColumn(dbml.NewColumn("id", "bigint"))
	users.AddColumn(dbml.NewColumn("username", "varchar"))
	users.AddColumn(dbml.NewColumn("email", "varchar"))
	users.AddColumn(dbml.NewColumn("age", "int"))
	users.AddColumn(dbml.NewColumn("active", "boolean"))
	users.AddColumn(dbml.NewColumn("created_at", "timestamp"))
	users.AddColumn(dbml.NewColumn("metadata", "jsonb"))
	users.AddColumn(dbml.NewColumn("tags", "text[]"))
	users.AddColumn(dbml.NewColumn("embedding", "vector"))
	project.AddTable(users)

	// Posts table
	posts := dbml.NewTable("posts")
	posts.AddColumn(dbml.NewColumn("id", "bigint"))
	posts.AddColumn(dbml.NewColumn("user_id", "bigint"))
	posts.AddColumn(dbml.NewColumn("title", "varchar"))
	posts.AddColumn(dbml.NewColumn("body", "text"))
	posts.AddColumn(dbml.NewColumn("published", "boolean"))
	posts.AddColumn(dbml.NewColumn("views", "int"))
	posts.AddColumn(dbml.NewColumn("created_at", "timestamp"))
	project.AddTable(posts)

	// Comments table
	comments := dbml.NewTable("comments")
	comments.AddColumn(dbml.NewColumn("id", "bigint"))
	comments.AddColumn(dbml.NewColumn("post_id", "bigint"))
	comments.AddColumn(dbml.NewColumn("user_id", "bigint"))
	comments.AddColumn(dbml.NewColumn("body", "text"))
	comments.AddColumn(dbml.NewColumn("created_at", "timestamp"))
	project.AddTable(comments)

	// Orders table
	orders := dbml.NewTable("orders")
	orders.AddColumn(dbml.NewColumn("id", "bigint"))
	orders.AddColumn(dbml.NewColumn("user_id", "bigint"))
	orders.AddColumn(dbml.NewColumn("total", "numeric"))
	orders.AddColumn(dbml.NewColumn("status", "varchar"))
	orders.AddColumn(dbml.NewColumn("created_at", "timestamp"))
	project.AddTable(orders)

	// Products table
	products := dbml.NewTable("products")
	products.AddColumn(dbml.NewColumn("id", "bigint"))
	products.AddColumn(dbml.NewColumn("name", "varchar"))
	products.AddColumn(dbml.NewColumn("price", "numeric"))
	products.AddColumn(dbml.NewColumn("category", "varchar"))
	products.AddColumn(dbml.NewColumn("stock", "int"))
	project.AddTable(products)

	instance, err := astql.NewFromDBML(project)
	if err != nil {
		t.Fatalf("Failed to create test instance: %v", err)
	}
	return instance
}

// AssertSQL compares expected and actual SQL, reporting detailed differences.
func AssertSQL(t *testing.T, expected, actual string) {
	t.Helper()
	if expected != actual {
		t.Errorf("SQL mismatch:\nExpected: %s\nActual:   %s", expected, actual)
	}
}

// AssertParams checks that the required params match expected values.
func AssertParams(t *testing.T, expected, actual []string) {
	t.Helper()
	if len(expected) != len(actual) {
		t.Errorf("Param count mismatch: expected %d, got %d\nExpected: %v\nActual: %v",
			len(expected), len(actual), expected, actual)
		return
	}

	expectedMap := make(map[string]bool)
	for _, p := range expected {
		expectedMap[p] = true
	}

	for _, p := range actual {
		if !expectedMap[p] {
			t.Errorf("Unexpected param: %s\nExpected: %v\nActual: %v", p, expected, actual)
		}
	}
}

// AssertContainsParam checks that a specific param is in the list.
func AssertContainsParam(t *testing.T, params []string, param string) {
	t.Helper()
	for _, p := range params {
		if p == param {
			return
		}
	}
	t.Errorf("Expected param %q not found in %v", param, params)
}

// AssertNoError fails the test if err is not nil.
func AssertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

// AssertError fails the test if err is nil.
func AssertError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("Expected error but got nil")
	}
}

// AssertErrorContains checks that error message contains substring.
func AssertErrorContains(t *testing.T, err error, substr string) {
	t.Helper()
	if err == nil {
		t.Fatalf("Expected error containing %q but got nil", substr)
	}
	if !containsString(err.Error(), substr) {
		t.Errorf("Expected error containing %q, got: %v", substr, err)
	}
}

// AssertPanics verifies that a function panics.
func AssertPanics(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic but function completed normally")
		}
	}()
	fn()
}

// AssertPanicsWithMessage verifies that a function panics with a specific message.
func AssertPanicsWithMessage(t *testing.T, fn func(), substr string) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("Expected panic containing %q but function completed normally", substr)
			return
		}
		var msg string
		switch v := r.(type) {
		case error:
			msg = v.Error()
		case string:
			msg = v
		default:
			t.Errorf("Panic value is not string or error: %T", r)
			return
		}
		if !containsString(msg, substr) {
			t.Errorf("Expected panic containing %q, got: %s", substr, msg)
		}
	}()
	fn()
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || substr == "" ||
		(s != "" && substr != "" && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
