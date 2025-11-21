#!/usr/bin/env bash
set -euo pipefail

# Gitea Repository Migration Script
# Migrates Git repositories from local directory to Gitea server

# Configuration (edit these values)
GITEA_URL="${GITEA_URL:-http://localhost:3000}"  # Gitea URL
GITEA_TOKEN="${GITEA_TOKEN:-}"                    # Gitea API token (required)
SOURCE_DIR="${SOURCE_DIR:-/home/mleku/Documents/github}"
VPS_HOST="${VPS_HOST:-}"                          # SSH host for VPS (e.g., user@vps.example.com)
USE_SSH="${USE_SSH:-false}"                       # Set to true to use SSH instead of HTTP for push
DRY_RUN="${DRY_RUN:-false}"                       # Set to true for dry run

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

# Stats
TOTAL_REPOS=0
CREATED_REPOS=0
PUSHED_REPOS=0
SKIPPED_REPOS=0
FAILED_REPOS=0

echo -e "${GREEN}=== Gitea Repository Migration Script ===${NC}"
echo ""

# Validate configuration
if [ -z "$GITEA_TOKEN" ]; then
    echo -e "${RED}Error: GITEA_TOKEN is required${NC}"
    echo ""
    echo "To generate a Gitea API token:"
    echo "1. Log in to Gitea at ${GITEA_URL}"
    echo "2. Go to Settings -> Applications -> Generate New Token"
    echo "3. Give it a name (e.g., 'migration') and select all scopes"
    echo "4. Copy the token and run:"
    echo "   export GITEA_TOKEN='your-token-here'"
    echo ""
    exit 1
fi

if [ ! -d "$SOURCE_DIR" ]; then
    echo -e "${RED}Error: Source directory does not exist: ${SOURCE_DIR}${NC}"
    exit 1
fi

echo "Configuration:"
echo "  Gitea URL: ${GITEA_URL}"
echo "  Source directory: ${SOURCE_DIR}"
echo "  VPS Host: ${VPS_HOST:-<local installation>}"
echo "  Push method: $([ "$USE_SSH" = "true" ] && echo "SSH" || echo "HTTP with token")"
echo "  Dry run: ${DRY_RUN}"
echo ""

# Function to check if Gitea is accessible
check_gitea() {
    echo -e "${YELLOW}Checking Gitea accessibility...${NC}"

    if ! curl -sf "${GITEA_URL}/api/v1/version" > /dev/null; then
        echo -e "${RED}Error: Cannot connect to Gitea at ${GITEA_URL}${NC}"
        echo "Make sure Gitea is running and accessible."
        exit 1
    fi

    # Verify token
    if ! curl -sf -H "Authorization: token ${GITEA_TOKEN}" "${GITEA_URL}/api/v1/user" > /dev/null; then
        echo -e "${RED}Error: Invalid Gitea token${NC}"
        exit 1
    fi

    GITEA_USER=$(curl -sf -H "Authorization: token ${GITEA_TOKEN}" "${GITEA_URL}/api/v1/user" | grep -o '"login":"[^"]*"' | cut -d'"' -f4)
    echo -e "${GREEN}✓ Connected to Gitea as: ${GITEA_USER}${NC}"
}

