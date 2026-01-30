#!/bin/bash

# check-go-version.sh
# Verifies that the installed Go version matches the expected version.

set -e

EXPECTED_VERSION="${1:-1.23.0}"

# Get the installed Go version.
INSTALLED_VERSION=$(go version | awk '{print $3}' | sed 's/go//')

# Compare major.minor versions.
EXPECTED_MAJOR_MINOR=$(echo "$EXPECTED_VERSION" | cut -d. -f1,2)
INSTALLED_MAJOR_MINOR=$(echo "$INSTALLED_VERSION" | cut -d. -f1,2)

if [ "$EXPECTED_MAJOR_MINOR" != "$INSTALLED_MAJOR_MINOR" ]; then
    echo "ERROR: Go version mismatch!"
    echo "  Expected: go$EXPECTED_VERSION (major.minor: $EXPECTED_MAJOR_MINOR)"
    echo "  Installed: go$INSTALLED_VERSION (major.minor: $INSTALLED_MAJOR_MINOR)"
    exit 1
fi

echo "Go version check passed: go$INSTALLED_VERSION (expected $EXPECTED_VERSION)"
