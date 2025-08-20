# Contributing to AutoTime

Thank you for your interest in contributing to AutoTime! We welcome contributions from the community and are grateful for any help you can provide.

## Table of Contents

- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Contributing Guidelines](#contributing-guidelines)
- [Code Style](#code-style)
- [Adding Connectors](#adding-connectors)
- [Testing](#testing)
- [Documentation](#documentation)
- [Pull Request Process](#pull-request-process)
- [Bug Reports](#bug-reports)
- [Feature Requests](#feature-requests)

## Getting Started

### Development Setup

1. **Fork the repository** on GitHub
2. **Clone your fork** locally:
   ```bash
   git clone https://github.com/yourusername/autotime.git
   cd autotime
   ```

3. **Set up development environment**:
   ```bash
   make setup
   ```

4. **Build and test**:
   ```bash
   make build
   make test
   ```

5. **Run the application**:
   ```bash
   make run
   ```

### Prerequisites

- Go 1.21 or higher
- Git
- Make (optional but recommended)
- golangci-lint (installed by `make setup`)

## Contributing Guidelines

### Types of Contributions

We welcome several types of contributions:

1. **Bug fixes** - Fix issues in existing functionality
- **New connectors** - Add support for new services
3. **Feature enhancements** - Improve existing features
4. **Documentation** - Improve or expand documentation
5. **Tests** - Add test coverage
6. **UI/UX improvements** - Enhance the terminal interface

### Before You Start

- **Check existing issues** to see if your idea is already being worked on
- **Create an issue** to discuss major changes before implementing
- **Keep changes focused** - one feature or fix per PR
- **Follow the existing code style** and patterns

## Code Style

### Go Guidelines

We follow standard Go conventions:

- Use `gofmt` for formatting (run `make fmt`)
- Follow effective Go practices
- Use meaningful variable and function names
- Add comments for exported functions and complex logic
- Keep functions small and focused

### Project Structure

```
autotime/
├── cmd/                    # CLI commands
├── internal/
│   ├── config/            # Configuration management
│   ├── connectors/        # Service connectors
│   ├── timeline/          # Timeline data structures
│   └── tui/              # Terminal UI components
├── docs/                  # Documentation
├── examples/              # Example configurations
└── tests/                 # Integration tests
```

### Naming Conventions

- **Packages**: lowercase, single word when possible
- **Files**: lowercase with underscores if needed
- **Functions**: camelCase, exported functions start with uppercase
- **Variables**: camelCase
- **Constants**: UPPER_CASE for package-level constants

## Adding Connectors

Adding a new connector is one of the most valuable contributions. Here's how:

### 1. Create the Connector

Create a new file in `internal/connectors/yourservice.go`:

```go
package connectors

import (
    "context"
    "time"
    "github.com/autotime/autotime/internal/timeline"
)

type YourServiceConnector struct {
    *BaseConnector
}

func NewYourServiceConnector() *YourServiceConnector {
    return &YourServiceConnector{
        BaseConnector: NewBaseConnector(
            "yourservice",
            "Description of what this connector does",
        ),
    }
}

func (c *YourServiceConnector) GetRequiredConfig() []ConfigField {
    return []ConfigField{
        {
            Key:         "api_key",
            Type:        "secret",
            Required:    true,
            Description: "API key for YourService",
        },
        // Add more fields as needed
    }
}

func (c *YourServiceConnector) ValidateConfig(config map[string]interface{}) error {
    // Validate configuration
    return nil
}

func (c *YourServiceConnector) TestConnection(ctx context.Context) error {
    // Test if the connector can connect to the service
    return nil
}

func (c *YourServiceConnector) GetActivities(ctx context.Context, date time.Time) ([]timeline.Activity, error) {
    // Fetch activities for the given date
    var activities []timeline.Activity
    // ... implementation
    return activities, nil
}
```

### 2. Register the Connector

Add your connector to `cmd/root.go` in the `initializeSystem` function:

```go
registry.Register(connectors.NewYourServiceConnector())
```

### 3. Add Default Configuration

Update `internal/config/config.go` to include default configuration:

```go
"yourservice": {
    Enabled: false,
    Config: map[string]interface{}{
        "api_key": "",
        // ... default values
    },
    RefreshInterval: "15m",
},
```

### 4. Add Documentation

- Update `README.md` with connector information
- Add configuration examples to `config.example.yaml`
- Document any special setup requirements

### 5. Add Tests

Create tests for your connector in `internal/connectors/yourservice_test.go`

## Testing

### Running Tests

```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# Run specific package tests
go test ./internal/connectors/...
```

### Writing Tests

- **Unit tests** for individual functions
- **Integration tests** for connector functionality
- **Mock external services** where appropriate
- **Test error conditions** and edge cases

### Test Structure

```go
func TestYourServiceConnector_GetActivities(t *testing.T) {
    connector := NewYourServiceConnector()

    // Set up test configuration
    config := map[string]interface{}{
        "api_key": "test-key",
    }

    err := connector.Configure(config)
    assert.NoError(t, err)

    // Test the functionality
    activities, err := connector.GetActivities(context.Background(), time.Now())

    assert.NoError(t, err)
    assert.NotNil(t, activities)
}
```

## Documentation

### What to Document

- **New features** - How to use them
- **Configuration changes** - New settings and options
- **Breaking changes** - Migration guides
- **API changes** - Updated interfaces

### Documentation Types

- **README.md** - Main project documentation
- **INSTALL.md** - Installation and setup
- **Code comments** - Inline documentation
- **Examples** - Working configuration examples

## Pull Request Process

### Before Submitting

1. **Run the full test suite**:
   ```bash
   make check
   ```

2. **Test your changes manually**:
   ```bash
   make run
   ```

3. **Update documentation** if needed

4. **Add tests** for new functionality

### PR Guidelines

1. **Create a clear title** describing the change
2. **Write a detailed description**:
   - What problem does this solve?
   - How does it solve it?
   - What are the breaking changes (if any)?
   - How to test the changes?

3. **Link related issues** using keywords like "fixes #123"
4. **Keep PRs focused** - avoid unrelated changes
5. **Respond to feedback** promptly and respectfully

### PR Template

```markdown
## Description
Brief description of changes

## Type of Change
- [ ] Bug fix
- [ ] New feature
- [ ] Breaking change
- [ ] Documentation update

## Testing
- [ ] Unit tests pass
- [ ] Integration tests pass
- [ ] Manual testing completed

## Checklist
- [ ] Code follows style guidelines
- [ ] Self-review completed
- [ ] Documentation updated
- [ ] Tests added for new functionality
```

## Bug Reports

### Before Reporting

1. **Search existing issues** to avoid duplicates
2. **Try the latest version** to see if it's already fixed
3. **Check the documentation** for known limitations

### Bug Report Template

```markdown
**Bug Description**
Clear description of what the bug is

**To Reproduce**
1. Steps to reproduce the behavior
2. Expected behavior
3. Actual behavior

**Environment**
- OS: [e.g., macOS 14.0]
- Go version: [e.g., 1.21]
- AutoTime version: [e.g., 0.1.0]

**Additional Context**
- Configuration file (redacted)
- Error messages
- Screenshots if applicable
```

## Feature Requests

### Before Requesting

1. **Check existing issues** for similar requests
2. **Consider the scope** - is this suitable for the core project?
3. **Think about implementation** - provide details where possible

### Feature Request Template

```markdown
**Feature Description**
Clear description of the desired feature

**Use Case**
Why would this feature be useful?
What problem does it solve?

**Proposed Solution**
How do you envision this working?

**Alternatives Considered**
What other approaches might work?

**Additional Context**
Any other relevant information
```

## Code Review Process

### For Contributors

- **Be responsive** to review feedback
- **Explain your reasoning** when disagreeing with feedback
- **Ask questions** if feedback is unclear
- **Update your PR** based on feedback

### For Reviewers

- **Be constructive** and specific in feedback
- **Explain the reasoning** behind suggestions
- **Appreciate the effort** contributors put in
- **Focus on code quality** and project consistency

## Getting Help

### Communication Channels

- **GitHub Issues** - For bugs and feature requests
- **GitHub Discussions** - For questions and general discussion
- **Code Comments** - For implementation questions

### What We Look For

- **Clean, readable code** that follows Go conventions
- **Comprehensive tests** for new functionality
- **Clear documentation** for user-facing changes
- **Respect for existing patterns** and architecture

## Recognition

Contributors are recognized in several ways:

- **Contributors list** in the README
- **Release notes** mention significant contributions
- **Commit history** preserves authorship

## License

By contributing to AutoTime, you agree that your contributions will be licensed under the same license as the project (MIT License).

---

Thank you for contributing to AutoTime! Your help makes this project better for everyone. 🙏
