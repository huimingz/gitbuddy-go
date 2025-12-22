# Contributing to GitBuddy-Go

Thank you for your interest in contributing to GitBuddy-Go! This document provides guidelines and instructions for contributing.

## Code of Conduct

Please be respectful and constructive in all interactions. We welcome contributors of all backgrounds and experience levels.

## How to Contribute

### Reporting Bugs

1. Check if the bug has already been reported in [Issues](https://github.com/huimingz/gitbuddy-go/issues)
2. If not, create a new issue with:
   - A clear, descriptive title
   - Steps to reproduce the bug
   - Expected vs actual behavior
   - Your environment (OS, Go version, GitBuddy version)
   - Any relevant logs or error messages

### Suggesting Features

1. Check existing issues for similar suggestions
2. Create a new issue with the `enhancement` label
3. Describe the feature and its use case
4. Explain why this would be valuable

### Submitting Code

#### Setup Development Environment

1. Fork the repository
2. Clone your fork:
   ```bash
   git clone https://github.com/YOUR_USERNAME/gitbuddy-go.git
   cd gitbuddy-go
   ```

3. Add the upstream remote:
   ```bash
   git remote add upstream https://github.com/huimingz/gitbuddy-go.git
   ```

4. Install dependencies:
   ```bash
   make deps
   ```

5. Build and test:
   ```bash
   make build
   make test
   ```

#### Making Changes

1. Create a new branch:
   ```bash
   git checkout -b feature/your-feature-name
   # or
   git checkout -b fix/your-bug-fix
   ```

2. Make your changes following the coding guidelines below

3. Run checks before committing:
   ```bash
   make check
   ```

4. Commit your changes using [Conventional Commits](https://www.conventionalcommits.org/):
   ```bash
   git commit -m "feat(scope): add new feature"
   # or
   git commit -m "fix(scope): fix bug description"
   ```

5. Push to your fork:
   ```bash
   git push origin feature/your-feature-name
   ```

6. Create a Pull Request from your fork to the main repository

#### Commit Message Format

We follow the [Conventional Commits](https://www.conventionalcommits.org/) specification:

```
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]
```

Types:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, etc.)
- `refactor`: Code refactoring
- `perf`: Performance improvements
- `test`: Adding or updating tests
- `chore`: Build process or auxiliary tool changes

Examples:
```
feat(agent): add support for custom prompts
fix(cli): handle empty diff gracefully
docs: update installation instructions
```

## Coding Guidelines

### Go Style

- Follow the official [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Run `go fmt` before committing
- Use meaningful variable and function names
- Add comments for exported functions and types
- Keep functions focused and small

### Testing

- Write tests for new features
- Maintain or improve code coverage
- Use table-driven tests where appropriate
- Test edge cases

### Documentation

- Update README if adding new features
- Add godoc comments for public APIs
- Update configuration examples if needed

## Pull Request Guidelines

1. **Title**: Use a clear, descriptive title following conventional commits
2. **Description**: Explain what changes were made and why
3. **Tests**: Include tests for new functionality
4. **Documentation**: Update docs if needed
5. **Single Purpose**: Each PR should focus on one thing

### PR Checklist

- [ ] Code follows the project's style guidelines
- [ ] Tests pass locally (`make test`)
- [ ] Linter passes (`make lint`)
- [ ] Documentation updated (if applicable)
- [ ] Commit messages follow conventional commits

## Development Commands

```bash
# Build the binary
make build

# Install to $GOPATH/bin
make install

# Run tests
make test

# Run tests with coverage
make test-cover

# Run linter
make lint

# Format code
make fmt

# Run all checks
make check

# Clean build artifacts
make clean
```

## Getting Help

- Open an issue for questions
- Check existing documentation
- Review closed issues for similar problems

## License

By contributing, you agree that your contributions will be licensed under the MIT License.

---

Thank you for contributing to GitBuddy-Go! 🎉

