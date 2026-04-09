package parser

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestAnalyzeAllBuildsInternalDependenciesAtFileGranularity(t *testing.T) {
	root := t.TempDir()

	writeTestFile(t, filepath.Join(root, "go.mod"), "module example.com/demo\n\ngo 1.21.0\n")
	writeTestFile(t, filepath.Join(root, "main", "use.go"), `package mainpkg

import "example.com/demo/lib"

func Use() {
	lib.Foo()
}
`)
	writeTestFile(t, filepath.Join(root, "lib", "a.go"), `package lib

func Foo() {}
`)
	writeTestFile(t, filepath.Join(root, "lib", "b.go"), `package lib

func Bar() {}
`)

	_, goFiles, err := ScanProject(root)
	if err != nil {
		t.Fatalf("scan project: %v", err)
	}

	pp := NewProjectParser(root)
	analyzer := NewAnalyzer(pp)
	if err := analyzer.AnalyzeAll(goFiles); err != nil {
		t.Fatalf("analyze project: %v", err)
	}

	usePod := pp.Pods["main/use.go"]
	if usePod == nil {
		t.Fatalf("main/use.go pod not found")
	}

	if len(usePod.DependsOn) != 1 || usePod.DependsOn[0] != "lib/a.go" {
		t.Fatalf("unexpected dependencies for main/use.go: %#v", usePod.DependsOn)
	}

	if slices.Contains(usePod.DependsOn, "lib/b.go") {
		t.Fatalf("main/use.go should not depend on lib/b.go: %#v", usePod.DependsOn)
	}

	if !slices.Contains(pp.Pods["lib/a.go"].DependedBy, "main/use.go") {
		t.Fatalf("lib/a.go should be depended by main/use.go: %#v", pp.Pods["lib/a.go"].DependedBy)
	}

	if slices.Contains(pp.Pods["lib/b.go"].DependedBy, "main/use.go") {
		t.Fatalf("lib/b.go should not be depended by main/use.go: %#v", pp.Pods["lib/b.go"].DependedBy)
	}
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
