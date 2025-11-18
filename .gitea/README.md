# Gitea Actions Setup

This directory contains workflows for Gitea Actions, which is a self-hosted CI/CD system compatible with GitHub Actions syntax.

## Workflow: go.yml

The `go.yml` workflow handles building, testing, and releasing the ORLY relay when version tags are pushed.

### Features

- **No external dependencies**: Uses only inline shell commands (no actions from GitHub)
- **Pure Go builds**: Uses CGO_ENABLED=0 with purego for secp256k1
- **Automated releases**: Creates Gitea releases with binaries and checksums
- **Tests included**: Runs the full test suite before building releases

### Prerequisites

1. **Gitea Token**: Add a secret named `GITEA_TOKEN` in your repository settings
   - Go to: Repository Settings → Secrets → Add Secret
   - Name: `GITEA_TOKEN`
   - Value: Your Gitea personal access token with `repo` and `write:packages` permissions

2. **Runner Configuration**: Ensure your Gitea Actions runner is properly configured
   - The runner should have access to pull Docker images
   - Ubuntu-latest image should be available

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
#    - Build the binary
#    - Run tests
#    - Create a release on your Gitea instance
#    - Upload the binary and checksums
```

### Environment Variables

The workflow uses standard Gitea Actions environment variables:

- `GITHUB_WORKSPACE`: Working directory for the job
- `GITHUB_REF_NAME`: Tag name (e.g., v1.2.3)
- `GITHUB_REPOSITORY`: Repository in format `owner/repo`
- `GITHUB_SERVER_URL`: Your Gitea instance URL (e.g., https://git.nostrdev.com)

### Troubleshooting

**Issue**: Workflow fails to clone repository
- **Solution**: Check that the repository is accessible without authentication, or configure runner credentials

**Issue**: Cannot create release
- **Solution**: Verify `GITEA_TOKEN` secret is set correctly with appropriate permissions

**Issue**: Go version not found
- **Solution**: The workflow downloads Go 1.25.0 directly from go.dev, ensure the runner has internet access

### Customization

To modify the workflow:

1. Edit `.gitea/workflows/go.yml`
2. Test changes by pushing a tag (or use `act` locally for testing)
3. Monitor the Actions tab in your Gitea repository for results

## Differences from GitHub Actions

- **Action dependencies**: This workflow doesn't use external actions (like `actions/checkout@v4`) to avoid GitHub dependency
- **Release creation**: Uses `tea` CLI instead of GitHub's release action
- **Inline commands**: All setup and build steps are done with shell scripts

This makes the workflow completely self-contained and independent of external services.
