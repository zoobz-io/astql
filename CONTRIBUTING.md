# Contributing to ASTQL

Thank you for your interest in contributing to ASTQL! This guide will help you get started.

## Code of Conduct

By participating in this project, you agree to maintain a respectful and inclusive environment for all contributors.

## Getting Started

1. Fork the repository
2. Clone your fork: `git clone https://github.com/yourusername/astql.git`
3. Create a feature branch: `git checkout -b feature/your-feature-name`
4. Make your changes
5. Run tests: `make test`
6. Run linter: `make lint`
7. Commit your changes with a descriptive message
8. Push to your fork: `git push origin feature/your-feature-name`
9. Create a Pull Request

## Development Guidelines

### Code Style

- Follow standard Go conventions
- Run `gofmt` before committing
- Pass all linter checks: `make lint`
- Add godoc comments for all exported functions and types
- Keep functions small and focused
- Add periods to comment lines

### Testing

- Write tests for new functionality
- Ensure all tests pass: `make test`
- Run race detector: `make test-race`
- Include benchmarks for performance-critical code
- Aim for >70% test coverage
- Test both PostgreSQL and SQLite providers

### Documentation

- Update README for significant features
- Add godoc comments for all exported APIs
- Include examples in documentation
- Update schema examples if adding new features
- Keep comments clear and concise

## Types of Contributions

### Bug Reports

- Use GitHub Issues
- Include minimal reproduction code
- Describe expected vs actual behavior
- Include Go version and OS
- Specify which provider (PostgreSQL/SQLite)

### Feature Requests

- Open an issue first to discuss
- Describe the use case
- Consider backward compatibility
- Propose API design if applicable

### Pull Requests

- Reference related issues
- Keep changes focused and atomic
- Add tests for new features
- Update documentation
- Ensure CI passes

## Development Workflow

### Quick Start

For a fresh machine, provision everything (Go toolchain, C compiler, lint/security
tools, and dependencies) in one step:

```bash
./tools/setup-dev.sh
```

Then open a new shell so `go` and the tools are on your `PATH`.

To do it by hand instead:

```bash
# Install dependencies
go mod download

# Install development tools
make install-tools

# Run tests
make test

# Run linter
make lint

# Run all checks
make check

# Generate coverage report
make coverage
```

### Provider-Specific Development

When working on provider-specific features:

1. Implement for both PostgreSQL and SQLite when applicable
2. If a feature is database-specific, document the limitation
3. Add provider-specific tests
4. Ensure consistent API across providers

### Schema Development

When modifying schema support:

1. Update both provider implementations
2. Add schema validation tests
3. Update schema documentation
4. Consider backward compatibility

## Code Review Process

1. All submissions require review
2. CI must pass
3. Maintainers will provide feedback
4. Address review comments
5. Squash commits if requested

## Release Process

1. Maintainers handle releases
2. Semantic versioning is used
3. Changelog is maintained
4. Tagged releases trigger automation

## Questions?

Feel free to open an issue for questions or join discussions in existing issues.

Thank you for contributing to ASTQL!