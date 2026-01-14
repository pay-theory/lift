package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func writeExecutable(t *testing.T, path string, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o755))
}

func TestMain_AllSuccess(t *testing.T) {
	tmp := t.TempDir()

	origWD, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Chdir(origWD) })
	require.NoError(t, os.Chdir(tmp))

	binDir := filepath.Join(tmp, "bin")
	fakeGo := filepath.Join(binDir, "go")

	writeExecutable(t, fakeGo, `#!/usr/bin/env bash
set -euo pipefail

cmd="${1:-}"
shift || true

if [[ "$cmd" == "version" ]]; then
  echo "go version go1.25.0 linux/amd64"
  exit 0
fi

if [[ "$cmd" == "list" && "${1:-}" == "-m" ]]; then
  dep="${2:-}"
  echo "${dep} v1.0.12"
  exit 0
fi

if [[ "$cmd" == "build" ]]; then
  out=""
  prev=""
  for arg in "$@"; do
    if [[ "$prev" == "-o" ]]; then
      out="$arg"
      break
    fi
    prev="$arg"
  done
  if [[ -z "$out" ]]; then
    echo "missing -o" >&2
    exit 1
  fi

  cat > "$out" <<'EOS'
#!/usr/bin/env bash
echo "✅ WithWebSocketSupport works!"
echo "✅ app.WebSocket() works!"
echo "✅ app.WebSocketHandler() works!"
EOS
  chmod 755 "$out"
  exit 0
fi

echo "unexpected command: $cmd" >&2
exit 1
`)

	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	main()

	_, err = os.Stat(filepath.Join(tmp, "test_websocket.go"))
	require.Error(t, err, "script should clean up temporary source file")
}

func TestMain_ErrorPaths(t *testing.T) {
	tmp := t.TempDir()

	origWD, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Chdir(origWD) })
	require.NoError(t, os.Chdir(tmp))

	binDir := filepath.Join(tmp, "bin")
	fakeGo := filepath.Join(binDir, "go")

	writeExecutable(t, fakeGo, `#!/usr/bin/env bash
set -euo pipefail

cmd="${1:-}"
shift || true

if [[ "$cmd" == "version" ]]; then
  echo "go is missing" >&2
  exit 1
fi

if [[ "$cmd" == "list" && "${1:-}" == "-m" ]]; then
  dep="${2:-}"
  if [[ "$dep" == "github.com/pay-theory/lift" ]]; then
    echo "github.com/pay-theory/lift v1.0.11"
    exit 0
  fi
  # Simulate missing dependency
  if [[ "$dep" == "github.com/aws/aws-lambda-go" ]]; then
    echo "missing dep" >&2
    exit 1
  fi
  echo "${dep} v0.0.0"
  exit 0
fi

if [[ "$cmd" == "build" ]]; then
  echo "build failed" >&2
  exit 1
fi

echo "unexpected command: $cmd" >&2
exit 1
`)

	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	main()
}
