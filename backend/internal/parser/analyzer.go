package parser

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"

	"gopodview/internal/model"
)

type Analyzer struct {
	parser     *ProjectParser
	importMap  map[string]string // import path -> relative file dir (package level)
	pkgToPods  map[string][]string
}

func NewAnalyzer(pp *ProjectParser) *Analyzer {
	return &Analyzer{
		parser:    pp,
		importMap: make(map[string]string),
		pkgToPods: make(map[string][]string),
	}
}

func (a *Analyzer) AnalyzeAll(goFiles []string) error {
	for _, relPath := range goFiles {
		if _, err := a.parser.ParseFile(relPath); err != nil {
			continue
		}
	}

	a.buildPackageIndex()
	a.buildPodDependencies()
	a.buildContainerReferences(goFiles)

	return nil
}

func (a *Analyzer) buildPackageIndex() {
	for relPath, pod := range a.parser.Pods {
		dir := filepath.Dir(relPath)
		a.pkgToPods[dir] = append(a.pkgToPods[dir], relPath)

		modImportPath := a.dirToImportPath(dir)
		if modImportPath != "" {
			a.importMap[modImportPath] = dir
		}

		_ = pod
	}
}

func (a *Analyzer) dirToImportPath(dir string) string {
	return dir
}

func (a *Analyzer) buildPodDependencies() {
	for relPath, pod := range a.parser.Pods {
		for _, imp := range pod.Imports {
			if isStdLib(imp) || isExternal(imp, a.parser.Root) {
				continue
			}

			depDir := a.resolveImport(imp, relPath)
			if depDir == "" {
				continue
			}

			depPods := a.pkgToPods[depDir]
			for _, depPath := range depPods {
				pod.DependsOn = appendUnique(pod.DependsOn, depPath)

				if depPod, ok := a.parser.Pods[depPath]; ok {
					depPod.DependedBy = appendUnique(depPod.DependedBy, relPath)
				}
			}
		}
	}
}

func (a *Analyzer) resolveImport(importPath string, fromFile string) string {
	if dir, ok := a.importMap[importPath]; ok {
		return dir
	}

	parts := strings.Split(importPath, "/")
	for i := len(parts); i >= 1; i-- {
		candidate := strings.Join(parts[len(parts)-i:], "/")
		if _, ok := a.pkgToPods[candidate]; ok {
			a.importMap[importPath] = candidate
			return candidate
		}
	}

	return ""
}

func (a *Analyzer) buildContainerReferences(goFiles []string) {
	containerIndex := make(map[string]map[string]*model.Container)
	for relPath, pod := range a.parser.Pods {
		m := make(map[string]*model.Container)
		for _, c := range pod.Containers {
			baseName := c.Name
			if idx := strings.LastIndex(baseName, "."); idx >= 0 {
				baseName = baseName[idx+1:]
			}
			m[baseName] = c
		}
		containerIndex[relPath] = m
	}

	for _, relPath := range goFiles {
		absPath := filepath.Join(a.parser.Root, relPath)
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, absPath, nil, 0)
		if err != nil {
			continue
		}

		pod := a.parser.Pods[relPath]
		if pod == nil {
			continue
		}

		importAliases := buildImportAliases(f)

		for _, container := range pod.Containers {
			refs := a.findReferences(f, fset, container, relPath, importAliases, containerIndex)
			container.References = refs
		}
	}
}

func buildImportAliases(f *ast.File) map[string]string {
	aliases := make(map[string]string)
	for _, imp := range f.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)
		var alias string
		if imp.Name != nil {
			alias = imp.Name.Name
		} else {
			parts := strings.Split(importPath, "/")
			alias = parts[len(parts)-1]
		}
		aliases[alias] = importPath
	}
	return aliases
}

func (a *Analyzer) findReferences(
	f *ast.File,
	fset *token.FileSet,
	container *model.Container,
	podPath string,
	importAliases map[string]string,
	containerIndex map[string]map[string]*model.Container,
) []*model.Reference {
	var refs []*model.Reference
	seen := make(map[string]bool)

	ast.Inspect(f, func(n ast.Node) bool {
		if n == nil {
			return true
		}

		pos := fset.Position(n.Pos())
		if pos.Line < container.StartLine || pos.Line > container.EndLine {
			return true
		}

		switch expr := n.(type) {
		case *ast.SelectorExpr:
			ident, ok := expr.X.(*ast.Ident)
			if !ok {
				return true
			}

			pkgAlias := ident.Name
			selName := expr.Sel.Name
			importPath, ok := importAliases[pkgAlias]
			if !ok {
				return true
			}

			depDir := a.resolveImport(importPath, podPath)
			if depDir == "" {
				return true
			}

			for depPod, containers := range containerIndex {
				if filepath.Dir(depPod) != depDir {
					continue
				}
				if target, ok := containers[selName]; ok {
					key := depPod + "#" + target.Name
					if !seen[key] {
						seen[key] = true
						refType := model.RefCall
						if target.Type == model.ContainerStruct || target.Type == model.ContainerInterface {
							refType = model.RefTypeRef
						}
						refs = append(refs, &model.Reference{
							ContainerName: target.Name,
							PodPath:       depPod,
							Type:          refType,
						})
					}
				}
			}
		}
		return true
	})

	return refs
}

func isStdLib(importPath string) bool {
	return !strings.Contains(importPath, ".")
}

func isExternal(importPath string, root string) bool {
	_ = root
	return false
}

func appendUnique(slice []string, val string) []string {
	for _, s := range slice {
		if s == val {
			return slice
		}
	}
	return append(slice, val)
}
