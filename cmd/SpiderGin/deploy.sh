#!/bin/bash

# Check if the "-kill" parameter is passed
if [ "$1" = "-kill" ]; then
  # Kill the existing SpiderGin process if it's running
  if pgrep SpiderGin >/dev/null 2>&1; then
    echo "Stopping existing SpiderGin process..."
    pkill -f SpiderGin
  else
    echo "SpiderGin process is not running"
  fi
  exit 0
fi

# Check if SpiderGin process is already running, and end the process
if pgrep SpiderGin >/dev/null 2>&1; then
  echo "Stopping existing SpiderGin process..."
  pkill -f SpiderGin
fi

# Update code repository
echo "Updating repository..."
git reset --hard HEAD
git pull

# Build the binary
echo "Building binary..."
export GOPROXY=https://goproxy.cn,direct
go build -o SpiderGin cmd/SpiderGin/main.go

chmod +x ./cmd/SpiderGin/deploy.sh

# Start SpiderGin and log output to SpiderGin.log
nohup ./SpiderGin > SpiderGin.log 2>&1 &

# View logs
tail -f SpiderGin.log
