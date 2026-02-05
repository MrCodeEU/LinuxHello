#!/bin/bash
# Test the GitHub workflow locally using act-cli

set -e

echo "Testing LinuxHello GitHub Workflow with act-cli"
echo "=============================================="

# Check if act is installed
if ! command -v act &> /dev/null; then
    echo "❌ act-cli not found. Install with:"
    echo "   curl https://raw.githubusercontent.com/nektos/act/master/install.sh | sudo bash"
    exit 1
fi

echo "Available workflows:"
act -l

echo ""
echo "Testing specific jobs..."

# Test linting job
echo "🔍 Testing lint job..."
act -j lint

# Test Fedora build (most relevant for RPM packaging)
echo "🏗️  Testing Fedora build job..."
act -j build-fedora

echo "✅ Workflow tests completed!"
echo ""
echo "To test the full release workflow:"
echo "  git tag v1.0.0-test"
echo "  act -j release"
echo "  git tag -d v1.0.0-test"