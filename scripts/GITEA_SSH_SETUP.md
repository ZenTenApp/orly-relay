# Gitea SSH Setup Guide

This guide explains how to configure Gitea to use your system's SSH server on port 22 instead of running its own SSH server on port 2222.

## Overview

By default, Gitea runs its own SSH server on port 2222 to avoid conflicts with the system SSH server. However, you can configure it to use the system SSH daemon on port 22, which provides:

- **Single SSH Port**: No need to remember port 2222
- **Standard Git URLs**: Use standard `git@host:repo.git` format
- **Centralized SSH Management**: All SSH traffic through system sshd
- **Better Integration**: Works seamlessly with existing SSH infrastructure

## Quick Setup (Automated)

Use the provided script to configure everything automatically:

```bash
# Run as the gitea user (mleku)
./scripts/gitea-ssh-setup.sh
```

The script will:
- Configure Gitea to use system SSH
- Update `app.ini` with correct SSH settings
- Back up your existing configuration
- Display next steps

## Manual Setup

If you prefer to configure manually, follow these steps:

### Step 1: Update Gitea Configuration

Edit `/home/mleku/gitea/custom/conf/app.ini` and update the `[server]` section:

```ini
[server]
# ... other settings ...

# Disable Gitea's built-in SSH server
START_SSH_SERVER = false

# SSH domain (use your server's hostname or IP)
SSH_DOMAIN       = your-server.com

# SSH port (system SSH port)
SSH_PORT         = 22

# Don't disable SSH protocol support
DISABLE_SSH      = false
```

### Step 2: Restart Gitea

```bash
sudo systemctl restart gitea
```

### Step 3: Verify Configuration

Check that Gitea is no longer listening on port 2222:

```bash
# Should not show Gitea on port 2222
sudo netstat -tlnp | grep 2222

# System SSH should be on port 22
sudo netstat -tlnp | grep :22
```

## Adding SSH Keys to Gitea

Once configured, users need to add their SSH public keys to Gitea:

### Generate SSH Key (if needed)

```bash
# Generate a new ED25519 key (recommended)
ssh-keygen -t ed25519 -C "your-email@example.com"

# Or generate RSA key (older systems)
ssh-keygen -t rsa -b 4096 -C "your-email@example.com"
```

### Add Key to Gitea

1. **Copy your public key:**
   ```bash
   cat ~/.ssh/id_ed25519.pub
   # or
   cat ~/.ssh/id_rsa.pub
   ```

2. **Add to Gitea:**
   - Log in to Gitea web interface
   - Click your avatar → **Settings**
   - Navigate to **SSH / GPG Keys**
   - Click **Add Key**
   - Paste your public key
   - Give it a name (e.g., "My Laptop")
   - Click **Add Key**

### Test SSH Connection

```bash
# Test SSH authentication
ssh -T git@your-server.com

# Expected output:
# Hi there, username! You've successfully authenticated, but Gitea does not provide shell access.
```

## Using SSH with Git

Once SSH is configured and your key is added, you can use standard Git SSH URLs:

### Clone Repository

```bash
# Standard SSH URL format
git clone git@your-server.com:username/repo-name.git

# Example
git clone git@your-server.com:mleku/orly.git
```

### Add Remote to Existing Repository

```bash
cd /path/to/your/repo
git remote add origin git@your-server.com:mleku/repo-name.git
```

### Push/Pull

```bash
git push origin main
git pull origin main
```

## Update Migration Script for SSH

If you want to use SSH for repository migration instead of HTTP, update the environment:

```bash
# Don't set VPS_HOST - SSH will use standard port 22
export GITEA_TOKEN="your-token"
export GITEA_URL="http://your-server:3000"  # Still needed for API

# Update migration script to use SSH
# (You would need to modify the script to support this)
```

## Troubleshooting

### "Permission denied (publickey)"

**Problem**: SSH key not added or not being used

**Solutions**:
1. Verify key is added to Gitea: Settings → SSH/GPG Keys
2. Ensure SSH agent has your key:
   ```bash
   ssh-add -l
   ssh-add ~/.ssh/id_ed25519  # Add if not listed
   ```
3. Test with verbose output:
   ```bash
   ssh -vvv -T git@your-server.com
   ```

