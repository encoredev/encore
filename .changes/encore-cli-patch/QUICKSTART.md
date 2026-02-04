# Rencore Quick Start Guide

Get up and running with Rencore (custom Encore CLI) in 5 minutes.

## Prerequisites

- macOS or Linux
- Homebrew (for easiest installation)
- Or: curl + tar (for manual installation)

## Step 1: Install Rencore

### Option A: Homebrew (Recommended)

```bash
# Add the tap
brew tap stagecraft-ing/tap

# Install Rencore
brew install rencore

# If you have official Encore installed, switch to Rencore
brew unlink encore 2>/dev/null || true
brew link --overwrite rencore
```

### Option B: Manual Install

```bash
# Detect your platform
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

# Map architecture names
if [ "$ARCH" = "x86_64" ]; then
  ARCH="amd64"
elif [ "$ARCH" = "aarch64" ]; then
  ARCH="arm64"
fi

# Download latest release
VERSION=v1.44.7  # Check https://github.com/stagecraft-ing/encore/releases for latest
curl -Lo encore.tar.gz "https://github.com/stagecraft-ing/encore/releases/download/$VERSION/encore-$VERSION-${OS}_${ARCH}.tar.gz"

# Extract and install
tar -xzf encore.tar.gz
sudo mv encore /usr/local/bin/
rm encore.tar.gz
```

## Step 2: Verify Installation

```bash
encore version
```

You should see output like:
```
encore version v1.44.7
```

## Step 3: Authenticate

### Using OAuth (Interactive)

```bash
encore auth login
```

This will open your browser for authentication with your self-hosted platform.

### Using API Key (Headless/CI)

```bash
encore auth login-apikey --auth-key=your-api-key
```

Or via stdin:
```bash
echo "your-api-key" | encore auth login-apikey
```

## Step 4: Create Your First App

```bash
# Create a new app
encore app create my-app

# Choose a template (e.g., "hello-world")
# Enter when prompted

# Navigate to the app
cd my-app
```

## Step 5: Run Locally

```bash
encore run
```

Your app is now running! The CLI will show you:
- Local API URL (e.g., `http://localhost:4000`)
- Local dashboard URL
- Cloud dashboard URL (pointing to your self-hosted platform)

## Step 6: Deploy

```bash
encore deploy --env production
```

The CLI will show you the deployment URL on your self-hosted platform.

## Next Steps

### Explore the Dashboard

Visit your cloud dashboard URL (shown after deployment) to:
- View traces and logs
- Monitor metrics
- Manage environments
- Configure secrets

### Add Features

```bash
# Generate a new service
encore gen service my-service

# Generate a new API endpoint
encore gen endpoint my-service my-endpoint
```

### Run Tests

```bash
encore test
```

### Generate Client Libraries

```bash
encore gen client my-app --lang=typescript --output=./client
```

## Configuration

Rencore is pre-configured with your self-hosted platform URLs:

- **Platform API**: https://api.stagecraft.ing
- **Web Dashboard**: https://app.stagecraft.ing
- **Dev Dashboard**: https://devdash.stagecraft.ing
- **Documentation**: https://docs.stagecraft.ing

These can be overridden with environment variables if needed:

```bash
export ENCORE_PLATFORM_API_URL="https://api.custom.com"
export ENCORE_WEBDASH_URL="https://app.custom.com"
encore run
```

## Switching Between Official and Custom Encore

### Use Rencore (Custom Build)

```bash
brew unlink encore 2>/dev/null || true
brew link --overwrite rencore
encore version  # Should show Rencore version
```

### Use Official Encore

```bash
brew unlink rencore
brew link --overwrite encore
encore version  # Should show official version
```

## Common Commands

| Command | Description |
|---------|-------------|
| `encore run` | Run app locally |
| `encore test` | Run tests |
| `encore deploy` | Deploy to cloud |
| `encore app create` | Create new app |
| `encore gen service <name>` | Generate service |
| `encore gen endpoint <svc> <name>` | Generate endpoint |
| `encore gen client <app>` | Generate client library |
| `encore logs` | View deployment logs |
| `encore secret set` | Set secret value |
| `encore db shell` | Open database shell |
| `encore version` | Show version |

## Troubleshooting

### Command not found: encore

```bash
# Check if installed
which encore

# If using Homebrew and it's not found
brew link --overwrite rencore

# Add to PATH if manual install
export PATH="/usr/local/bin:$PATH"
```

### Authentication Issues

```bash
# Clear existing auth
rm -rf ~/.config/encore/.auth_token

# Try logging in again
encore auth login
```

### Port Already in Use

```bash
# Run on a different port
encore run --port 4001
```

### Can't Connect to Platform

```bash
# Verify URLs
encore version  # Shows configured URLs

# Check environment variables
env | grep ENCORE

# Test connectivity
curl -v https://api.stagecraft.ing/health
```

## Getting Help

- **Documentation**: https://docs.stagecraft.ing
- **Issues**: https://github.com/stagecraft-ing/encore/issues
- **Discussions**: https://github.com/stagecraft-ing/encore/discussions
- **Official Docs**: https://encore.dev/docs (for general Encore help)

## What's Next?

- [Full Documentation](RENCORE.md)
- [Homebrew Tap Setup](HOMEBREW_TAP_README.md)
- [Contributing Guide](CONTRIBUTING.md)
- [Release Process](RELEASES.md)

---

🎉 **Congratulations!** You're now up and running with Rencore.

Happy coding! 🚀