# Function to find all Git repositories
find_repos() {
    echo -e "${YELLOW}Scanning for Git repositories...${NC}"
    mapfile -t REPOS < <(find "${SOURCE_DIR}" -maxdepth 2 -name .git -type d | sed 's|/.git$||')
    TOTAL_REPOS=${#REPOS[@]}
    echo -e "${GREEN}✓ Found ${TOTAL_REPOS} repositories${NC}"
    echo ""
}

# Function to get repository name from path
get_repo_name() {
    basename "$1"
}

# Function to get repository description from Git config
get_repo_description() {
    local repo_path="$1"
    local desc=""

    # Try to get description from .git/description
    if [ -f "${repo_path}/.git/description" ]; then
        desc=$(cat "${repo_path}/.git/description" | grep -v "^Unnamed repository" || true)
    fi

    # If empty, try README
    if [ -z "$desc" ] && [ -f "${repo_path}/README.md" ]; then
        desc=$(head -n 1 "${repo_path}/README.md" | sed 's/^#*\s*//')
    fi

    echo "$desc"
}

# Function to check if repo exists in Gitea
repo_exists() {
    local repo_name="$1"
    curl -sf -H "Authorization: token ${GITEA_TOKEN}" \
        "${GITEA_URL}/api/v1/repos/${GITEA_USER}/${repo_name}" > /dev/null
}

# Function to create repository in Gitea
create_repo() {
    local repo_name="$1"
    local description="$2"

    if [ "$DRY_RUN" = "true" ]; then
        echo -e "${BLUE}[DRY RUN]${NC} Would create: ${repo_name}"
        return 0
    fi

    local json_data=$(cat <<EOF
{
    "name": "${repo_name}",
    "description": "${description}",
    "private": false,
    "auto_init": false
}
EOF
)

    if curl -sf -X POST \
        -H "Authorization: token ${GITEA_TOKEN}" \
        -H "Content-Type: application/json" \
        -d "$json_data" \
        "${GITEA_URL}/api/v1/user/repos" > /dev/null; then
        return 0
    else
        return 1
    fi
}

# Function to push repository to Gitea
push_repo() {
    local repo_path="$1"
    local repo_name="$2"

    if [ "$DRY_RUN" = "true" ]; then
        echo -e "${BLUE}[DRY RUN]${NC} Would push: ${repo_name}"
        return 0
    fi

    cd "$repo_path"

    # Check if gitea remote already exists
    if git remote | grep -q "^gitea$"; then
        git remote remove gitea 2>/dev/null || true
    fi

    # Build Git URL based on SSH or HTTP preference
    local git_url
    if [ "$USE_SSH" = "true" ]; then
        # Use SSH URL
        if [ -n "$VPS_HOST" ]; then
            # Extract host from VPS_HOST (format: user@host or just host)
            local ssh_host=$(echo "$VPS_HOST" | grep -oP '@\K.*' || echo "$VPS_HOST")
            git_url="git@${ssh_host}:${GITEA_USER}/${repo_name}.git"
        else
            # Use local host
            local gitea_host=$(echo "$GITEA_URL" | sed -E 's|https?://||' | cut -d':' -f1)
            git_url="git@${gitea_host}:${GITEA_USER}/${repo_name}.git"
        fi
    else
        # Use HTTP URL with token authentication (more reliable for automation)
        # Extract the base URL (http:// or https://)
        local protocol=$(echo "$GITEA_URL" | grep -oP '^https?')
        local url_without_protocol=$(echo "$GITEA_URL" | sed -E 's|^https?://||')

        # Build authenticated URL: http://username:token@host:port/username/repo.git
        git_url="${protocol}://${GITEA_USER}:${GITEA_TOKEN}@${url_without_protocol}/${GITEA_USER}/${repo_name}.git"
    fi

    # Add Gitea remote with URL
    git remote add gitea "$git_url"

    # Push all branches and tags
    local push_success=true

    # Push all branches
    if ! git push gitea --all 2>&1; then
        push_success=false
    fi

    # Push all tags
    if ! git push gitea --tags 2>&1; then
        push_success=false
    fi

    # Clean up - remove the remote to avoid storing token in git config (HTTP only)
    if [ "$USE_SSH" != "true" ]; then
        git remote remove gitea 2>/dev/null || true
    fi

    if [ "$push_success" = true ]; then
        return 0
    else
        return 1
    fi
}

# Main migration function
migrate_repos() {
    echo -e "${GREEN}Starting migration...${NC}"
    echo ""

    local count=0
    for repo_path in "${REPOS[@]}"; do
        count=$((count + 1))
        local repo_name=$(get_repo_name "$repo_path")

        echo -e "${BLUE}[${count}/${TOTAL_REPOS}]${NC} Processing: ${repo_name}"

        # Check if repo already exists
        if repo_exists "$repo_name"; then
            echo -e "  ${YELLOW}⚠${NC}  Repository already exists in Gitea"
            SKIPPED_REPOS=$((SKIPPED_REPOS + 1))

            # Ask if user wants to push updates
            if [ "$DRY_RUN" = "false" ]; then
                    if push_repo "$repo_path" "$repo_name"; then
                        echo -e "  ${GREEN}✓${NC}  Pushed updates"
                        PUSHED_REPOS=$((PUSHED_REPOS + 1))
                    else
                        echo -e "  ${RED}✗${NC}  Failed to push"
                        FAILED_REPOS=$((FAILED_REPOS + 1))
                fi
            fi
            echo ""
            continue
        fi

        # Get description
        local description=$(get_repo_description "$repo_path")
        if [ -n "$description" ]; then
            echo -e "  Description: ${description}"
        fi

        # Create repository
        echo -e "  Creating repository in Gitea..."
        if create_repo "$repo_name" "$description"; then
            echo -e "  ${GREEN}✓${NC}  Repository created"
            CREATED_REPOS=$((CREATED_REPOS + 1))

            # Push repository
            echo -e "  Pushing repository data..."
            if push_repo "$repo_path" "$repo_name"; then
                echo -e "  ${GREEN}✓${NC}  Repository pushed"
                PUSHED_REPOS=$((PUSHED_REPOS + 1))
            else
                echo -e "  ${RED}✗${NC}  Failed to push"
                FAILED_REPOS=$((FAILED_REPOS + 1))
            fi
        else
            echo -e "  ${RED}✗${NC}  Failed to create repository"
            FAILED_REPOS=$((FAILED_REPOS + 1))
        fi

        echo ""
    done
}

# Print summary
print_summary() {
    echo ""
    echo -e "${GREEN}=== Migration Summary ===${NC}"
    echo "Total repositories: ${TOTAL_REPOS}"
    echo "Created: ${CREATED_REPOS}"
    echo "Pushed: ${PUSHED_REPOS}"
    echo "Skipped: ${SKIPPED_REPOS}"
    echo "Failed: ${FAILED_REPOS}"
    echo ""

    if [ "$DRY_RUN" = "true" ]; then
        echo -e "${YELLOW}This was a dry run. No changes were made.${NC}"
        echo "Run without DRY_RUN=true to perform actual migration."
    elif [ $FAILED_REPOS -eq 0 ]; then
        echo -e "${GREEN}✓ Migration completed successfully!${NC}"
        echo ""
        echo "Visit your Gitea instance at: ${GITEA_URL}"
    else
        echo -e "${YELLOW}⚠ Migration completed with some failures${NC}"
        echo "Check the output above for details on failed repositories."
    fi
}

# Main execution
check_gitea
find_repos

# Confirm before proceeding
if [ "$DRY_RUN" = "false" ]; then
    echo -e "${YELLOW}Ready to migrate ${TOTAL_REPOS} repositories.${NC}"
    read -p "Continue? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Migration cancelled."
        exit 0
    fi
    echo ""
fi

migrate_repos
print_summary
