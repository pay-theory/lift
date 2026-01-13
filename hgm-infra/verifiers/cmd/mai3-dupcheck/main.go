package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"go/scanner"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type dupGroup struct {
	hash  string
	paths []string
}

func main() {
	fmt.Println("MAI-3 duplication policy (v0.1.0 implementation)")
	fmt.Println("- Scope: pkg/** and cmd/** Go files")
	fmt.Println("- Ignore: *_test.go")
	fmt.Println("- Method: hash normalized token stream (comments/whitespace removed)")
	fmt.Println("- Output: stable sha256 fingerprint + sorted file list")
	fmt.Println("")

	files, err := collectGoFiles([]string{"pkg", "cmd"})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Scan error: %v\n", err)
		os.Exit(1)
	}

	if len(files) == 0 {
		fmt.Println("No in-scope files found.")
		os.Exit(0)
	}

	hashes := make(map[string][]string)
	for _, path := range files {
		normalized, err := normalizedTokens(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Normalize error for %s: %v\n", path, err)
			os.Exit(1)
		}
		sum := sha256.Sum256([]byte(normalized))
		hash := hex.EncodeToString(sum[:])
		hashes[hash] = append(hashes[hash], path)
	}

	var groups []dupGroup
	for hash, paths := range hashes {
		if len(paths) > 1 {
			sort.Strings(paths)
			groups = append(groups, dupGroup{hash: hash, paths: paths})
		}
	}

	sort.Slice(groups, func(i, j int) bool { return groups[i].hash < groups[j].hash })

	if len(groups) == 0 {
		fmt.Printf("Checked %d files. No duplicates found.\n", len(files))
		os.Exit(0)
	}

	fmt.Printf("Checked %d files. Duplicate groups: %d\n", len(files), len(groups))
	for _, group := range groups {
		fmt.Printf("\nDuplicate group: %s\n", group.hash)
		for _, path := range group.paths {
			fmt.Printf("- %s\n", path)
		}
	}

	os.Exit(1)
}

func collectGoFiles(roots []string) ([]string, error) {
	var files []string
	for _, root := range roots {
		info, err := os.Stat(root)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		if !info.IsDir() {
			continue
		}
		err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if filepath.Ext(path) != ".go" {
				return nil
			}
			if strings.HasSuffix(path, "_test.go") {
				return nil
			}
			files = append(files, path)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(files)
	return files, nil
}

func normalizedTokens(path string) (string, error) {
	// #nosec G304 -- path is derived from fixed WalkDir roots (pkg/, cmd/), not user input.
	src, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	fileSet := token.NewFileSet()
	file := fileSet.AddFile(path, fileSet.Base(), len(src))

	var scanErrs []string
	var s scanner.Scanner
	s.Init(file, src, func(pos token.Position, msg string) {
		scanErrs = append(scanErrs, fmt.Sprintf("%s: %s", pos, msg))
	}, scanner.ScanComments)

	var b strings.Builder
	for {
		_, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		if tok == token.COMMENT || tok == token.SEMICOLON {
			continue
		}
		b.WriteString(tok.String())
		if lit != "" {
			b.WriteString(lit)
		}
	}

	if len(scanErrs) > 0 {
		return "", errors.New(strings.Join(scanErrs, "; "))
	}

	return b.String(), nil
}
