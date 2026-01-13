package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type violation struct {
	path string
	line int
	rule string
}

var forbiddenSelectors = map[string]map[string]struct{}{
	"fmt": {
		"Print":   {},
		"Printf":  {},
		"Println": {},
	},
	"log": {
		"Print":   {},
		"Printf":  {},
		"Println": {},
		"Fatal":   {},
		"Fatalf":  {},
		"Fatalln": {},
		"Panic":   {},
		"Panicf":  {},
		"Panicln": {},
	},
}

func main() {
	fmt.Println("COM-6 logging/ops standards check (v0.1.0)")
	fmt.Println("- Scope: pkg/** and cmd/** Go files")
	fmt.Println("- Ignore: *_test.go")
	fmt.Println("- Forbidden: fmt.Print/Printf/Println, log.Print/Printf/Println/Fatal*/Panic*, builtin println")
	fmt.Println("")

	files, err := collectGoFiles([]string{"pkg", "cmd"})
	if err != nil {
		fmt.Fprintf(os.Stderr, "scan error: %v\n", err)
		os.Exit(2)
	}

	violations := scanFiles(files)
	if len(violations) == 0 {
		fmt.Printf("Checked %d files. No violations found.\n", len(files))
		return
	}

	sort.Slice(violations, func(i, j int) bool {
		if violations[i].path == violations[j].path {
			if violations[i].line == violations[j].line {
				return violations[i].rule < violations[j].rule
			}
			return violations[i].line < violations[j].line
		}
		return violations[i].path < violations[j].path
	})

	fmt.Printf("Checked %d files. Violations: %d\n", len(files), len(violations))
	for _, v := range violations {
		fmt.Printf("%s:%d: %s\n", v.path, v.line, v.rule)
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

func scanFiles(files []string) []violation {
	var violations []violation
	fileSet := token.NewFileSet()

	for _, path := range files {
		astFile, err := parser.ParseFile(fileSet, path, nil, 0)
		if err != nil {
			violations = append(violations, violation{path: path, line: 1, rule: fmt.Sprintf("parse error: %v", err)})
			continue
		}

		if importsLog(astFile) {
			line := fileSet.Position(astFile.Package).Line
			violations = append(violations, violation{path: path, line: line, rule: "import log"})
		}

		ast.Inspect(astFile, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			if ident, ok := call.Fun.(*ast.Ident); ok {
				if ident.Name == "println" {
					pos := fileSet.Position(call.Lparen)
					violations = append(violations, violation{path: path, line: pos.Line, rule: "builtin println"})
				}
				return true
			}

			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}

			pkgIdent, ok := selector.X.(*ast.Ident)
			if !ok {
				return true
			}

			forbidden, ok := forbiddenSelectors[pkgIdent.Name]
			if !ok {
				return true
			}

			if _, ok := forbidden[selector.Sel.Name]; ok {
				pos := fileSet.Position(selector.Sel.Pos())
				rule := fmt.Sprintf("%s.%s", pkgIdent.Name, selector.Sel.Name)
				violations = append(violations, violation{path: path, line: pos.Line, rule: rule})
			}

			return true
		})
	}

	return violations
}

func importsLog(file *ast.File) bool {
	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, "\"")
		if path == "log" {
			return true
		}
	}
	return false
}
