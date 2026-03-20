package astql_test

import (
	"testing"

	"github.com/zoobz-io/astql"
	"github.com/zoobz-io/astql/internal/types"
	"github.com/zoobz-io/astql/postgres"
	"github.com/zoobz-io/dbml"
)

func createTestInstance(t *testing.T) *astql.ASTQL {
	t.Helper()

	project := dbml.NewProject("test_db")

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
	project.AddTable(posts)

	instance, err := astql.NewFromDBML(project)
	if err != nil {
		t.Fatalf("Failed to create test instance: %v", err)
	}

	return instance
}

func TestNewFromDBML(t *testing.T) {
	project := dbml.NewProject("test")
	table := dbml.NewTable("users")
	table.AddColumn(dbml.NewColumn("id", "bigint"))
	project.AddTable(table)

	instance, err := astql.NewFromDBML(project)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if instance == nil {
		t.Fatal("Expected instance, got nil")
	}
}

func TestNewFromDBML_NilProject(t *testing.T) {
	_, err := astql.NewFromDBML(nil)
	if err == nil {
		t.Fatal("Expected error for nil project")
	}
}

func TestTryF_ValidField(t *testing.T) {
	instance := createTestInstance(t)

	field, err := instance.TryF("id")
	if err != nil {
		t.Fatalf("Expected no error for valid field, got: %v", err)
	}
	if field.Name != "id" {
		t.Errorf("Expected field name 'id', got '%s'", field.Name)
	}
}

func TestTryF_InvalidField(t *testing.T) {
	instance := createTestInstance(t)

	_, err := instance.TryF("nonexistent")
	if err == nil {
		t.Fatal("Expected error for invalid field")
	}
}

func TestF_ValidField(t *testing.T) {
	instance := createTestInstance(t)

	field := instance.F("username")
	if field.Name != "username" {
		t.Errorf("Expected field name 'username', got '%s'", field.Name)
	}
}

func TestF_InvalidField_Panics(t *testing.T) {
	instance := createTestInstance(t)

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for invalid field")
		}
	}()

	instance.F("nonexistent")
}

func TestTryT_ValidTable(t *testing.T) {
	instance := createTestInstance(t)

	table, err := instance.TryT("users")
	if err != nil {
		t.Fatalf("Expected no error for valid table, got: %v", err)
	}
	if table.Name != "users" {
		t.Errorf("Expected table name 'users', got '%s'", table.Name)
	}
}

func TestTryT_WithAlias(t *testing.T) {
	instance := createTestInstance(t)

	table, err := instance.TryT("users", "u")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if table.Name != "users" {
		t.Errorf("Expected table name 'users', got '%s'", table.Name)
	}
	if table.Alias != "u" {
		t.Errorf("Expected alias 'u', got '%s'", table.Alias)
	}
}

func TestTryT_InvalidTable(t *testing.T) {
	instance := createTestInstance(t)

	_, err := instance.TryT("nonexistent")
	if err == nil {
		t.Fatal("Expected error for invalid table")
	}
}

func TestTryT_InvalidAlias(t *testing.T) {
	instance := createTestInstance(t)

	_, err := instance.TryT("users", "AB")
	if err == nil {
		t.Fatal("Expected error for invalid alias (must be single lowercase letter)")
	}
}

func TestTryP_ValidParam(t *testing.T) {
	instance := createTestInstance(t)

	param, err := instance.TryP("user_id")
	if err != nil {
		t.Fatalf("Expected no error for valid param, got: %v", err)
	}
	if param.Name != "user_id" {
		t.Errorf("Expected param name 'user_id', got '%s'", param.Name)
	}
}

func TestTryP_InvalidParam(t *testing.T) {
	instance := createTestInstance(t)

	tests := []struct {
		name  string
		param string
	}{
		{"starts with number", "123abc"},
		{"contains space", "user id"},
		{"SQL injection attempt", "id; DROP TABLE"},
		{"contains comment", "id--"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := instance.TryP(tt.param)
			if err == nil {
				t.Errorf("Expected error for param '%s'", tt.param)
			}
		})
	}
}

func TestTryC_ValidCondition(t *testing.T) {
	instance := createTestInstance(t)

	field := instance.F("id")
	param := instance.P("user_id")

	cond, err := instance.TryC(field, "=", param)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if cond.Field.Name != "id" {
		t.Errorf("Expected field 'id', got '%s'", cond.Field.Name)
	}
}

func TestTryNull(t *testing.T) {
	instance := createTestInstance(t)

	field := instance.F("email")
	cond, err := instance.TryNull(field)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if cond.Operator != "IS NULL" {
		t.Errorf("Expected IS NULL operator, got '%s'", cond.Operator)
	}
}

