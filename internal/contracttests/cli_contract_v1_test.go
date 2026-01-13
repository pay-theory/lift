package contracttests

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type templateSpec struct {
	name string
}

func TestContractDocV1(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	path := filepath.Join(repoRoot, "docs", "cli-contract-v1.md")
	content := readFile(t, path)

	requiredSnippets := []string{"version: 1", "`dev`", "`staging`", "`live`"}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(content, snippet) {
			t.Fatalf("contract doc missing required snippet %q in %s", snippet, path)
		}
	}
}

func TestTemplateContractV1(t *testing.T) {
	repoRoot := mustRepoRoot(t)
	templates := []templateSpec{
		{name: "basic-api"},
		{name: "microservice"},
		{name: "event-driven"},
		{name: "merchant-app"},
		{name: "sns-processor"},
	}

	variants := []string{"", "-pt"}
	for _, tmpl := range templates {
		for _, variant := range variants {
			variantName := tmpl.name + variant
			path := filepath.Join(repoRoot, "internal", "templates", variantName, "lift.yaml.tmpl")
			lines := readLines(t, path)
			assertTemplateInvariant(t, path, lines, tmpl.name)
		}
	}
}

func assertTemplateInvariant(t *testing.T, path string, lines []string, templateName string) {
	t.Helper()

	checks := []struct {
		name  string
		found bool
	}{
		{name: "version: 1"},
		{name: "app:"},
		{name: fmt.Sprintf("template: %s", templateName)},
		{name: "stages:"},
		{name: "dev:"},
		{name: "staging:"},
		{name: "live:"},
		{name: "domains:"},
		{name: "base_domain:"},
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		for i := range checks {
			if checks[i].name == trimmed || (checks[i].name == "base_domain:" && strings.HasPrefix(trimmed, "base_domain:")) {
				checks[i].found = true
			}
		}
	}

	for _, check := range checks {
		if !check.found {
			t.Fatalf("template %s missing required line %q", path, check.name)
		}
	}
}

func mustRepoRoot(t *testing.T) string {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	current := wd
	for {
		if _, err := os.Stat(filepath.Join(current, "go.mod")); err == nil {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}

	t.Fatalf("repo root not found from %s", wd)
	return ""
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	// #nosec G304 -- path is derived from repo root with fixed filenames.
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file %s: %v", path, err)
	}

	return string(content)
}

func readLines(t *testing.T, path string) []string {
	t.Helper()

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open file %s: %v", path, err)
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan file %s: %v", path, err)
	}

	return lines
}
