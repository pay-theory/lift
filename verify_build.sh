#!/bin/bash

echo "Verifying Lift build..."
echo

# Build all main packages
echo "Building main packages..."
if go build ./pkg/... 2>&1; then
    echo "✅ Main packages build successfully"
else
    echo "❌ Main packages build failed"
    exit 1
fi

echo
echo "Building CDK packages..."
if go build ./pkg/cdk/constructs ./pkg/cdk/patterns ./pkg/cdk/stacks 2>&1; then
    echo "✅ CDK packages build successfully"
else
    echo "❌ CDK packages build failed"
    exit 1
fi

echo
echo "Building compliance package..."
if go build ./pkg/compliance 2>&1; then
    echo "✅ Compliance package builds successfully"
else
    echo "❌ Compliance package build failed"
    exit 1
fi

echo
echo "All packages compiled successfully! 🎉"