# ORLY Relay Bootstrap Script

This directory contains a bootstrap script that automates the deployment of the ORLY relay.

## Quick Start

### One-Line Installation

Clone the repository and deploy the relay with a single command:

```bash
curl -sSL https://git.nostrdev.com/mleku/git.smesh.lol/orly/raw/branch/main/scripts/bootstrap.sh | bash
```

**Note:** This assumes the script is accessible at the raw URL path. Adjust the URL based on your git server's raw file URL format.

### Alternative: Download and Execute

If you prefer to review the script before running it:

```bash
# Download the script
curl -o bootstrap.sh https://git.nostrdev.com/mleku/git.smesh.lol/orly/raw/branch/main/scripts/bootstrap.sh

# Review the script
cat bootstrap.sh

# Make it executable and run
chmod +x bootstrap.sh
./bootstrap.sh
```

## What the Bootstrap Script Does

1. **Checks Prerequisites**
   - Verifies that `git` is installed on your system

2. **Clones or Updates Repository**
   - Clones the repository to `~/src/git.smesh.lol/orly` if it doesn't exist
   - If the repository already exists, pulls the latest changes from the main branch
   - Stashes any local changes before updating

3. **Runs Deployment**
   - Executes `scripts/deploy.sh` to:
     - Install Go if needed
     - Build the ORLY relay with embedded web UI
     - Install the binary to `~/.local/bin/orly`
     - Set up systemd service
     - Configure necessary capabilities

4. **Provides Next Steps**
   - Shows commands to start, check status, and view logs

## Post-Installation

After the bootstrap script completes, you can:

### Start the relay
```bash
sudo systemctl start orly
```

### Enable on boot
```bash
sudo systemctl enable orly
```

### Check status
```bash
sudo systemctl status orly
```

### View logs
```bash
sudo journalctl -u orly -f
```

### View relay identity
```bash
~/.local/bin/orly identity
```

## Configuration

The relay configuration is managed through environment variables. Edit the systemd service file to configure:

```bash
sudo systemctl edit orly
```

See the main README.md for available configuration options.

## Troubleshooting

### Git Not Found
```bash
# Ubuntu/Debian
sudo apt-get update && sudo apt-get install -y git

# Fedora/RHEL
sudo dnf install -y git

# Arch
sudo pacman -S git
```

### Permission Denied Errors

Make sure your user has sudo privileges for systemd service management.

### Port 443 Already in Use

If you're running TLS on port 443, make sure no other service is using that port:

```bash
sudo netstat -tlnp | grep :443
```

### Script Fails to Clone

If the repository URL is not accessible, you may need to:
- Check your network connection
- Verify the git server is accessible
- Use SSH URL instead (modify the script's `REPO_URL` variable)

## Manual Deployment

If you prefer to deploy manually without the bootstrap script:

```bash
# Clone repository
git clone https://git.nostrdev.com/mleku/git.smesh.lol/orly.git ~/src/git.smesh.lol/orly

# Enter directory
cd ~/src/git.smesh.lol/orly

# Run deployment
./scripts/deploy.sh
```

## Security Considerations

When running scripts from the internet:
1. Always review the script contents before execution
2. Use HTTPS URLs to prevent man-in-the-middle attacks
3. Verify the source is trustworthy
4. Consider using the "download and review" method instead of piping directly to bash

## Support

For issues or questions:
- Open an issue on the git repository
- Check the main README.md for detailed documentation
- Review logs with `sudo journalctl -u orly -f`
