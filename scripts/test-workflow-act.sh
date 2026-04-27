#!/usr/bin/env bash
# Run GitHub Actions workflow locally using act
# Usage: ./scripts/test-workflow-local.sh [job-name]
#   job-name: optional, defaults to 'build'. Can be 'build' or 'release'

set -e

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
WORKFLOW_FILE="${SCRIPT_DIR}/../.github/workflows/go.yml"
JOB_NAME="${1:-build}"

# Check if act is installed
if ! command -v act >/dev/null 2>&1; then
    echo "Error: 'act' is not installed"
    echo "Install it with:"
    echo "  curl https://raw.githubusercontent.com/nektos/act/master/install.sh | sudo bash"
    echo "  # or on macOS: brew install act"
    exit 1
fi

echo "=== Running GitHub Actions workflow locally ==="
echo "Workflow: .github/workflows/go.yml"
echo "Job: $JOB_NAME"
echo ""

case "$JOB_NAME" in
    build)
        echo "Running build job..."
        act push --workflows "$WORKFLOW_FILE" --job build
        ;;
    release)
        echo "Running release job (simulating tag push)..."
        # Simulate a tag push event with a valid tag format
        # The workflow requires build to run first and succeed
        echo "Step 1: Running build job (required dependency)..."
        if ! act push --workflows "$WORKFLOW_FILE" --job build; then
            echo "Error: Build job failed. Release job cannot proceed."
            exit 1
        fi
        
        echo ""
        echo "Step 2: Running release job..."
        echo "Note: GitHub release creation may fail locally (no valid token), but binary building will be tested"
        # Use a tag that matches the workflow pattern: v[0-9]+.[0-9]+.[0-9]+
        # Provide a dummy GITHUB_TOKEN to prevent immediate failure
        # The release won't actually be created, but the workflow will test binary building
        # Temporarily disable exit on error to allow release step to fail gracefully
        set +e
        GITHUB_REF=refs/tags/v1.0.0 \
        GITHUB_TOKEN=dummy_token_for_local_testing \
        act push \
            --workflows "$WORKFLOW_FILE" \
            --job release \
            --secret GITHUB_TOKEN=dummy_token_for_local_testing \
            --eventpath /dev/stdin <<EOF
{
  "ref": "refs/tags/v1.0.0",
  "pusher": {"name": "test"},
  "repository": {
    "name": "git.smesh.lol/orly",
    "full_name": "test/git.smesh.lol/orly"
  },
  "head_commit": {
    "id": "test123"
  }
}
EOF
        RELEASE_EXIT_CODE=$?
        set -e
        
        # Check if binary building succeeded (exit code 0) or if only release creation failed
        if [ $RELEASE_EXIT_CODE -eq 0 ]; then
            echo "✓ Release job completed successfully (including binary building)"
        else
            echo "⚠ Release job completed with errors (likely GitHub release creation failed)"
            echo "  This is expected in local testing. Binary building should have succeeded."
            echo "  Check the output above to verify 'Build Release Binaries' step succeeded."
        fi
        ;;
    *)
        echo "Error: Unknown job '$JOB_NAME'"
        echo "Valid jobs: build, release"
        exit 1
        ;;
esac

echo ""
echo "=== Workflow completed ==="

