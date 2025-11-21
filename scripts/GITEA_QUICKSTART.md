# Gitea Quick Start Guide

Quick reference for installing and using Gitea with your repositories.

## Installation

### 1. Install Gitea on Your Server

```bash
# Run the installation script
./scripts/giteainstall.sh

# Install systemd service
sudo cp scripts/gitea.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable gitea
sudo systemctl start gitea

# Check status
sudo systemctl status gitea
```

### 2. Complete Web Setup

Visit `http://your-server-ip:3000` and complete the installation wizard.

## SSH Configuration (Optional)

To use system SSH on port 22 instead of Gitea's built-in SSH on port 2222:

```bash
# Run on the server as the mleku user
./scripts/gitea-ssh-setup.sh

# Restart Gitea
sudo systemctl restart gitea
```

See [GITEA_SSH_SETUP.md](GITEA_SSH_SETUP.md) for detailed SSH configuration.

## Repository Migration

### Quick Migration (HTTP - Recommended)

```bash
# 1. Generate API token in Gitea (Settings → Applications)

# 2. Configure environment
export GITEA_TOKEN="your-token-here"
export GITEA_URL="http://your-server-ip:3000"
export VPS_HOST="mleku@your-server-ip"

# 3. Run migration
./scripts/gitea-migrate-repos.sh
```

### SSH Migration (After SSH Setup)

```bash
# After configuring SSH (see above)
export GITEA_TOKEN="your-token-here"
export GITEA_URL="http://your-server-ip:3000"
export VPS_HOST="mleku@your-server-ip"
export USE_SSH=true

./scripts/gitea-migrate-repos.sh
```

## Available Scripts

| Script | Purpose |
|--------|---------|
| `giteainstall.sh` | Install Gitea to `/home/mleku/gitea` |
| `gitea-ssh-setup.sh` | Configure system SSH (port 22) |
| `gitea-migrate-repos.sh` | Migrate repositories from local directory |

## Configuration Files

| File | Purpose |
|------|---------|
| `/home/mleku/gitea/custom/conf/app.ini` | Gitea configuration |
| `scripts/gitea.service` | Systemd service file |
| `/etc/systemd/system/gitea.service` | Installed service file |

## Common Commands

### Service Management

```bash
# Start/stop/restart
sudo systemctl start gitea
sudo systemctl stop gitea
sudo systemctl restart gitea

# View status
sudo systemctl status gitea

# View logs
sudo journalctl -u gitea -f
```

### Database Location

```bash
# SQLite database
/home/mleku/gitea/data/gitea.db

# Repositories
/home/mleku/gitea/data/gitea-repositories/
```

### Backup

```bash
# Backup entire Gitea directory
tar -czf gitea-backup-$(date +%Y%m%d).tar.gz /home/mleku/gitea

# Backup database only
cp /home/mleku/gitea/data/gitea.db ~/backups/
```

## URLs and Ports

| Service | URL/Port | Purpose |
|---------|----------|---------|
| Web UI | `http://server:3000` | Gitea web interface |
| HTTP Git | `http://server:3000/user/repo.git` | HTTP clone |
| SSH (system) | `git@server:user/repo.git` | SSH clone (port 22) |
| SSH (built-in) | `git@server:2222/user/repo.git` | SSH clone (port 2222) |

## Environment Variables for Migration

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `GITEA_TOKEN` | Yes | - | API token from Gitea |
| `GITEA_URL` | No | `http://localhost:3000` | Gitea server URL |
| `VPS_HOST` | No | - | SSH host (e.g., `mleku@server`) |
| `SOURCE_DIR` | No | `/home/mleku/Documents/github` | Source repository directory |
| `USE_SSH` | No | `false` | Use SSH instead of HTTP |
| `DRY_RUN` | No | `false` | Test without making changes |

## Troubleshooting

### Gitea won't start

```bash
# Check logs
sudo journalctl -u gitea -n 50

# Check configuration
cat /home/mleku/gitea/custom/conf/app.ini

# Check permissions
ls -la /home/mleku/gitea
```

### Migration fails

```bash
# Test connection
curl http://your-server:3000/api/v1/version

# Verify token
curl -H "Authorization: token ${GITEA_TOKEN}" \
     http://your-server:3000/api/v1/user

# Try dry run first
DRY_RUN=true ./scripts/gitea-migrate-repos.sh
```

### SSH not working

```bash
# Test SSH
ssh -T git@your-server

# Check Gitea SSH config
grep SSH /home/mleku/gitea/custom/conf/app.ini

# Check SSH keys in Gitea
# Settings → SSH/GPG Keys
```

## Security Notes

1. **Firewall**: Open required ports (3000 for HTTP, 22 for SSH)
2. **API Token**: Keep your API token secure, never commit it
3. **SSH Keys**: Add your public SSH key to Gitea for SSH access
4. **Backups**: Regularly backup `/home/mleku/gitea`
5. **HTTPS**: Consider setting up TLS for production use

## Next Steps

After installation:

1. ✅ Create admin account (via web interface)
2. ✅ Generate API token (Settings → Applications)
3. ✅ Configure SSH (optional, see GITEA_SSH_SETUP.md)
4. ✅ Migrate repositories (see GITEA_MIGRATION.md)
5. ✅ Set up backups (automated cron job recommended)
6. ✅ Configure HTTPS (for production)

## Getting Help

- **Gitea Documentation**: See [GITEA_MIGRATION.md](GITEA_MIGRATION.md) and [GITEA_SSH_SETUP.md](GITEA_SSH_SETUP.md)
- **Gitea Logs**: `sudo journalctl -u gitea -f`
- **Configuration**: `/home/mleku/gitea/custom/conf/app.ini`
- **Official Docs**: https://docs.gitea.com/
