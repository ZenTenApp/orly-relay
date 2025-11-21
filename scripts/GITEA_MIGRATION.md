# Gitea Repository Migration Guide

This guide explains how to migrate your existing Git repositories from `/home/mleku/Documents/github` to your Gitea installation.

## Prerequisites

1. **Gitea is installed and running** on your VPS (use `giteainstall.sh`)
2. **You have completed the Gitea setup wizard** and created an admin account
3. **SSH access** to your VPS (if migrating to remote server)
4. **Git is installed** on your local machine

## Step 1: Generate Gitea API Token

1. Log in to your Gitea instance at `http://your-vps-ip:3000`
2. Click on your avatar → **Settings**
3. Navigate to **Applications** → **Manage Access Tokens**
4. Click **Generate New Token**
5. Give it a name (e.g., "migration")
6. Select the following scopes:
   - `read:repository`
   - `write:repository`
   - `read:user`
   - `write:user`
   - Or simply select **all** scopes for convenience
7. Click **Generate Token**
8. **Copy the token** (you won't be able to see it again!)

## Step 2: Configure Environment Variables

On your **local machine** (where the repositories are), set up the configuration:

```bash
# Gitea API token (required)
export GITEA_TOKEN="your-token-here"

# Gitea URL (update with your VPS IP or domain)
export GITEA_URL="http://your-vps-ip:3000"

# VPS SSH connection (if migrating to remote server)
# Format: user@hostname or user@ip
export VPS_HOST="mleku@your-vps-ip"

# Source directory (default is /home/mleku/Documents/github)
export SOURCE_DIR="/home/mleku/Documents/github"
```

### For Local Gitea Installation

If Gitea is installed locally (on the same machine as the repos):

```bash
export GITEA_TOKEN="your-token-here"
export GITEA_URL="http://localhost:3000"
# Don't set VPS_HOST
```

## Step 3: Test with Dry Run

Before actually migrating, do a dry run to see what would happen:

```bash
DRY_RUN=true ./scripts/gitea-migrate-repos.sh
```

This will:
- Check connectivity to Gitea
- List all repositories to be migrated
- Show what would be created
- **Not make any actual changes**

## Step 4: Run the Migration

Once you've verified the dry run looks good:

```bash
./scripts/gitea-migrate-repos.sh
```

The script will:
1. Connect to Gitea and verify your token
2. Scan for all Git repositories in the source directory
3. Ask for confirmation before proceeding
4. For each repository:
   - Check if it already exists in Gitea
   - Create the repository via API (if it doesn't exist)
   - Push all branches and tags to Gitea
   - Report success or failure
5. Display a summary at the end

## Step 5: Verify Migration

After the migration completes:

1. Visit your Gitea instance: `http://your-vps-ip:3000`
2. Check that all repositories are visible
3. Browse a few repositories to verify the content
4. Check that branches and tags were migrated

## Configuration Options

The script supports several environment variables:

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `GITEA_TOKEN` | Gitea API token | - | Yes |
| `GITEA_URL` | Gitea server URL | `http://localhost:3000` | No |
| `VPS_HOST` | SSH host for VPS | - | No (if local) |
| `SOURCE_DIR` | Source repo directory | `/home/mleku/Documents/github` | No |
| `DRY_RUN` | Dry run mode | `false` | No |

## SSH Configuration for Remote VPS

If migrating to a remote VPS, ensure SSH access is configured:

```bash
# Test SSH connection
ssh mleku@your-vps-ip

# Set up SSH key if not already done
ssh-copy-id mleku@your-vps-ip
```

### SSH Port Configuration

By default, Gitea uses SSH port **2222** (to avoid conflicts with system SSH on port 22).

If you need to use a different SSH port, you'll need to:

1. Update Gitea configuration in `/home/mleku/gitea/custom/conf/app.ini`:
   ```ini
   [server]
   SSH_LISTEN_PORT = 2222  # Change this to your preferred port
   ```

2. Restart Gitea:
   ```bash
   sudo systemctl restart gitea
   ```

## Troubleshooting

### "Cannot connect to Gitea"

- Check that Gitea is running: `sudo systemctl status gitea`
- Verify the URL is correct
- Check firewall rules if accessing remotely

### "Invalid Gitea token"

- Regenerate the token in Gitea settings
- Make sure you copied the entire token
- Verify the token has the necessary scopes

### "Failed to push"

- Check SSH connectivity: `ssh -p 2222 mleku@your-vps-ip`
- Verify SSH keys are set up correctly
- Check Gitea logs: `sudo journalctl -u gitea -f`

### "Repository already exists"

The script will skip repositories that already exist. You can:
- Answer "y" when prompted to push updates to existing repos
- Delete the repo in Gitea and run the script again
- Manually push updates using git

## Example: Complete Migration

Here's a complete example for migrating to a VPS:

```bash
# 1. Set up environment
export GITEA_TOKEN="a1b2c3d4e5f6g7h8i9j0"
export GITEA_URL="http://192.168.1.100:3000"
export VPS_HOST="mleku@192.168.1.100"

# 2. Test connection and do dry run
DRY_RUN=true ./scripts/gitea-migrate-repos.sh

# 3. Review output, then run actual migration
./scripts/gitea-migrate-repos.sh

# 4. Confirm when prompted
# Press 'y' to continue

# 5. Wait for migration to complete
# The script will show progress for each repository

# 6. Review summary
# Check for any failures and investigate
```

## Post-Migration

After successful migration:

1. **Update remote URLs** in your local development repositories:
   ```bash
   cd /home/mleku/Documents/github/your-repo
   git remote set-url origin http://your-vps-ip:3000/mleku/your-repo.git
   ```

2. **Set up SSH URLs** for password-less push (recommended):
   ```bash
   git remote set-url origin ssh://mleku@your-vps-ip:2222/mleku/your-repo.git
   ```

3. **Test push/pull**:
   ```bash
   git pull origin main
   git push origin main
   ```

## Statistics

Your migration includes:
- **86 repositories** found in `/home/mleku/Documents/github`
- All branches will be migrated
- All tags will be migrated
- Repository descriptions will be extracted from README.md or .git/description

## Support

If you encounter issues:
1. Check Gitea logs: `sudo journalctl -u gitea -f`
2. Enable verbose Git output: `GIT_TRACE=1 ./scripts/gitea-migrate-repos.sh`
3. Check the script output for specific error messages
