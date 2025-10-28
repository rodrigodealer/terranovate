# Pre-commit Hooks

Terranovate includes pre-commit hooks to ensure code quality and consistency.

## Installation

### 1. Install pre-commit

```bash
# macOS
brew install pre-commit

# Ubuntu/Debian
sudo apt-get install pre-commit

# pip
pip install pre-commit
```

### 2. Install the git hooks

```bash
cd terranovate
pre-commit install
```

## Hooks Included

### Go Formatting and Linting

- **go-fmt**: Formats Go code with `gofmt`
- **go-imports**: Formats imports with `goimports`
- **go-vet**: Runs `go vet` to find suspicious constructs
- **go-mod-tidy**: Ensures go.mod and go.sum are tidy
- **golangci-lint**: Comprehensive linting with multiple linters

### General File Checks

- **check-added-large-files**: Prevents committing large files (>1MB)
- **check-yaml**: Validates YAML syntax
- **check-merge-conflict**: Detects merge conflict markers
- **trailing-whitespace**: Removes trailing whitespace
- **end-of-file-fixer**: Ensures files end with a newline
- **detect-private-key**: Prevents committing private keys

### Markdown

- **markdownlint**: Lints and fixes markdown files

### Testing

- **go-test**: Runs all tests with race detection

## Usage

### Automatic (on git commit)

Hooks run automatically when you commit:

```bash
git add .
git commit -m "feat: add new feature"
# Hooks run automatically
```

### Manual Execution

Run hooks on all files:

```bash
pre-commit run --all-files
```

Run specific hook:

```bash
pre-commit run golangci-lint --all-files
```

Run on specific files:

```bash
pre-commit run --files path/to/file.go
```

## Skipping Hooks

In rare cases where you need to skip hooks:

```bash
git commit --no-verify -m "emergency fix"
```

⚠️ **Warning**: Only use `--no-verify` in emergencies. Skipping hooks can introduce issues.

## Configuration

### Pre-commit Config

Configuration is in `.pre-commit-config.yaml`. To update hook versions:

```bash
pre-commit autoupdate
```

### golangci-lint Config

Linter settings are in `.golangci.yml`. Customize as needed:

```yaml
linters:
  enable:
    - errcheck
    - gosimple
    - govet
    # ... add more
```

### Markdown Config

Markdown rules are in `.markdownlint.json`:

```json
{
  "default": true,
  "MD013": {
    "line_length": 120
  }
}
```

## Troubleshooting

### Hooks Not Running

```bash
# Reinstall hooks
pre-commit uninstall
pre-commit install
```

### golangci-lint Timeout

Increase timeout in `.pre-commit-config.yaml`:

```yaml
- id: golangci-lint
  args:
    - --timeout=10m  # Increase from 5m
```

### False Positives

Disable specific linters in `.golangci.yml`:

```yaml
linters:
  disable:
    - errcheck  # Example
```

Or add inline comments:

```go
//nolint:errcheck // Reason for disabling
func example() {
    doSomething()
}
```

## CI/CD Integration

Pre-commit can run in CI to catch issues:

```yaml
# GitHub Actions
- name: Run pre-commit
  uses: pre-commit/action@v3.0.0
```

## Best Practices

1. ✅ **Run before pushing**: `pre-commit run --all-files`
2. ✅ **Keep hooks updated**: `pre-commit autoupdate` monthly
3. ✅ **Fix issues, don't skip**: Address hook failures properly
4. ✅ **Customize for your team**: Adjust rules in configs
5. ❌ **Don't disable security hooks**: Keep `detect-private-key` enabled

## Hook Details

### Go Test Hook

The test hook runs with:
- `-race`: Race condition detection
- `-timeout=2m`: 2-minute timeout
- `./...`: All packages

Customize in `.pre-commit-config.yaml`:

```yaml
- id: go-test
  name: go test
  entry: go test -v -race -timeout=5m -short ./...
  language: system
  pass_filenames: false
  files: \.go$
```

### Performance

Hooks are optimized to run only on changed files when possible. Initial run on all files may take longer.

## Further Reading

- [pre-commit documentation](https://pre-commit.com/)
- [golangci-lint documentation](https://golangci-lint.run/)
- [markdownlint rules](https://github.com/DavidAnson/markdownlint/blob/main/doc/Rules.md)
