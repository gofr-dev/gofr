#!/bin/zsh
set -e

echo "🔍 Checking Go installation..."
if ! command -v go >/dev/null 2>&1; then
  echo "❌ Go not found. Installing via Homebrew..."
  brew install go
fi

GO_VERSION=$(go version | awk '{print $3}')
echo "✅ Current Go version: $GO_VERSION"

echo "🔧 Ensuring Go 1.25.0 toolchain is installed..."
if ! command -v go1.25.0 >/dev/null 2>&1; then
  echo "⬇️ Installing Go 1.25.0..."
  go install golang.org/dl/go1.25.0@latest
  go1.25.0 download
else
  echo "✅ Go 1.25.0 already installed."
fi

# Make sure Go uses local toolchain by default
if ! grep -q "GOTOOLCHAIN=local" ~/.zshrc; then
  echo 'export GOTOOLCHAIN=local' >> ~/.zshrc
  echo "✅ Added GOTOOLCHAIN=local to ~/.zshrc"
fi
export GOTOOLCHAIN=local

echo "📦 Running go mod tidy..."
go mod tidy

echo "🎉 Done! You’re all set."

