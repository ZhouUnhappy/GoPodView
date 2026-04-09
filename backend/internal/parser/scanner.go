package parser

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopodview/internal/model"
)

func ScanProject(root string) (*model.FileTreeNode, []string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, nil, err
	}

	rootNode := &model.FileTreeNode{
		Name:  filepath.Base(root),
		Path:  "",
		IsDir: true,
	}

	var goFiles []string
	err = buildTree(root, root, rootNode)
	if err != nil {
		return nil, nil, err
	}

	collectGoFiles(rootNode, &goFiles)
	return rootNode, goFiles, nil
}

func ScanExternalPackage(packageDir string) ([]string, error) {
	entries, err := os.ReadDir(packageDir)
	if err != nil {
		return nil, err
	}

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		files = append(files, filepath.Join(packageDir, name))
	}

	sort.Strings(files)
	return files, nil
}

func buildTree(absPath, root string, node *model.FileTreeNode) error {
	entries, err := os.ReadDir(absPath)
	if err != nil {
		return err
	}

	sort.Slice(entries, func(i, j int) bool {
		di, dj := entries[i].IsDir(), entries[j].IsDir()
		if di != dj {
			return di
		}
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		name := entry.Name()
		if shouldSkip(name) {
			continue
		}

		fullPath := filepath.Join(absPath, name)
		relPath, _ := filepath.Rel(root, fullPath)

		if entry.IsDir() {
			dirNode := &model.FileTreeNode{
				Name:  name,
				Path:  relPath,
				IsDir: true,
			}
			if err := buildTree(fullPath, root, dirNode); err != nil {
				continue
			}
			if hasGoFiles(dirNode) {
				node.Children = append(node.Children, dirNode)
			}
		} else if strings.HasSuffix(name, ".go") {
			node.Children = append(node.Children, &model.FileTreeNode{
				Name:  name,
				Path:  relPath,
				IsDir: false,
			})
		}
	}
	return nil
}

func shouldSkip(name string) bool {
	skipDirs := []string{"vendor", "node_modules", ".git", ".idea", ".vscode", "testdata"}
	for _, d := range skipDirs {
		if name == d {
			return true
		}
	}
	return strings.HasPrefix(name, ".")
}

func hasGoFiles(node *model.FileTreeNode) bool {
	if !node.IsDir {
		return strings.HasSuffix(node.Name, ".go")
	}
	for _, child := range node.Children {
		if hasGoFiles(child) {
			return true
		}
	}
	return false
}

func collectGoFiles(node *model.FileTreeNode, files *[]string) {
	if !node.IsDir {
		if strings.HasSuffix(node.Name, ".go") {
			*files = append(*files, node.Path)
		}
		return
	}
	for _, child := range node.Children {
		collectGoFiles(child, files)
	}
}
