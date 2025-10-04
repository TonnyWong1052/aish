# APT Repository Setup Guide

This guide explains how to set up the APT repository for distributing aish via `sudo apt install aish`.

## Overview

We use GitHub Pages to host a custom APT repository. The setup involves:

1. **Creating a separate repository** (`aish-apt-repo`) to host the APT repository
2. **Configuring GitHub Actions** to automatically update the repository when releases are published
3. **Setting up GPG signing** for package verification
4. **Configuring users' systems** to trust and use our repository

## Prerequisites

- GitHub CLI (`gh`) installed and authenticated
- GPG installed for package signing
- Access to the main aish repository

## Step 1: Create the APT Repository

Run the setup script from the main aish repository:

```bash
./scripts/setup-apt-repo.sh
```

This script will:
- Create a new `aish-apt-repo` repository
- Set up the basic APT repository structure
- Enable GitHub Pages for the repository
- Create initial documentation

## Step 2: Generate GPG Key for Package Signing

```bash
# Generate a new GPG key (if you don't have one)
gpg --full-generate-key

# Choose:
# - RSA and RSA (default)
# - 4096 bits
# - No expiration (or set as preferred)
# - Provide name and email (should match the maintainer info)

# List your GPG keys to get the key ID
gpg --list-secret-keys --keyid-format LONG

# Export the public key to the repository
gpg --armor --export YOUR_KEY_ID > KEY.gpg

# Add the public key to the apt repository
cd aish-apt-repo
git add KEY.gpg
git commit -m "Add GPG public key for package verification"
git push
```

## Step 3: Configure GitHub Secrets

In the main **aish repository**, add these secrets (Settings → Secrets and variables → Actions):

### Required Secrets

1. **`APT_REPO_TOKEN`**
   - Personal access token with `repo` scope
   - Used to push updates to the `aish-apt-repo` repository
   - Generate at: https://github.com/settings/tokens

2. **`GPG_PRIVATE_KEY`**
   - Your private GPG key in ASCII armor format
   - Export with: `gpg --armor --export-secret-keys YOUR_KEY_ID`
   - Copy the entire output including headers

3. **`GPG_PASSPHRASE`**
   - The passphrase for your GPG key
   - If you didn't set a passphrase, leave empty

4. **`GPG_KEY_ID`** (optional but recommended)
   - 你的 GPG 金鑰 ID（像是 `ABCDEF1234567890`）。
   - 供工作流在簽署 Release/InRelease 時以 `--local-user` 指定金鑰。

### Setting up the secrets:
```bash
# Export private key (copy this output to GPG_PRIVATE_KEY secret)
gpg --armor --export-secret-keys YOUR_KEY_ID

# Your GPG passphrase goes to GPG_PASSPHRASE secret

# (Optional) Set GPG_KEY_ID secret to YOUR_KEY_ID
```

## Step 4: Test the Release Process

1. **Create a test release** to verify the automation:
   ```bash
   git tag v0.0.2-test
   git push origin v0.0.2-test
   ```

2. **Monitor the GitHub Action** in the main repository:
   - Go to Actions tab
   - Watch the "Release and Update APT Repository" workflow
   - Check that both jobs complete successfully

3. **Verify the APT repository** was updated:
   - Visit `https://tonnywong1052.github.io/aish-apt-repo`
   - Check that the new .deb package appears in `pool/main/a/aish/`
   - Verify that `dists/stable/main/binary-amd64/Packages` was updated

## Step 5: Test Installation

Test the APT installation on a Debian/Ubuntu system:

```bash
# Add the repository GPG key
curl -fsSL https://tonnywong1052.github.io/aish-apt-repo/KEY.gpg | sudo gpg --dearmor -o /usr/share/keyrings/aish-archive-keyring.gpg

# Add the repository
echo "deb [signed-by=/usr/share/keyrings/aish-archive-keyring.gpg] https://tonnywong1052.github.io/aish-apt-repo stable main" | sudo tee /etc/apt/sources.list.d/aish.list

# Update package list
sudo apt update

# Install aish
sudo apt install aish

# Verify installation
aish --version
```

## Repository Structure

The APT repository follows this structure:

```
aish-apt-repo/
├── README.md                     # Repository documentation
├── index.html                    # GitHub Pages landing page
├── KEY.gpg                       # Public GPG key for verification
├── dists/
│   └── stable/
│       ├── Release               # Repository metadata
│       ├── Release.gpg           # GPG signature
│       ├── InRelease            # Signed Release file
│       └── main/
│           ├── binary-amd64/
│           │   ├── Packages      # Package index
│           │   └── Packages.gz   # Compressed index
│           └── binary-arm64/
│               ├── Packages
│               └── Packages.gz
└── pool/
    └── main/
        └── a/
            └── aish/
                ├── aish_0.0.1_amd64.deb
                ├── aish_0.0.1_arm64.deb
                └── ...           # Future releases
```

## Automation Workflow

When you push a new tag to the main repository:

1. **GoReleaser runs** and builds .deb and .rpm packages
2. **GitHub Action downloads** the .deb package
3. **APT repository is updated**:
   - .deb file is copied to `pool/main/a/aish/`
   - Package indexes are regenerated
   - Release file is updated and signed
4. **Changes are committed** to the `aish-apt-repo` repository
5. **GitHub Pages automatically** serves the updated repository

## Troubleshooting

### GPG Issues
- Ensure GPG key has no expiration or is not expired
- Verify the passphrase is correct
- Check that the private key export includes the full key

### Repository Access Issues
- Verify the `APT_REPO_TOKEN` has sufficient permissions
- Ensure the token is not expired
- Check that the repository name matches exactly

### Package Installation Issues
- Verify the GPG key was added correctly to the user's system
- Check that the repository URL is accessible
- Ensure the package architecture matches the system

### Testing Locally
```bash
# Download and inspect a package
wget https://tonnywong1052.github.io/aish-apt-repo/pool/main/a/aish/aish_VERSION_amd64.deb
dpkg-deb -I aish_VERSION_amd64.deb  # Show package info
dpkg-deb -c aish_VERSION_amd64.deb  # Show package contents
```

## Maintenance

### Adding Support for New Distributions

To support additional distributions (like `unstable`, `focal`, etc.):

1. Update the GitHub Action to create multiple distribution directories
2. Modify the repository structure in `dists/`
3. Update documentation with new distribution names

### Updating the GPG Key

If you need to update the GPG key:

1. Generate a new key following Step 2
2. Update the GitHub secrets
3. Add the new `KEY.gpg` to the repository
4. Users will need to re-import the new key

### Monitoring

- Check GitHub Actions regularly for failed releases
- Monitor repository size (GitHub has limits)
- Verify package integrity periodically

## Security Considerations

- Keep GPG private key secure and never commit it to repositories
- Use strong passphrases for GPG keys
- Regularly rotate access tokens
- Monitor repository access logs
- Consider key expiration for enhanced security

## Further Reading

- [Debian Repository Format](https://wiki.debian.org/DebianRepository/Format)
- [APT Repository Setup](https://help.ubuntu.com/community/Repositories/Personal)
- [GoReleaser NFPM Documentation](https://goreleaser.com/customization/nfpm/)
