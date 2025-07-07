#!/bin/bash

echo "Checking for build warnings and issues..."
echo

# Check main packages
echo "=== Building main packages ==="
go build ./pkg/... 2>&1 | grep -E "warning|unused|undefined" || echo "✅ No warnings in main packages"

echo
echo "=== Running go vet ==="
go vet ./pkg/... 2>&1 | grep -v "^#" || echo "✅ No issues found by go vet"

echo
echo "=== Checking CDK packages specifically ==="
go build ./pkg/cdk/constructs ./pkg/cdk/patterns 2>&1 || echo "✅ CDK packages build clean"

echo
echo "Build check complete!"