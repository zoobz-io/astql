// Package benchmarks provides performance benchmarks for astql.
package benchmarks

import (
	"testing"

	"github.com/zoobz-io/astql"
	"github.com/zoobz-io/astql/postgres"
	"github.com/zoobz-io/dbml"
)

func createBenchmarkInstance(b *testing.B) *astql.ASTQL {
	b.Helper()

	project := dbml.NewProject("bench")

	users := dbml.NewTable("users")
	users.AddColumn(dbml.NewColumn("id", "bigint"))
	users.AddColumn(dbml.NewColumn("username", "varchar"))
	users.AddColumn(dbml.NewColumn("email", "varchar"))
	users.AddColumn(dbml.NewColumn("age", "int"))
	users.AddColumn(dbml.NewColumn("active", "boolean"))
	users.AddColumn(dbml.NewColumn("created_at", "timestamp"))
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
		b.Fatalf("Failed to create instance: %v", err)
	}
	return instance
}

// BenchmarkSimpleSelect measures simple SELECT query rendering.
func BenchmarkSimpleSelect(b *testing.B) {
	instance := createBenchmarkInstance(b)
	table := instance.T("users")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := astql.Select(table).Render(postgres.New())
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSelectWithFields measures SELECT with explicit fields.
func BenchmarkSelectWithFields(b *testing.B) {
	instance := createBenchmarkInstance(b)
	table := instance.T("users")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := astql.Select(table).Fields(
			instance.F("id"),
			instance.F("username"),
			instance.F("email"),
			instance.F("age"),
		).Render(postgres.New())
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSelectWithWhere measures SELECT with WHERE clause.
func BenchmarkSelectWithWhere(b *testing.B) {
	instance := createBenchmarkInstance(b)
	table := instance.T("users")
	cond := instance.C(instance.F("active"), "=", instance.P("is_active"))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := astql.Select(table).Where(cond).Render(postgres.New())
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSelectWithMultipleConditions measures SELECT with complex WHERE.
func BenchmarkSelectWithMultipleConditions(b *testing.B) {
	instance := createBenchmarkInstance(b)
	table := instance.T("users")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := astql.Select(table).
			Where(instance.And(
				instance.C(instance.F("active"), "=", instance.P("is_active")),
				instance.Or(
					instance.C(instance.F("age"), ">", instance.P("min_age")),
					instance.C(instance.F("username"), "LIKE", instance.P("pattern")),
				),
			)).
			Render(postgres.New())
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSelectWithJoin measures SELECT with JOIN.
func BenchmarkSelectWithJoin(b *testing.B) {
	instance := createBenchmarkInstance(b)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := astql.Select(instance.T("users", "u")).
			Fields(instance.WithTable(instance.F("username"), "u")).
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
			b.Fatal(err)
		}
	}
}

// BenchmarkSelectWithOrderByLimit measures SELECT with ORDER BY and LIMIT.
func BenchmarkSelectWithOrderByLimit(b *testing.B) {
	instance := createBenchmarkInstance(b)
	table := instance.T("users")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := astql.Select(table).
			OrderBy(instance.F("created_at"), astql.DESC).
			Limit(10).
			Offset(20).
			Render(postgres.New())
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSelectWithAggregates measures SELECT with aggregate functions.
func BenchmarkSelectWithAggregates(b *testing.B) {
	instance := createBenchmarkInstance(b)
	table := instance.T("orders")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := astql.Select(table).
			Fields(instance.F("user_id")).
			SelectExpr(astql.As(astql.Sum(instance.F("total")), "total_spent")).
			SelectExpr(astql.As(astql.CountStar(), "order_count")).
			GroupBy(instance.F("user_id")).
			Render(postgres.New())
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkInsert measures INSERT query rendering.
func BenchmarkInsert(b *testing.B) {
	instance := createBenchmarkInstance(b)
	table := instance.T("users")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		vm := instance.ValueMap()
		vm[instance.F("username")] = instance.P("username")
		vm[instance.F("email")] = instance.P("email")
		vm[instance.F("age")] = instance.P("age")

		_, err := astql.Insert(table).Values(vm).Render(postgres.New())
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkInsertWithReturning measures INSERT with RETURNING.
func BenchmarkInsertWithReturning(b *testing.B) {
	instance := createBenchmarkInstance(b)
	table := instance.T("users")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		vm := instance.ValueMap()
		vm[instance.F("username")] = instance.P("username")
		vm[instance.F("email")] = instance.P("email")

		_, err := astql.Insert(table).
			Values(vm).
			Returning(instance.F("id")).
			Render(postgres.New())
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkUpdate measures UPDATE query rendering.
func BenchmarkUpdate(b *testing.B) {
	instance := createBenchmarkInstance(b)
	table := instance.T("users")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := astql.Update(table).
			Set(instance.F("username"), instance.P("new_username")).
			Set(instance.F("email"), instance.P("new_email")).
			Where(instance.C(instance.F("id"), "=", instance.P("user_id"))).
			Render(postgres.New())
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkDelete measures DELETE query rendering.
func BenchmarkDelete(b *testing.B) {
	instance := createBenchmarkInstance(b)
	table := instance.T("users")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := astql.Delete(table).
			Where(instance.C(instance.F("id"), "=", instance.P("user_id"))).
			Render(postgres.New())
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCount measures COUNT query rendering.
func BenchmarkCount(b *testing.B) {
	instance := createBenchmarkInstance(b)
	table := instance.T("users")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := astql.Count(table).
			Where(instance.C(instance.F("active"), "=", instance.P("is_active"))).
			Render(postgres.New())
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCaseExpression measures CASE expression rendering.
func BenchmarkCaseExpression(b *testing.B) {
	instance := createBenchmarkInstance(b)
	table := instance.T("users")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		caseExpr := astql.Case().
			When(instance.C(instance.F("age"), "<", instance.P("young")), instance.P("young_label")).
			When(instance.C(instance.F("age"), "<", instance.P("mid")), instance.P("mid_label")).
			Else(instance.P("old_label")).
			As("age_group").
			Build()

		_, err := astql.Select(table).
			Fields(instance.F("username")).
			SelectExpr(caseExpr).
			Render(postgres.New())
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkWindowFunction measures window function rendering.
func BenchmarkWindowFunction(b *testing.B) {
	instance := createBenchmarkInstance(b)
	table := instance.T("orders")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		winExpr := astql.RowNumber().
			PartitionBy(instance.F("user_id")).
			OrderBy(instance.F("total"), astql.DESC).
			As("rank")

		_, err := astql.Select(table).
			Fields(instance.F("id"), instance.F("total")).
			SelectExpr(winExpr).
			Render(postgres.New())
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkBetween measures BETWEEN condition rendering.
func BenchmarkBetween(b *testing.B) {
	instance := createBenchmarkInstance(b)
	table := instance.T("users")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := astql.Select(table).
			Where(astql.Between(instance.F("age"), instance.P("min"), instance.P("max"))).
			Render(postgres.New())
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSubquery measures subquery rendering.
func BenchmarkSubquery(b *testing.B) {
	instance := createBenchmarkInstance(b)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		subquery := astql.Sub(
			astql.Select(instance.T("posts")).
				Fields(instance.F("user_id")).
				Where(instance.C(instance.F("published"), "=", instance.P("is_published"))),
		)

		_, err := astql.Select(instance.T("users")).
			Where(astql.CSub(instance.F("id"), astql.IN, subquery)).
			Render(postgres.New())
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkComplexQuery measures a complex real-world query.
func BenchmarkComplexQuery(b *testing.B) {
	instance := createBenchmarkInstance(b)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := astql.Select(instance.T("users", "u")).
			Fields(
				instance.WithTable(instance.F("id"), "u"),
				instance.WithTable(instance.F("username"), "u"),
			).
			SelectExpr(astql.As(astql.Sum(instance.WithTable(instance.F("total"), "o")), "total_spent")).
			SelectExpr(astql.As(astql.CountStar(), "order_count")).
			InnerJoin(
				instance.T("orders", "o"),
				astql.CF(
					instance.WithTable(instance.F("id"), "u"),
					"=",
					instance.WithTable(instance.F("user_id"), "o"),
				),
			).
			Where(instance.And(
				instance.C(instance.WithTable(instance.F("active"), "u"), "=", instance.P("is_active")),
				instance.C(instance.WithTable(instance.F("status"), "o"), "=", instance.P("status")),
			)).
			GroupBy(
				instance.WithTable(instance.F("id"), "u"),
				instance.WithTable(instance.F("username"), "u"),
			).
			HavingAgg(astql.HavingSum(instance.WithTable(instance.F("total"), "o"), astql.GT, instance.P("min_total"))).
			OrderBy(instance.WithTable(instance.F("username"), "u"), astql.ASC).
			Limit(10).
			Render(postgres.New())
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkDistinctOn measures DISTINCT ON rendering.
func BenchmarkDistinctOn(b *testing.B) {
	instance := createBenchmarkInstance(b)
	table := instance.T("posts")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := astql.Select(table).
			DistinctOn(instance.F("user_id")).
			Fields(instance.F("user_id"), instance.F("title")).
			OrderBy(instance.F("user_id"), astql.ASC).
			OrderBy(instance.F("views"), astql.DESC).
			Render(postgres.New())
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkForUpdate measures FOR UPDATE locking rendering.
func BenchmarkForUpdate(b *testing.B) {
	instance := createBenchmarkInstance(b)
	table := instance.T("users")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := astql.Select(table).
			Where(instance.C(instance.F("id"), "=", instance.P("user_id"))).
			ForUpdate().
			Render(postgres.New())
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkNullsOrdering measures NULLS FIRST/LAST rendering.
func BenchmarkNullsOrdering(b *testing.B) {
	instance := createBenchmarkInstance(b)
	table := instance.T("users")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := astql.Select(table).
			OrderByNulls(instance.F("age"), astql.ASC, astql.NullsFirst).
			Render(postgres.New())
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkTypeCast measures type casting rendering.
func BenchmarkTypeCast(b *testing.B) {
	instance := createBenchmarkInstance(b)
	table := instance.T("users")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := astql.Select(table).
			SelectExpr(astql.As(astql.Cast(instance.F("age"), astql.CastText), "age_text")).
			Render(postgres.New())
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCoalesce measures COALESCE rendering.
func BenchmarkCoalesce(b *testing.B) {
	instance := createBenchmarkInstance(b)
	table := instance.T("users")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := astql.Select(table).
			SelectExpr(astql.As(
				astql.Coalesce(instance.P("val1"), instance.P("val2"), instance.P("val3")),
				"result",
			)).
			Render(postgres.New())
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark table for component creation (not rendering).

// BenchmarkFieldCreation measures field creation overhead.
func BenchmarkFieldCreation(b *testing.B) {
	instance := createBenchmarkInstance(b)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = instance.F("username")
	}
}

// BenchmarkTableCreation measures table creation overhead.
func BenchmarkTableCreation(b *testing.B) {
	instance := createBenchmarkInstance(b)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = instance.T("users")
	}
}

// BenchmarkParamCreation measures parameter creation overhead.
func BenchmarkParamCreation(b *testing.B) {
	instance := createBenchmarkInstance(b)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = instance.P("user_id")
	}
}

// BenchmarkConditionCreation measures condition creation overhead.
func BenchmarkConditionCreation(b *testing.B) {
	instance := createBenchmarkInstance(b)
	field := instance.F("active")
	param := instance.P("is_active")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = instance.C(field, "=", param)
	}
}
