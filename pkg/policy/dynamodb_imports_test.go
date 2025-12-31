package policy

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNoRawDynamoDBSDKInRuntime(t *testing.T) {
	t.Parallel()

	root := repoRootFromTestDir(t)
	pkgDir := filepath.Join(root, "pkg")

	// Direct DynamoDB SDK usage should be centralized in DynamORM (or infra tooling).
	// This prevents drift into multiple ad-hoc DynamoDB implementations across runtime packages.
	bannedImports := map[string]struct{}{
		"github.com/aws/aws-sdk-go-v2/service/dynamodb":                {},
		"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue": {},
		"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression":     {},
	}

	ignoredPrefixes := []string{
		filepath.Join(pkgDir, "cdk") + string(os.PathSeparator),
		filepath.Join(pkgDir, "cli") + string(os.PathSeparator),
		filepath.Join(pkgDir, "performance") + string(os.PathSeparator),
	}

	var offenders []string
	err := filepath.WalkDir(pkgDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if d.IsDir() {
			// Skip ignored roots quickly.
			for _, prefix := range ignoredPrefixes {
				if strings.HasPrefix(path+string(os.PathSeparator), prefix) {
					return filepath.SkipDir
				}
			}
			return nil
		}

		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		fileSet := token.NewFileSet()
		parsed, parseErr := parser.ParseFile(fileSet, path, nil, parser.ImportsOnly)
		if parseErr != nil {
			return parseErr
		}

		for _, imp := range parsed.Imports {
			importPath := strings.Trim(imp.Path.Value, "\"")
			if _, banned := bannedImports[importPath]; banned {
				offenders = append(offenders, path+" -> "+importPath)
			}
		}

		return nil
	})

	require.NoError(t, err)
	require.Empty(t, offenders, "found banned DynamoDB SDK imports in runtime packages:\n%s", strings.Join(offenders, "\n"))
}

func repoRootFromTestDir(t *testing.T) string {
	t.Helper()

	wd, err := os.Getwd()
	require.NoError(t, err)

	// This test lives at "<repo>/pkg/policy"; root is two levels up.
	root := filepath.Clean(filepath.Join(wd, "..", ".."))
	_, statErr := os.Stat(filepath.Join(root, "go.mod"))
	require.NoError(t, statErr, "could not locate repo root from %q", wd)

	return root
}