func TestTryNotNull(t *testing.T) {
	instance := createTestInstance(t)

	field := instance.F("email")
	cond, err := instance.TryNotNull(field)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if cond.Operator != "IS NOT NULL" {
		t.Errorf("Expected IS NOT NULL operator, got '%s'", cond.Operator)
	}
}

func TestTryAnd(t *testing.T) {
	instance := createTestInstance(t)

	cond1 := instance.C(instance.F("id"), "=", instance.P("id"))
	cond2 := instance.C(instance.F("active"), "=", instance.P("active"))

	group, err := instance.TryAnd(cond1, cond2)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if group.Logic != "AND" {
		t.Errorf("Expected AND logic, got '%s'", group.Logic)
	}
	if len(group.Conditions) != 2 {
		t.Errorf("Expected 2 conditions, got %d", len(group.Conditions))
	}
}

func TestTryAnd_NoConditions(t *testing.T) {
	instance := createTestInstance(t)

	_, err := instance.TryAnd()
	if err == nil {
		t.Fatal("Expected error for AND with no conditions")
	}
}

func TestTryOr(t *testing.T) {
	instance := createTestInstance(t)

	cond1 := instance.C(instance.F("id"), "=", instance.P("id"))
	cond2 := instance.C(instance.F("active"), "=", instance.P("active"))

	group, err := instance.TryOr(cond1, cond2)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if group.Logic != "OR" {
		t.Errorf("Expected OR logic, got '%s'", group.Logic)
	}
	if len(group.Conditions) != 2 {
		t.Errorf("Expected 2 conditions, got %d", len(group.Conditions))
	}
}

func TestTryOr_NoConditions(t *testing.T) {
	instance := createTestInstance(t)

	_, err := instance.TryOr()
	if err == nil {
		t.Fatal("Expected error for OR with no conditions")
	}
}

func TestTryC_InvalidField(t *testing.T) {
	instance := createTestInstance(t)

	// Create a field that doesn't exist in the schema
	invalidField := types.Field{Name: "nonexistent_field"}
	param := instance.P("value")

	_, err := instance.TryC(invalidField, "=", param)
	if err == nil {
		t.Fatal("Expected error for invalid field")
	}
}

func TestTryNull_InvalidField(t *testing.T) {
	instance := createTestInstance(t)

	invalidField := types.Field{Name: "nonexistent_field"}

	_, err := instance.TryNull(invalidField)
	if err == nil {
		t.Fatal("Expected error for invalid field")
	}
}

func TestTryNotNull_InvalidField(t *testing.T) {
	instance := createTestInstance(t)

	invalidField := types.Field{Name: "nonexistent_field"}

	_, err := instance.TryNotNull(invalidField)
	if err == nil {
		t.Fatal("Expected error for invalid field")
	}
}

func TestWithTable(t *testing.T) {
	instance := createTestInstance(t)

	field := instance.F("id")
	fieldWithTable := instance.WithTable(field, "u")

	if fieldWithTable.Name != "id" {
		t.Errorf("Expected field name 'id', got '%s'", fieldWithTable.Name)
	}
	if fieldWithTable.Table != "u" {
		t.Errorf("Expected table 'u', got '%s'", fieldWithTable.Table)
	}
}

func TestWithTable_InvalidTable(t *testing.T) {
	instance := createTestInstance(t)

	field := instance.F("id")

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for invalid table reference")
		}
	}()

	instance.WithTable(field, "nonexistent")
}

// Test TryWithTable success case.
func TestTryWithTable_Success(t *testing.T) {
	instance := createTestInstance(t)

	field := instance.F("id")
	fieldWithTable, err := instance.TryWithTable(field, "u")

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if fieldWithTable.Name != "id" {
		t.Errorf("Expected field name 'id', got '%s'", fieldWithTable.Name)
	}
	if fieldWithTable.Table != "u" {
		t.Errorf("Expected table 'u', got '%s'", fieldWithTable.Table)
	}
}

// Test TryWithTable with invalid table.
func TestTryWithTable_InvalidTable(t *testing.T) {
	instance := createTestInstance(t)

	field := instance.F("id")
	_, err := instance.TryWithTable(field, "nonexistent")

	if err == nil {
		t.Error("Expected error for invalid table reference")
	}
}

// Test GetInstance method.
func TestGetInstance(t *testing.T) {
	instance := createTestInstance(t)

	retrieved := instance.GetInstance()

	if retrieved != instance {
		t.Error("GetInstance should return the same instance")
	}
}

