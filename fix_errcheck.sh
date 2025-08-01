#!/bin/bash

# Script to systematically fix errcheck violations in the lift codebase
# Focuses on the most common patterns

echo "Fixing errcheck violations systematically..."

# 1. Fix app.GET, app.POST, etc. patterns in test files
find . -name "*_test.go" -exec sed -i '' 's/^\(\s*\)app\.GET(/\1if err := app.GET(/g' {} \;
find . -name "*_test.go" -exec sed -i '' 's/^\(\s*\)app\.POST(/\1if err := app.POST(/g' {} \;
find . -name "*_test.go" -exec sed -i '' 's/^\(\s*\)app\.SQS(/\1if err := app.SQS(/g' {} \;
find . -name "*_test.go" -exec sed -i '' 's/^\(\s*\)app\.S3(/\1if err := app.S3(/g' {} \;
find . -name "*_test.go" -exec sed -i '' 's/^\(\s*\)app\.EventBridge(/\1if err := app.EventBridge(/g' {} \;
find . -name "*_test.go" -exec sed -i '' 's/^\(\s*\)app\.Handle(/\1if err := app.Handle(/g' {} \;

echo "Fixed app method calls in test files..."

# 2. Fix os.Setenv and os.Unsetenv patterns in test files  
find . -name "*_test.go" -exec sed -i '' 's/^\(\s*\)os\.Setenv(/\1if err := os.Setenv(/g' {} \;
find . -name "*_test.go" -exec sed -i '' 's/^\(\s*\)os\.Unsetenv(/\1if err := os.Unsetenv(/g' {} \;

echo "Fixed os.Setenv and os.Unsetenv calls in test files..."

# 3. Fix Close() patterns
find . -name "*.go" -exec sed -i '' 's/^\(\s*\)defer \([^.]*\)\.Close()$/\1defer func() { _ = \2.Close() }()/g' {} \;

echo "Fixed Close() patterns..."

echo "Systematic fixes complete. You may need to manually add error handling for the 'if err :=' patterns."