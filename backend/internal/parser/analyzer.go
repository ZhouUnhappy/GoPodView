package parser

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"gopodview/internal/model"
)

type Analyzer struct {
	parser    *ProjectParser
	modInfo   *ModuleInfo
	importMap map[string]string // import path -> package key
	pkgToPods map[string][]string
}

func NewAnalyzer(pp *ProjectParser) *Analyzer {
	return &Analyzer{
		parser:    pp,
		importMap: make(map[string]string),
		pkgToPods: make(map[string][]string),
	}
}

func (a *Analyzer) AnalyzeAll(goFiles []string) error {
	if modInfo, err := ParseGoMod(a.parser.Root); err == nil {
		a.modInfo = modInfo
	}

	for _, relPath := range goFiles {
		if _, err := a.parser.ParseFile(relPath); err != nil {
			continue
		}
	}

	a.buildPackageIndex()
	a.buildPodDependencies()
	a.buildContainerReferences()

	return nil
}

func (a *Analyzer) buildPackageIndex() {
	a.importMap = make(map[string]string)
	a.pkgToPods = make(map[string][]string)

	for relPath := range a.parser.Pods {
		pod := a.parser.Pods[relPath]
		packageKey := a.podPackageKey(pod)
		if packageKey == "" {
			continue
		}
		a.pkgToPods[packageKey] = append(a.pkgToPods[packageKey], relPath)
		a.importMap[packageKey] = packageKey
	}

	for pkg := range a.pkgToPods {
		sort.Strings(a.pkgToPods[pkg])
	}
}

func (a *Analyzer) dirToImportPath(dir string) string {
	dir = filepath.ToSlash(dir)
	if a.modInfo == nil || a.modInfo.ModuleName == "" {
		return dir
	}
	if dir == "." || dir == "" {
		return a.modInfo.ModuleName
	}
	return a.modInfo.ModuleName + "/" + strings.TrimPrefix(dir, "./")
}

