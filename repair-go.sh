#!/bin/zsh
set -e

echo "🔍 Checking for broken Go toolchain..."
if [ -d "/Users/$USER/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.0.darwin-arm64" ]; then
  echo "🗑 Removing broken toolchain..."
  rm -rf "/Users/$USER/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.0.darwin-arm64"
else
  echo "✅ No broken toolchain found."
fi

echo "🔧 Reinstalling Go with Homebrew..."
brew reinstall go

echo "⚙️  Setting up environment variables..."
# Ensure exports exist only once
if ! grep -q "export GOROOT=" ~/.zshrc; then
  echo 'export GOROOT="$(brew --prefix go)/libexec"' >> ~/.zshrc
fi
if ! grep -q "export GOPATH=" ~/.zshrc; then
  echo 'export GOPATH="$HOME/go"' >> ~/.zshrc
fi
if ! grep -q "export PATH=.*$GOROOT/bin" ~/.zshrc; then
  echo 'export PATH="$PATH:$GOROOT/bin:$GOPATH/bin"' >> ~/.zshrc
fi
if ! grep -q "export GOTOOLCHAIN=" ~/.zshrc; then
  echo 'export GOTOOLCHAIN=local' >> ~/.zshrc
fi

# Load updated env
source ~/.zshrc

echo "✅ Go setup after reinstall:"
go version
which go
echo "GOROOT=$GOROOT"
echo "GOPATH=$GOPATH"

echo "📦 Running go mod tidy..."
go mod tidy

echo "🎉 All fixed! You’re good to GoFr 🚀"

