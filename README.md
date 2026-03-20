# astql

[![CI](https://github.com/zoobz-io/astql/actions/workflows/ci.yml/badge.svg)](https://github.com/zoobz-io/astql/actions/workflows/ci.yml)
[![Coverage](https://codecov.io/gh/zoobz-io/astql/branch/main/graph/badge.svg)](https://codecov.io/gh/zoobz-io/astql)
[![Go Report Card](https://goreportcard.com/badge/github.com/zoobz-io/astql)](https://goreportcard.com/report/github.com/zoobz-io/astql)
[![CodeQL](https://github.com/zoobz-io/astql/actions/workflows/codeql.yml/badge.svg)](https://github.com/zoobz-io/astql/actions/workflows/codeql.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/zoobz-io/astql.svg)](https://pkg.go.dev/github.com/zoobz-io/astql)
[![License](https://img.shields.io/github/license/zoobz-io/astql)](LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/zoobz-io/astql)](go.mod)
[![Release](https://img.shields.io/github/v/release/zoobz-io/astql)](https://github.com/zoobz-io/astql/releases)

Type-safe SQL query builder with DBML schema validation.

Build queries as an AST, validate against your schema, render to parameterized SQL.

## Injection-safe SQL Expressions, Any Dialect

```go
instance.T("users")      // ✓ exists in schema
instance.T("uusers")     // panic: table "uusers" not found

instance.F("email")      // ✓ exists in schema
instance.F("emial")      // panic: field "emial" not found

query := astql.Select(instance.T("users")).
    Fields(instance.F("username"), instance.F("email")).
    Where(instance.C(instance.F("active"), astql.EQ, instance.P("is_active")))

result, _ := query.Render(postgres.New())
// SELECT "username", "email" FROM "users" WHERE "active" = :is_active
```

Typos become compile-time failures, not runtime surprises. Values are parameterized. Identifiers are quoted. The schema is the source of truth.

Same query, different databases:

```go
import (
    "github.com/zoobz-io/astql/postgres"
    "github.com/zoobz-io/astql/sqlite"
    "github.com/zoobz-io/astql/mariadb"
    "github.com/zoobz-io/astql/mssql"
)

result, _ := query.Render(postgres.New())  // "username", LIMIT 10
result, _ := query.Render(sqlite.New())    // "username", LIMIT 10
result, _ := query.Render(mariadb.New())   // `username`, LIMIT 10
result, _ := query.Render(mssql.New())     // [username], TOP 10
```

One AST. Four dialects. Each renderer handles identifier quoting, pagination syntax, vendor-specific operators.

## Install

```bash
go get github.com/zoobz-io/astql
go get github.com/zoobz-io/dbml
```

Requires Go 1.24+.

## Quick Start

```go
package main

import (
    "fmt"
    "github.com/zoobz-io/astql"
    "github.com/zoobz-io/astql/postgres"
    "github.com/zoobz-io/dbml"
)

func main() {
    // Define schema
    project := dbml.NewProject("myapp")
    users := dbml.NewTable("users")
    users.AddColumn(dbml.NewColumn("id", "bigint"))
    users.AddColumn(dbml.NewColumn("username", "varchar"))
    users.AddColumn(dbml.NewColumn("email", "varchar"))
    users.AddColumn(dbml.NewColumn("active", "boolean"))
    project.AddTable(users)

    // Create instance
    instance, err := astql.NewFromDBML(project)
    if err != nil {
        panic(err)
    }

    // Build and render
    result, err := astql.Select(instance.T("users")).
        Fields(instance.F("username"), instance.F("email")).
        Where(instance.C(instance.F("active"), astql.EQ, instance.P("is_active"))).
        OrderBy(instance.F("username"), astql.ASC).
        Limit(10).
        Render(postgres.New())

    if err != nil {
        panic(err)
    }

    fmt.Println(result.SQL)
    // SELECT "username", "email" FROM "users" WHERE "active" = :is_active ORDER BY "username" ASC LIMIT 10
    fmt.Println(result.RequiredParams)
    // [is_active]
}
```

## Capabilities

| Feature              | Description                                          | Docs                                                      |
| -------------------- | ---------------------------------------------------- | --------------------------------------------------------- |
| Schema Validation    | Tables and fields checked against DBML at build time | [Schema Validation](docs/3.guides/1.schema-validation.md) |
| Multi-Dialect        | PostgreSQL, SQLite, MariaDB, MSSQL from one AST      | [Architecture](docs/2.learn/3.architecture.md)            |
| Parameterized Values | Injection-resistant queries with named parameters    | [Conditions](docs/3.guides/2.conditions.md)               |
| Composable Queries   | Subqueries, JOINs, aggregates, window functions      | [Joins](docs/3.guides/3.joins.md)                         |
| CASE Expressions     | Conditional logic within queries                     | [API](docs/5.reference/1.api.md)                          |

## Why ASTQL?

- **Schema-validated** — `T("users")` and `F("email")` checked against DBML at build time
- **Injection-resistant** — parameterized values, quoted identifiers, no string concatenation
- **Multi-dialect** — one query, four databases
- **Composable** — subqueries, JOINs, aggregates, window functions, CASE expressions

## Schema-First Data Access

ASTQL enables a pattern: **define schema once in DBML, generate everything else**.

Your DBML becomes the single source of truth. Downstream tools consume the schema to build:

- **Type-safe repositories** — generated data access layers with [cereal](https://github.com/zoobz-io/cereal)
- **Query builders** — domain-specific methods that can't reference invalid columns
- **Multi-database applications** — same business logic, swappable storage backends

```go
// Schema defines what's valid
project := dbml.ParseFile("schema.dbml")
instance, _ := astql.NewFromDBML(project)

// Queries are structurally correct by construction
users := instance.T("users")
query := astql.Select(users).
    Fields(instance.F("id"), instance.F("email")).
    Where(instance.C(instance.F("active"), astql.EQ, instance.P("active")))

// Render to any supported database
sql, _ := query.Render(postgres.New())  // production
sql, _ := query.Render(sqlite.New())    // testing
```

The schema guards the boundary. Queries inside the boundary are safe by construction.

## Documentation

- [Overview](docs/1.overview.md) — what astql does and why

### Learn

- [Quickstart](docs/2.learn/1.quickstart.md) — get started in minutes
- [Concepts](docs/2.learn/2.concepts.md) — tables, fields, params, conditions, builders
- [Architecture](docs/2.learn/3.architecture.md) — AST structure, render pipeline, security layers

### Guides

- [Schema Validation](docs/3.guides/1.schema-validation.md) — DBML integration and validation
- [Conditions](docs/3.guides/2.conditions.md) — WHERE, AND/OR, subqueries, BETWEEN
- [Joins](docs/3.guides/3.joins.md) — INNER, LEFT, RIGHT, CROSS joins
- [Aggregates](docs/3.guides/4.aggregates.md) — GROUP BY, HAVING, window functions
- [Testing](docs/3.guides/5.testing.md) — testing patterns for query builders

### Cookbook

- [Pagination](docs/4.cookbook/1.pagination.md) — LIMIT/OFFSET and cursor patterns
- [Vector Search](docs/4.cookbook/2.vector-search.md) — pgvector similarity queries
- [Upserts](docs/4.cookbook/3.upserts.md) — ON CONFLICT patterns
- [ORM Foundation](docs/4.cookbook/4.orm-foundation.md) — building type-safe ORMs with cereal

### Reference

- [API](docs/5.reference/1.api.md) — complete function documentation
- [Operators](docs/5.reference/2.operators.md) — all comparison and special operators

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines. For security issues, see [SECURITY.md](SECURITY.md).

## License

MIT — see [LICENSE](LICENSE).
