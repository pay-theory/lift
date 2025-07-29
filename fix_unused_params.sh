#!/bin/bash

# Fix unused parameters in test functions
find pkg -name "*.go" -exec sed -i '' 's/func.*\(t \*testing\.T\) {/func.*(_ *testing.T) {/g' {} \;

# Fix unused context parameters
find pkg -name "*.go" -exec sed -i '' 's/(ctx context\.Context,/(_ context.Context,/g' {} \;
find pkg -name "*.go" -exec sed -i '' 's/(ctx context\.Context)/(_ context.Context)/g' {} \;

# Fix unused http.Request parameters
find pkg -name "*.go" -exec sed -i '' 's/, r \*http\.Request/, _ *http.Request/g' {} \;

echo "Fixed common unused parameter patterns"