#!/bin/bash

# Release script for aish v0.0.3
# This script will prepare and publish the new release with GPG signing improvements

set -e

echo "🚀 Starting release process for aish v0.0.3..."

# Check if we're on main branch
CURRENT_BRANCH=$(git branch --show-current)
if [ "$CURRENT_BRANCH" != "main" ]; then
    echo "❌ Please switch to main branch before releasing"
    exit 1
fi

# Check if working directory is clean
if [ -n "$(git status --porcelain)" ]; then
    echo "❌ Working directory is not clean. Please commit or stash changes first."
    exit 1
fi

echo "✅ Working directory is clean"

# Add all changes
echo "📝 Adding changes to git..."
git add .
git add .github/workflows/release.yml
git add docs/GPG_SIGNING_IMPROVEMENTS.md

# Commit changes
echo "💾 Committing changes..."
git commit -m "feat: enhance GPG signing verification for APT repository release

- Add comprehensive GPG key health checks
- Implement secure GPG setup with temporary directories
- Add DEB file integrity verification
- Separate DEB files by architecture (amd64/arm64)
- Enhanced APT repository validation
- Improve error handling and logging
- Optimize permissions and dependency versions
- Add detailed documentation for GPG improvements

This addresses security and reliability issues in the release workflow
and ensures proper GPG signing for APT packages."

# Create and push tag
echo "🏷️ Creating and pushing tag v0.0.3..."
git tag -a v0.0.3 -m "Release v0.0.3: Enhanced GPG Signing and APT Repository

🔐 GPG Signing Improvements:
- Comprehensive GPG key health checks and validation
- Secure GPG setup with temporary directories
- Enhanced signature verification and testing

📦 APT Repository Enhancements:
- Architecture-specific package handling (amd64/arm64)
- DEB file integrity verification
- Improved repository validation and error handling

🔒 Security & Reliability:
- Optimized permissions following least privilege principle
- Fixed dependency versions for stability
- Enhanced error handling and detailed logging

📚 Documentation:
- Added comprehensive GPG signing documentation
- Detailed setup and troubleshooting guides

This release significantly improves the reliability and security
of the automated APT repository publishing workflow."

# Push to GitHub
echo "📤 Pushing to GitHub..."
git push origin main
git push origin v0.0.3

echo "✅ Release v0.0.3 has been prepared and pushed to GitHub!"
echo ""
echo "🎯 Next steps:"
echo "1. The GitHub Action will automatically trigger and build the release"
echo "2. Monitor the action at: https://github.com/TonnyWong1052/aish/actions"
echo "3. Check the release at: https://github.com/TonnyWong1052/aish/releases/tag/v0.0.3"
echo "4. Verify the APT repository is updated at: https://tonnywong1052.github.io/aish-apt-repo"
echo ""
echo "🔍 To monitor the release process:"
echo "   - Watch the GitHub Actions workflow"
echo "   - Check for any GPG signing issues"
echo "   - Verify the APT repository is properly updated"
echo ""
echo "🎉 Release preparation complete!"