### "Could not resolve hostname"

**Problem**: DNS or hostname issue

**Solutions**:
1. Use IP address instead:
   ```bash
   git clone git@192.168.1.100:mleku/repo.git
   ```
2. Add entry to `/etc/hosts`:
   ```
   192.168.1.100  your-server.com
   ```

### SSH Uses Wrong Port

**Problem**: Git trying to use port 2222

**Solutions**:
1. Verify `app.ini` has `SSH_PORT = 22`
2. Restart Gitea: `sudo systemctl restart gitea`
3. Check repository remote URL:
   ```bash
   git remote -v
   # Should show: git@host:user/repo.git (not port 2222)
   ```

### Gitea Still Listening on Port 2222

**Problem**: Built-in SSH server still running

**Solutions**:
1. Verify `START_SSH_SERVER = false` in `app.ini`
2. Restart Gitea: `sudo systemctl restart gitea`
3. Check logs: `sudo journalctl -u gitea -f`

## SSH Configuration Files

### Gitea Configuration

Location: `/home/mleku/gitea/custom/conf/app.ini`

Relevant settings:
```ini
[server]
START_SSH_SERVER = false   # Don't run built-in SSH
SSH_DOMAIN       = localhost
SSH_PORT         = 22       # System SSH port
DISABLE_SSH      = false    # Allow SSH protocol
```

### System SSH Configuration

Location: `/etc/ssh/sshd_config`

No changes needed - Gitea works with standard SSH configuration.

## Advanced: SSH Config File

For convenience, add an SSH config entry:

Create/edit `~/.ssh/config`:

```
Host gitea
    HostName your-server.com
    User git
    Port 22
    IdentityFile ~/.ssh/id_ed25519

Host gitea-vpn
    HostName 10.0.0.1
    User git
    Port 22
    IdentityFile ~/.ssh/id_ed25519
```

Usage:
```bash
# Clone using SSH alias
git clone gitea:mleku/repo.git

# Or VPN alias
git clone gitea-vpn:mleku/repo.git
```

## Security Considerations

### SSH Key Best Practices

1. **Use ED25519 keys** (modern, secure, fast)
2. **Protect private keys** with passphrase
3. **Use different keys** for different servers
4. **Rotate keys** periodically
5. **Remove old keys** from Gitea when no longer needed

### Firewall Configuration

Ensure port 22 is accessible:

```bash
# UFW
sudo ufw allow 22/tcp

# iptables
sudo iptables -A INPUT -p tcp --dport 22 -j ACCEPT
```

### Restrict SSH Access

In `/etc/ssh/sshd_config`:

```
# Only allow specific users
AllowUsers mleku git

# Disable password authentication (keys only)
PasswordAuthentication no
PubkeyAuthentication yes

# Disable root login
PermitRootLogin no
```

Restart SSH:
```bash
sudo systemctl restart sshd
```

## Comparison: Built-in vs System SSH

| Feature | Built-in SSH (Port 2222) | System SSH (Port 22) |
|---------|--------------------------|----------------------|
| Port | 2222 | 22 |
| Configuration | Gitea-managed | System sshd |
| URLs | `git@host:2222/user/repo.git` | `git@host:user/repo.git` |
| Setup | Automatic | Requires configuration |
| Isolation | Separate from system | Shared with system |
| Management | Via Gitea | Via sshd config |

## Migration: 2222 → 22

If you already have repositories using port 2222, update them:

```bash
# Update single repository
cd /path/to/repo
git remote set-url origin git@your-server.com:mleku/repo.git

# Bulk update script
for repo in ~/Documents/github/*/; do
    cd "$repo"
    if git remote get-url origin | grep -q ":2222/"; then
        new_url=$(git remote get-url origin | sed 's/:2222\//:/g')
        git remote set-url origin "$new_url"
        echo "Updated: $(basename $repo)"
    fi
done
```

## Summary

Using system SSH with Gitea provides a cleaner, more standard Git experience:

✅ Standard SSH port (22)
✅ Standard Git URLs (`git@host:user/repo.git`)
✅ Centralized SSH management
✅ Works with existing SSH infrastructure
✅ No special port forwarding needed

The trade-off is slightly more complex initial setup compared to using Gitea's built-in SSH server.
