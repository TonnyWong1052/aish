#!/bin/bash

# Script to set up APT repository on GitHub Pages
# This script should be run once to create the aish-apt-repo repository

set -e

REPO_NAME="aish-apt-repo"
GITHUB_USER="TonnyWong1052"
GPG_KEY_ID="${GPG_KEY_ID:-}"

echo "🚀 Setting up APT repository for aish..."

# Check if gh CLI is installed
if ! command -v gh &> /dev/null; then
    echo "❌ GitHub CLI (gh) is required but not installed."
    echo "Please install it from: https://cli.github.com/"
    exit 1
fi

# Check if user is logged in to GitHub CLI
if ! gh auth status &> /dev/null; then
    echo "❌ Please login to GitHub CLI first:"
    echo "gh auth login"
    exit 1
fi

# Create the repository
echo "📦 Creating repository ${GITHUB_USER}/${REPO_NAME}..."
gh repo create "${REPO_NAME}" --public --description "APT repository for aish - AI shell helper" || echo "Repository might already exist, continuing..."

# Clone the repository
echo "📥 Cloning repository..."
if [ -d "${REPO_NAME}" ]; then
    echo "Directory ${REPO_NAME} already exists, removing..."
    rm -rf "${REPO_NAME}"
fi

gh repo clone "${GITHUB_USER}/${REPO_NAME}"
cd "${REPO_NAME}"

# Create initial repository structure
echo "📁 Creating APT repository structure..."
mkdir -p pool/main/a/aish
mkdir -p dists/stable/main/binary-amd64
mkdir -p dists/stable/main/binary-arm64

# Create initial empty Packages files
touch dists/stable/main/binary-amd64/Packages
touch dists/stable/main/binary-arm64/Packages

# Create initial Release file
cat > dists/stable/Release << EOF
Origin: aish
Label: aish
Suite: stable
Codename: stable
Version: 1.0
Architectures: amd64 arm64
Components: main
Description: aish APT repository
Date: $(date -Ru)
EOF

# Create README
cat > README.md << 'EOF'
# aish APT Repository

This repository contains Debian packages for [aish](https://github.com/TonnyWong1052/aish) - an AI shell helper.

## Usage

Add this repository to your system:

```bash
# Add the repository GPG key
curl -fsSL https://tonnywong1052.github.io/aish-apt-repo/KEY.gpg | sudo gpg --dearmor -o /usr/share/keyrings/aish-archive-keyring.gpg

# Add the repository
echo "deb [signed-by=/usr/share/keyrings/aish-archive-keyring.gpg] https://tonnywong1052.github.io/aish-apt-repo stable main" | sudo tee /etc/apt/sources.list.d/aish.list

# Update package list
sudo apt update

# Install aish
sudo apt install aish
```

## Repository Structure

- `pool/` - Contains the actual .deb packages
- `dists/` - Contains repository metadata and package indexes
- `KEY.gpg` - Public GPG key for package verification

This repository is automatically updated when new releases of aish are published.
EOF

# Create index.html for GitHub Pages
cat > index.html << 'EOF'
<!DOCTYPE html>
<html>
<head>
    <title>aish APT Repository</title>
    <meta charset="utf-8">
    <style>
        body { font-family: Arial, sans-serif; max-width: 800px; margin: 50px auto; padding: 20px; }
        code { background: #f4f4f4; padding: 2px 4px; border-radius: 3px; }
        pre { background: #f4f4f4; padding: 15px; border-radius: 5px; overflow-x: auto; }
    </style>
</head>
<body>
    <h1>aish APT Repository</h1>
    <p>This is the APT repository for <strong>aish</strong> - an AI shell helper.</p>
    
    <h2>Installation</h2>
    <p>Add this repository to your system and install aish:</p>
    
    <pre><code># Add the repository GPG key
curl -fsSL https://tonnywong1052.github.io/aish-apt-repo/KEY.gpg | sudo gpg --dearmor -o /usr/share/keyrings/aish-archive-keyring.gpg

# Add the repository
echo "deb [signed-by=/usr/share/keyrings/aish-archive-keyring.gpg] https://tonnywong1052.github.io/aish-apt-repo stable main" | sudo tee /etc/apt/sources.list.d/aish.list

# Update package list
sudo apt update

# Install aish
sudo apt install aish</code></pre>

    <h2>Links</h2>
    <ul>
        <li><a href="https://github.com/TonnyWong1052/aish">aish on GitHub</a></li>
        <li><a href="https://github.com/TonnyWong1052/aish-apt-repo">Repository Source</a></li>
    </ul>
</body>
</html>
EOF

# Commit initial structure
echo "💾 Committing initial repository structure..."
git add .
git config user.name "aish-bot"
git config user.email "releases@aish.local"
git commit -m "Initial APT repository structure"
git push origin main

# Enable GitHub Pages
echo "🌐 Enabling GitHub Pages..."
gh api repos/"${GITHUB_USER}"/"${REPO_NAME}"/pages \
  --method POST \
  --field source='{"branch":"main","path":"/"}' \
  || echo "GitHub Pages might already be enabled"

echo ""
echo "✅ APT repository setup complete!"
echo ""
echo "📋 Next steps:"
echo "1. Generate a GPG key for package signing:"
echo "   gpg --full-generate-key"
echo ""
echo "2. Export the public key:"
echo "   gpg --armor --export YOUR_KEY_ID > KEY.gpg"
echo "   git add KEY.gpg && git commit -m 'Add GPG public key' && git push"
echo ""
echo "3. Add the following secrets to your main aish repository:"
echo "   - APT_REPO_TOKEN: Personal access token with repo access"
echo "   - GPG_PRIVATE_KEY: Your private GPG key (gpg --armor --export-secret-keys YOUR_KEY_ID)"
echo "   - GPG_PASSPHRASE: Your GPG key passphrase"
echo ""
echo "4. The repository will be available at:"
echo "   https://${GITHUB_USER}.github.io/${REPO_NAME}"
echo ""

cd ..
echo "Repository created in: $(pwd)/${REPO_NAME}"