// Test that factory methods enable programmatic query building.
func TestFactories_ProgrammaticFields(t *testing.T) {
	project := dbml.NewProject("test")
	users := dbml.NewTable("users")
	users.AddColumn(dbml.NewColumn("id", "bigint"))
	users.AddColumn(dbml.NewColumn("username", "varchar"))
	users.AddColumn(dbml.NewColumn("email", "varchar"))
	users.AddColumn(dbml.NewColumn("age", "int"))
	project.AddTable(users)

	instance, err := astql.NewFromDBML(project)
	if err != nil {
		t.Fatalf("Failed to create instance: %v", err)
	}

	// Simulate dynamic field selection
	dynamicFieldNames := []string{"id", "username", "email"}

	fields := instance.Fields()
	for _, name := range dynamicFieldNames {
		fields = append(fields, instance.F(name))
	}

	result, err := astql.Select(instance.T("users")).
		Fields(fields...).
		Render(postgres.New())

	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT "id", "username", "email" FROM "users"`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

func TestFactories_ProgrammaticValues(t *testing.T) {
	project := dbml.NewProject("test")
	users := dbml.NewTable("users")
	users.AddColumn(dbml.NewColumn("id", "bigint"))
	users.AddColumn(dbml.NewColumn("username", "varchar"))
	users.AddColumn(dbml.NewColumn("email", "varchar"))
	project.AddTable(users)

	instance, err := astql.NewFromDBML(project)
	if err != nil {
		t.Fatalf("Failed to create instance: %v", err)
	}

	// Simulate dynamic insert values
	// Map field names to parameter names (both must be valid identifiers)
	dynamicFields := []string{"username", "email"}

	vm := instance.ValueMap()
	for _, field := range dynamicFields {
		vm[instance.F(field)] = instance.P(field)
	}

	result, err := astql.Insert(instance.T("users")).
		Values(vm).
		Render(postgres.New())

	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	// Fields are sorted alphabetically
	expected := `INSERT INTO "users" ("email", "username") VALUES (:email, :username)`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

func TestFactories_MultiRowInsert(t *testing.T) {
	project := dbml.NewProject("test")
	users := dbml.NewTable("users")
	users.AddColumn(dbml.NewColumn("username", "varchar"))
	users.AddColumn(dbml.NewColumn("email", "varchar"))
	project.AddTable(users)

	instance, err := astql.NewFromDBML(project)
	if err != nil {
		t.Fatalf("Failed to create instance: %v", err)
	}

	// Simulate inserting multiple rows
	// Each row uses the same field names with numbered parameters
	fieldNames := []string{"username", "email"}

	query := astql.Insert(instance.T("users"))
	for i := 0; i < 3; i++ {
		vm := instance.ValueMap()
		for _, field := range fieldNames {
			vm[instance.F(field)] = instance.P(field)
		}
		query = query.Values(vm)
	}

	result, err := query.Render(postgres.New())
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `INSERT INTO "users" ("email", "username") VALUES (:email, :username), (:email, :username), (:email, :username)`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

func TestFactories_ProgrammaticConditionItems(t *testing.T) {
	project := dbml.NewProject("test")
	users := dbml.NewTable("users")
	users.AddColumn(dbml.NewColumn("id", "bigint"))
	users.AddColumn(dbml.NewColumn("active", "boolean"))
	users.AddColumn(dbml.NewColumn("age", "int"))
	project.AddTable(users)

	instance, err := astql.NewFromDBML(project)
	if err != nil {
		t.Fatalf("Failed to create instance: %v", err)
	}

	// Simulate dynamic condition building
	conditions := instance.ConditionItems()
	conditions = append(conditions, instance.C(instance.F("id"), "=", instance.P("user_id")))
	conditions = append(conditions, instance.C(instance.F("active"), "=", instance.P("is_active")))
	conditions = append(conditions, instance.C(instance.F("age"), ">", instance.P("min_age")))

	result, err := astql.Select(instance.T("users")).
		Where(instance.And(conditions...)).
		Render(postgres.New())

	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT * FROM "users" WHERE ("id" = :user_id AND "active" = :is_active AND "age" > :min_age)`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

func TestFactories_ConditionItemsWithOr(t *testing.T) {
	project := dbml.NewProject("test")
	users := dbml.NewTable("users")
	users.AddColumn(dbml.NewColumn("status", "varchar"))
	users.AddColumn(dbml.NewColumn("role", "varchar"))
	project.AddTable(users)

	instance, err := astql.NewFromDBML(project)
	if err != nil {
		t.Fatalf("Failed to create instance: %v", err)
	}

	// Build OR conditions dynamically
	statusValues := []string{"active", "pending", "verified"}

	conditions := instance.ConditionItems()
	for _, status := range statusValues {
		conditions = append(conditions, instance.C(
			instance.F("status"),
			"=",
			instance.P(status),
		))
	}

	result, err := astql.Select(instance.T("users")).
		Where(instance.Or(conditions...)).
		Render(postgres.New())

	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := `SELECT * FROM "users" WHERE ("status" = :active OR "status" = :pending OR "status" = :verified)`
	if result.SQL != expected {
		t.Errorf("Expected SQL:\n%s\nGot:\n%s", expected, result.SQL)
	}
}

func TestFactories_ProgrammaticParams(t *testing.T) {
	project := dbml.NewProject("test")
	users := dbml.NewTable("users")
	users.AddColumn(dbml.NewColumn("id", "bigint"))
	users.AddColumn(dbml.NewColumn("username", "varchar"))
	users.AddColumn(dbml.NewColumn("email", "varchar"))
	users.AddColumn(dbml.NewColumn("age", "int"))
	project.AddTable(users)

	instance, err := astql.NewFromDBML(project)
	if err != nil {
		t.Fatalf("Failed to create instance: %v", err)
	}

	// Simulate collecting params dynamically for batch condition building
	paramNames := []string{"min_age", "max_age", "status"}

	params := instance.Params()
	for _, name := range paramNames {
		params = append(params, instance.P(name))
	}

	// Verify we collected the expected params
	if len(params) != 3 {
		t.Errorf("Expected 3 params, got %d", len(params))
	}

	expectedNames := []string{"min_age", "max_age", "status"}
	for i, p := range params {
		if p.GetName() != expectedNames[i] {
			t.Errorf("Expected param name %s, got %s", expectedNames[i], p.GetName())
		}
	}
}

// Test Operation accessor methods.
func TestOperationAccessors(t *testing.T) {
	instance := createTestInstance(t)

	tests := []struct {
		name     string
		accessor func() astql.Operation
		expected astql.Operation
	}{
		{"OpSelect", instance.OpSelect, astql.OpSelect},
		{"OpInsert", instance.OpInsert, astql.OpInsert},
		{"OpUpdate", instance.OpUpdate, astql.OpUpdate},
		{"OpDelete", instance.OpDelete, astql.OpDelete},
		{"OpCount", instance.OpCount, astql.OpCount},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.accessor()
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// Test Direction accessor methods.
func TestDirectionAccessors(t *testing.T) {
	instance := createTestInstance(t)

	tests := []struct {
		name     string
		accessor func() astql.Direction
		expected astql.Direction
	}{
		{"ASC", instance.ASC, astql.ASC},
		{"DESC", instance.DESC, astql.DESC},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.accessor()
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// Test Operator accessor methods.
func TestOperatorAccessors(t *testing.T) {
	instance := createTestInstance(t)

	tests := []struct {
		name     string
		accessor func() astql.Operator
		expected astql.Operator
	}{
		{"EQ", instance.EQ, astql.EQ},
		{"NE", instance.NE, astql.NE},
		{"GT", instance.GT, astql.GT},
		{"GE", instance.GE, astql.GE},
		{"LT", instance.LT, astql.LT},
		{"LE", instance.LE, astql.LE},
		{"IN", instance.IN, astql.IN},
		{"NotIn", instance.NotIn, astql.NotIn},
		{"LIKE", instance.LIKE, astql.LIKE},
		{"NotLike", instance.NotLike, astql.NotLike},
		{"ILIKE", instance.ILIKE, astql.ILIKE},
		{"NotILike", instance.NotILike, astql.NotILike},
		{"IsNull", instance.IsNull, astql.IsNull},
		{"IsNotNull", instance.IsNotNull, astql.IsNotNull},
		{"EXISTS", instance.EXISTS, astql.EXISTS},
		{"NotExists", instance.NotExists, astql.NotExists},
		{"RegexMatch", instance.RegexMatch, astql.RegexMatch},
		{"RegexIMatch", instance.RegexIMatch, astql.RegexIMatch},
		{"NotRegexMatch", instance.NotRegexMatch, astql.NotRegexMatch},
		{"NotRegexIMatch", instance.NotRegexIMatch, astql.NotRegexIMatch},
		{"ArrayContains", instance.ArrayContains, astql.ArrayContains},
		{"ArrayContainedBy", instance.ArrayContainedBy, astql.ArrayContainedBy},
		{"ArrayOverlap", instance.ArrayOverlap, astql.ArrayOverlap},
		{"VectorL2Distance", instance.VectorL2Distance, astql.VectorL2Distance},
		{"VectorInnerProduct", instance.VectorInnerProduct, astql.VectorInnerProduct},
		{"VectorCosineDistance", instance.VectorCosineDistance, astql.VectorCosineDistance},
		{"VectorL1Distance", instance.VectorL1Distance, astql.VectorL1Distance},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.accessor()
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}
