# FleetTUI - Agent Guidelines

## Project Overview

FleetTUI is a terminal UI application for monitoring server fleets in real-time. Built with Go using the Charm stack (Bubble Tea, Lipgloss, Bubbles).

Repects KISS and Vim binding when adding new features

### Architecture

The project follows clean architecture principles with clear separation of concerns:

```
internal/
├── domain/              # Core business entities and logic
│   ├── node.go         # Node entity with availability checks
│   └── metrics.go      # Metrics types and configuration
├── ports/              # Interface definitions (primary/secondary adapters)
│   └── output/         # Output ports (SSH, config loader interfaces)
├── adapters/           # Implementation of ports
│   ├── input/          # TUI adapter (Bubble Tea)
│   └── output/         # SSH client, config loader implementations
├── service/            # Business logic layer
│   └── collector.go    # Metrics collection orchestration
```

### Key Components

- **Domain Layer**: Pure business logic, no external dependencies
- **Service Layer**: Orchestrates domain operations, uses ports
- **Adapters**: Implement external interactions (SSH, files, TUI)
- **Ports**: Define contracts between layers

## Development Guidelines

### Testing Requirements

**ALL new features in the following packages MUST have unit tests:**

1. **`internal/domain/`** - Domain logic tests
   - Test all public methods
   - Use table-driven tests
   - Test edge cases (nil, empty, boundary values)

2. **`internal/adapters/output/config/`** - Config loader tests
   - Test file I/O with temporary files
   - Test error handling (missing files, invalid YAML)
   - Test default value application

3. **`internal/service/`** - Service layer tests
   - Use mocks for external dependencies
   - Test concurrent operations
   - Test error propagation

### Testing Approach

- Use **table-driven tests** for all test cases
- Use **testify/mock** (via mockery) for mocking interfaces
- Place tests in same package as code. 
- Test both success and failure paths

### Mock Generation

Mocks are generated using mockery. Configuration is in `.mockery.yaml`.

```bash
# Generate/update all mocks
mockery --all
```

### Code Quality

**ALL code changes MUST be formatted before submission:**

```bash
gofmt -w . 
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run specific package tests
go test ./internal/domain/...
go test ./internal/adapters/output/config/...
go test ./internal/service/...
```

### Adding New Features

When adding features to domain, config, or service layers:

1. Write the implementation
2. Write comprehensive tests (table-driven)
3. Generate mocks if new interfaces are added
5. Run linters && formatters
6. Ensure all tests pass

### Project Commands

```bash
# Build
go build .
```

/!\ Never commit the code or push by yourself.
