# Contributing to ORLY

Thank you for your interest in contributing to ORLY! This document outlines the process for reporting bugs, requesting features, and submitting contributions.

**Canonical Repository:** https://git.nostrdev.com/mleku/git.smesh.lol/orly

## Issue Reporting Policy

### Before Opening an Issue

1. **Search existing issues** to avoid duplicates
2. **Check the documentation** in the repository
3. **Verify your version** - run `./orly version` and ensure you're on a recent release
4. **Review the CLAUDE.md** file for configuration guidance

### Bug Reports

Use the **Bug Report** template when reporting unexpected behavior. A good bug report includes:

- **Version information** - exact ORLY version from `./orly version`
- **Database backend** - Badger, Neo4j, or WasmDB
- **Clear description** - what happened vs. what you expected
- **Reproduction steps** - detailed steps to trigger the bug
- **Logs** - relevant log output (use `ORLY_LOG_LEVEL=debug` or `trace`)
- **Configuration** - relevant environment variables (redact secrets)

#### Log Levels for Debugging

```bash
export ORLY_LOG_LEVEL=trace   # Most verbose
export ORLY_LOG_LEVEL=debug   # Development debugging
export ORLY_LOG_LEVEL=info    # Default
```

### Feature Requests

Use the **Feature Request** template when suggesting new functionality. A good feature request includes:

- **Problem statement** - what problem does this solve?
- **Proposed solution** - specific description of desired behavior
- **Alternatives considered** - workarounds you've tried
- **Related NIP** - if this implements a Nostr protocol specification
- **Impact assessment** - is this a minor tweak or major change?

#### Feature Categories

- **Protocol** - NIP implementations and Nostr protocol features
- **Database** - Storage backends, indexing, query optimization
- **Performance** - Caching, SIMD operations, memory optimization
- **Policy** - Access control, event filtering, validation
- **Web UI** - Admin interface improvements
- **Operations** - Deployment, monitoring, systemd integration

## Code Contributions

### Development Setup

```bash
# Clone the repository
git clone https://git.nostrdev.com/mleku/git.smesh.lol/orly.git
cd git.smesh.lol/orly

# Build
CGO_ENABLED=0 go build -o orly

# Run tests
./scripts/test.sh

# Build with web UI
./scripts/update-embedded-web.sh
```

### Pull Request Guidelines

1. **One feature/fix per PR** - keep changes focused
2. **Write tests** - for new functionality and bug fixes
3. **Follow existing patterns** - match the code style of surrounding code
4. **Update documentation** - if your change affects configuration or behavior
5. **Test your changes** - run `./scripts/test.sh` before submitting

### Commit Message Format

```
Short summary (72 chars max, imperative mood)

- Bullet point describing change 1
- Bullet point describing change 2

Files modified:
- path/to/file1.go: Description of change
- path/to/file2.go: Description of change
```

## Communication

- **Issues:** https://git.nostrdev.com/mleku/git.smesh.lol/orly/issues
- **Documentation:** https://git.nostrdev.com/mleku/git.smesh.lol/orly

## License

By contributing to ORLY, you agree that your contributions will be licensed under the same license as the project.
