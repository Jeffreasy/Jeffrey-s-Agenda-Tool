# Contributing to Agenda Automator

Thank you for your interest in contributing to the Agenda Automator backend! This document provides guidelines and information for contributors.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Workflow](#development-workflow)
- [Coding Standards](#coding-standards)
- [Testing](#testing)
- [Submitting Changes](#submitting-changes)
- [Reporting Issues](#reporting-issues)

## Code of Conduct

This project follows a code of conduct to ensure a welcoming environment for all contributors. Please be respectful and constructive in all interactions.

## Getting Started

1. **Set up your development environment** following the [Setup Guide](SETUP.md)
2. **Familiarize yourself** with the [Architecture](ARCHITECTURE.md) and [API Reference](API_REFERENCE.md)
3. **Choose an issue** from the issue tracker or propose a new feature

## Development Workflow

### 1. Fork and Clone

```bash
# Fork the repository on GitHub
# Clone your fork
git clone https://github.com/YOUR-USERNAME/agenda-automator-backend.git
cd agenda-automator-backend
```

### 2. Create a Feature Branch

```bash
# Create and switch to a new branch
git checkout -b feature/your-feature-name
# or for bug fixes
git checkout -b fix/issue-number-description
```

### 3. Make Changes

- Write clean, well-documented code
- Follow the coding standards below
- Test your changes thoroughly
- Update documentation if needed

### 4. Test Your Changes

```bash
# Run tests
go test ./...

# Run with race detection
go test -race ./...

# Test with coverage
go test -cover ./...
```

### 5. Commit Your Changes

```bash
# Stage your changes
git add .

# Commit with a descriptive message
git commit -m "feat: add new feature description

- What was changed
- Why it was changed
- Any breaking changes"
```

### 6. Push and Create Pull Request

```bash
# Push to your fork
git push origin feature/your-feature-name

# Create a Pull Request on GitHub
```

## Coding Standards

### Go Code

- Follow standard Go formatting: `go fmt`
- Use `go vet` to check for common errors
- Follow the [Effective Go](https://golang.org/doc/effective_go.html) guidelines
- Use meaningful variable and function names
- Add comments for exported functions and types
- Keep functions small and focused

### Code Structure

- Place code in appropriate packages under `/internal`
- Use the Repository pattern for data access
- Separate concerns (API, business logic, data storage)
- Handle errors properly with context

### Commit Messages

Follow conventional commit format:

```
type(scope): description

[optional body]

[optional footer]
```

Types:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes
- `refactor`: Code refactoring
- `test`: Test additions/changes
- `chore`: Maintenance tasks

Examples:
```
feat(api): add user registration endpoint
fix(worker): handle token refresh errors
docs(readme): update setup instructions
```

### Branch Naming

- Features: `feature/description`
- Bug fixes: `fix/issue-number-description`
- Hotfixes: `hotfix/description`

## Testing

### Unit Tests

- Write tests for all new functions
- Place test files alongside source files (`*_test.go`)
- Aim for good test coverage (>80%)
- Use table-driven tests for multiple test cases

Example:
```go
func TestValidateEmail(t *testing.T) {
    tests := []struct {
        input    string
        expected bool
    }{
        {"user@example.com", true},
        {"invalid-email", false},
        {"", false},
    }

    for _, test := range tests {
        result := validateEmail(test.input)
        if result != test.expected {
            t.Errorf("validateEmail(%q) = %v; want %v", test.input, result, test.expected)
        }
    }
}
```

### Integration Tests

- Test API endpoints with real database
- Use build tags for integration tests
- Ensure tests are isolated and repeatable

### Running Tests

```bash
# Run all tests
go test ./...

# Run with verbose output
go test -v ./...

# Run specific package
go test ./internal/api

# Run with coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## Submitting Changes

### Pull Request Process

1. **Ensure your branch is up to date**
   ```bash
   git fetch origin
   git rebase origin/main
   ```

2. **Run full test suite**
   ```bash
   go test ./...
   go vet ./...
   go fmt ./...
   ```

3. **Update documentation** if needed
   - Update README.md for new features
   - Update API documentation
   - Add migration notes for database changes

4. **Create Pull Request**
   - Provide clear title and description
   - Reference related issues
   - Include screenshots for UI changes
   - List any breaking changes

5. **Code Review**
   - Address reviewer feedback
   - Make requested changes
   - Ensure CI checks pass

### Pull Request Template

Please use this template for pull requests:

```markdown
## Description
Brief description of the changes.

## Type of Change
- [ ] Bug fix
- [ ] New feature
- [ ] Breaking change
- [ ] Documentation update

## Testing
- [ ] Unit tests added/updated
- [ ] Integration tests added/updated
- [ ] Manual testing completed

## Checklist
- [ ] Code follows project standards
- [ ] Tests pass
- [ ] Documentation updated
- [ ] No breaking changes
```

## Reporting Issues

### Bug Reports

When reporting bugs, please include:

1. **Clear title** describing the issue
2. **Steps to reproduce**
3. **Expected behavior**
4. **Actual behavior**
5. **Environment details**
   - Go version
   - OS and version
   - Database version
6. **Logs** or error messages
7. **Screenshots** if applicable

### Feature Requests

For new features, please:

1. **Check existing issues** to avoid duplicates
2. **Describe the problem** the feature solves
3. **Explain the solution** you'd like
4. **Consider alternatives** you've thought about
5. **Provide context** about why this feature is needed

## Database Changes

### Migrations

- Create migration files in `db/migrations/`
- Use descriptive names: `000002_add_user_preferences.up.sql`
- Include rollback migrations: `000002_add_user_preferences.down.sql`
- Test migrations on a copy of production data

### Schema Changes

- Document breaking changes
- Update models in `/internal/domain`
- Update store methods in `/internal/store`
- Update API handlers if needed

## Security

- Never commit sensitive data (passwords, API keys)
- Use environment variables for configuration
- Follow OWASP guidelines for API security
- Report security issues privately to maintainers

## Getting Help

- Check the [documentation](README.md) first
- Search existing issues and discussions
- Ask questions in GitHub Discussions
- Join our community chat (if available)

## Recognition

Contributors will be recognized in:
- CHANGELOG.md for significant contributions
- GitHub's contributor insights
- Release notes

Thank you for contributing to Agenda Automator! 🎉