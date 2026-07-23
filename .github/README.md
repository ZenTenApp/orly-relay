# GitHub Actions Setup

This directory contains workflows and issue templates for GitHub Actions.

## Workflow: go.yml

The `go.yml` workflow handles building, testing, and releasing the ORLY relay when version tags are pushed.

### Features

- **Standard GitHub Actions**: Uses `actions/checkout`, `actions/setup-go`, `oven-sh/setup-bun`, and `softprops/action-gh-release`
- **Pure Go builds**: Uses `CGO_ENABLED=0` with purego for secp256k1
- **Automated releases**: Creates GitHub releases with binaries and checksums
- **Tests included**: Runs the full test suite before building releases

### Prerequisites

The workflow uses the built-in `GITHUB_TOKEN` to create releases, so no additional
secrets are required. The job requests `contents: write` permission, which is
declared in the workflow.

If your organization restricts the default token permissions, ensure that
**Settings → Actions → General → Workflow permissions** allows read and write
access (or grants `contents: write` to Actions).

### Usage

To create a new release:

```bash
# 1. Update version in pkg/version/version file
echo "v0.29.4" > pkg/version/version

# 2. Commit the version change
git add pkg/version/version
git commit -m "bump to v0.29.4"

# 3. Create and push the tag
git tag v0.29.4
git push origin v0.29.4

# 4. The workflow will automatically:
#    - Build the binaries
#    - Run tests
#    - Create a release on GitHub
#    - Upload the binaries and checksums
```

### Environment Variables

The workflow uses standard GitHub Actions environment variables:

- `GITHUB_WORKSPACE`: Working directory for the job
- `GITHUB_REF_NAME`: Tag name (e.g., v1.2.3)
- `GITHUB_REPOSITORY`: Repository in format `owner/repo`

### Troubleshooting

**Issue**: Cannot create release
- **Solution**: Verify the workflow has `contents: write` permission and that the
  repository's Actions workflow permissions allow write access.

**Issue**: Go version not found
- **Solution**: `actions/setup-go` installs Go 1.25.3. Adjust the `go-version` in
  `go.yml` if a different version is required.

### Customization

To modify the workflow:

1. Edit `.github/workflows/go.yml`
2. Test changes by pushing a tag (or use `act` locally for testing)
3. Monitor the Actions tab in the GitHub repository for results

## Issue Templates

Issue templates live in `.github/ISSUE_TEMPLATE/`:

- `bug_report.yaml` - Bug report form
- `feature_request.yaml` - Feature request form
- `config.yml` - Template chooser configuration and contact links
