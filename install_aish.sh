#!/bin/bash
echo "Removing existing aish directory if it exists..."
rm -rf aish
echo "Cloning aish repository..."
git clone https://github.com/TonnyWong1052/aish.git
cd aish
echo "Building aish..."
go build -o aish ./cmd/aish
if [ $? -ne 0 ]; then
  echo "Go build failed. Trying to build main.go directly..."
  go build -o aish ./cmd/aish/main.go
fi
if [ ! -f "aish" ]; then
  echo "Build failed. Could not create the 'aish' binary."
  exit 1
fi
echo "Installing aish to $HOME/bin..."
mkdir -p $HOME/bin
mv aish $HOME/bin/
echo "Installation complete! Please make sure $HOME/bin is in your PATH."