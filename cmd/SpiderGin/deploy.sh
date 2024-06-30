#!/bin/bash

# Check if the "-kill" parameter is passed
if [ "$1" = "-kill" ]; then
  # Kill the existing SpiderGin_backend process if it's running
  if pgrep SpiderGin_backend >/dev/null 2>&1; then
    echo "Stopping existing SpiderGin_backend process..."
    pkill -f SpiderGin_backend
  else
    echo "SpiderGin_backend process is not running"
  fi
  exit 0
fi

# Check if SpiderGin_backend process is already running, and end the process
if pgrep SpiderGin_backend >/dev/null 2>&1; then
  echo "Stopping existing SpiderGin_backend process..."
  pkill -f SpiderGin_backend
fi

# Update code repository
echo "Updating repository..."
git reset --hard HEAD
git pull

# Build the binary
echo "Building binary..."
export GOPROXY=https://goproxy.cn,direct
go build -o SpiderGin_backend main.go

chmod +x deploy.sh

# Start SpiderGin_backend and log output to SpiderGin_backend.log
nohup ./SpiderGin_backend > SpiderGin_backend.log 2>&1 &

# View logs
tail -f SpiderGin_backend.log