func (a *Analyzer) buildPodDependencies() {
	for relPath, pod := range a.parser.Pods {
		if pod.IsExternal {
			continue
		}

		for _, imp := range pod.Imports {
			if isStdLib(imp) || a.isExternal(imp) {
				continue
			}

			depPkg := a.resolveImport(imp, relPath)
			if depPkg == "" {
				continue
			}

			depPods := a.pkgToPods[depPkg]
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

	if a.modInfo != nil && a.modInfo.ModuleName != "" && isImportWithinModule(importPath, a.modInfo.ModuleName) {
		relPath := strings.TrimPrefix(importPath, a.modInfo.ModuleName)
		relPath = strings.TrimPrefix(relPath, "/")
		if relPath == "" {
			relPath = "."
		}
		candidate := a.dirToImportPath(relPath)
		if _, ok := a.pkgToPods[candidate]; ok {
			a.importMap[importPath] = candidate
			return candidate
		}
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

func (a *Analyzer) buildContainerReferences() {
	containerIndex := a.buildContainerIndex()
	for relPath := range a.parser.Pods {
		a.refreshPodContainerReferences(relPath, containerIndex)
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

			depPkg := a.resolveImport(importPath, podPath)
			if depPkg == "" {
				return true
			}

			for _, depPod := range a.pkgToPods[depPkg] {
				containers := containerIndex[depPod]
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

func (a *Analyzer) podPackageKey(pod *model.Pod) string {
	if pod == nil {
		return ""
	}
	if pod.IsExternal {
		return path.Dir(pod.Path)
	}
	return a.dirToImportPath(filepath.Dir(pod.Path))
}

func isStdLib(importPath string) bool {
	return !strings.Contains(importPath, ".")
}

func (a *Analyzer) isExternal(importPath string) bool {
	if a.modInfo == nil || a.modInfo.ModuleName == "" {
		return false
	}
	return !isImportWithinModule(importPath, a.modInfo.ModuleName)
}

func (a *Analyzer) LoadExternalDependenciesForContainer(podPath, containerName string) (*model.Container, []*model.Pod, error) {
	pod := a.parser.Pods[podPath]
	if pod == nil {
		return nil, nil, fmt.Errorf("pod not found: %s", podPath)
	}

	container := findContainerByName(pod, containerName)
	if container == nil {
		return nil, nil, fmt.Errorf("container not found: %s", containerName)
	}

	if pod.IsExternal || a.modInfo == nil {
		return container, nil, nil
	}

	importPaths, err := a.collectExternalImportsForContainer(podPath, container)
	if err != nil {
		return nil, nil, err
	}

	if len(importPaths) == 0 {
		a.refreshPodContainerReferences(podPath, a.buildContainerIndex())
		return findContainerByName(a.parser.Pods[podPath], containerName), nil, nil
	}

	for _, importPath := range importPaths {
		if err := a.ensureExternalPackageLoaded(importPath); err != nil {
			continue
		}
	}

	a.buildPackageIndex()
	a.refreshPodExternalEdges(podPath, importPaths)
	externalPods := a.externalPodsForImports(importPaths)
	containerIndex := a.buildContainerIndex()
	a.refreshPodContainerReferences(podPath, containerIndex)
	for _, pod := range externalPods {
		a.refreshPodContainerReferences(pod.Path, containerIndex)
	}

	return findContainerByName(a.parser.Pods[podPath], containerName), externalPods, nil
}

func (a *Analyzer) buildContainerIndex() map[string]map[string]*model.Container {
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
	return containerIndex
}

func (a *Analyzer) refreshPodContainerReferences(podPath string, containerIndex map[string]map[string]*model.Container) {
	src, absPath, ok := a.parser.SourceForPod(podPath)
	if !ok {
		return
	}

	pod := a.parser.Pods[podPath]
	if pod == nil {
		return
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, absPath, src, 0)
	if err != nil {
		return
	}

	importAliases := buildImportAliases(f)
	for _, container := range pod.Containers {
		container.References = a.findReferences(f, fset, container, podPath, importAliases, containerIndex)
	}
}

func (a *Analyzer) collectExternalImportsForContainer(podPath string, container *model.Container) ([]string, error) {
	src, absPath, ok := a.parser.SourceForPod(podPath)
	if !ok {
		return nil, nil
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, absPath, src, 0)
	if err != nil {
		return nil, err
	}

	importAliases := buildImportAliases(f)
	importSet := make(map[string]struct{})

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

			importPath, ok := importAliases[ident.Name]
			if !ok || !a.isExternal(importPath) {
				return true
			}

			importSet[importPath] = struct{}{}
		}

		return true
	})

	result := make([]string, 0, len(importSet))
	for importPath := range importSet {
		result = append(result, importPath)
	}
	sort.Strings(result)
	return result, nil
}

func (a *Analyzer) ensureExternalPackageLoaded(importPath string) error {
	resolved, ok := a.modInfo.ResolveImport(importPath)
	if !ok {
		return fmt.Errorf("external import not resolved: %s", importPath)
	}

	files, err := ScanExternalPackage(resolved.PackageDir)
	if err != nil {
		return err
	}

	for _, absPath := range files {
		displayPath := path.Join(importPath, filepath.Base(absPath))
		if _, exists := a.parser.Pods[displayPath]; exists {
			continue
		}
		if _, err := a.parser.ParseExternalFile(absPath, displayPath, resolved.ModulePath); err != nil {
			continue
		}
	}

	return nil
}

func (a *Analyzer) refreshPodExternalEdges(podPath string, importPaths []string) {
	pod := a.parser.Pods[podPath]
	if pod == nil {
		return
	}

	for _, importPath := range importPaths {
		depPkg := a.resolveImport(importPath, podPath)
		if depPkg == "" {
			continue
		}

		for _, depPath := range a.pkgToPods[depPkg] {
			depPod := a.parser.Pods[depPath]
			if depPod == nil || !depPod.IsExternal {
				continue
			}

			pod.DependsOn = appendUnique(pod.DependsOn, depPath)
			depPod.DependedBy = appendUnique(depPod.DependedBy, podPath)
		}
	}
}

func (a *Analyzer) externalPodsForImports(importPaths []string) []*model.Pod {
	seen := make(map[string]bool)
	pods := make([]*model.Pod, 0)

	for _, importPath := range importPaths {
		depPkg := a.resolveImport(importPath, "")
		if depPkg == "" {
			continue
		}

		for _, depPath := range a.pkgToPods[depPkg] {
			if seen[depPath] {
				continue
			}

			pod := a.parser.Pods[depPath]
			if pod == nil || !pod.IsExternal {
				continue
			}

			seen[depPath] = true
			pods = append(pods, pod)
		}
	}

	sort.Slice(pods, func(i, j int) bool {
		return pods[i].Path < pods[j].Path
	})
	return pods
}

func findContainerByName(pod *model.Pod, containerName string) *model.Container {
	if pod == nil {
		return nil
	}
	for _, container := range pod.Containers {
		if container.Name == containerName {
			return container
		}
	}
	return nil
}

func appendUnique(slice []string, val string) []string {
	for _, s := range slice {
		if s == val {
			return slice
		}
	}
	return append(slice, val)
}
