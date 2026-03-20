package astql

import (
	"github.com/zoobz-io/astql/internal/render"
	"github.com/zoobz-io/astql/internal/types"
)

// Renderer defines the interface for SQL dialect-specific rendering.
// Implementations convert an AST to dialect-specific SQL with named parameters.
type Renderer interface {
	// Render converts an AST to a QueryResult with dialect-specific SQL.
	Render(ast *types.AST) (*types.QueryResult, error)

	// RenderCompound converts a CompoundQuery (UNION, INTERSECT, EXCEPT) to SQL.
	RenderCompound(query *types.CompoundQuery) (*types.QueryResult, error)

	// Capabilities returns the SQL features supported by this dialect.
	Capabilities() render.Capabilities
}
