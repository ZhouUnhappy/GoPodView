package parser

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type ModuleInfo struct {
	ModuleName   string
	GoVersion    string
	Requirements []ModRequirement
	Replaces     map[string]ModReplace
	ModCachePath string

	requirementMap map[string]ModRequirement
}

type ModRequirement struct {
	Path     string
	Version  string
	Indirect bool
}

type ModReplace struct {
	Path      string
	Version   string
	LocalPath string
}

type ResolvedImport struct {
	ModulePath  string
	PackagePath string
	PackageDir  string
}

type goModEditJSON struct {
	Module  *struct{ Path string }
	Go      string
	Require []struct {
		Path     string
		Version  string
		Indirect bool
	}
	Replace []struct {
		Old struct {
			Path    string
			Version string
		}
		New struct {
			Path    string
			Version string
		}
	}
}

func ParseGoMod(projectRoot string) (*ModuleInfo, error) {
	goModPath := filepath.Join(projectRoot, "go.mod")
	if _, err := os.Stat(goModPath); err != nil {
		return nil, err
	}

	modJSON, err := runGoCommand(projectRoot, "mod", "edit", "-json")
	if err != nil {
		return nil, fmt.Errorf("parse go.mod: %w", err)
	}

	var parsed goModEditJSON
	if err := json.Unmarshal(modJSON, &parsed); err != nil {
		return nil, fmt.Errorf("decode go.mod json: %w", err)
	}

	modCache, err := resolveGoModCache(projectRoot)
	if err != nil {
		modCache = ""
	}

	info := &ModuleInfo{
		GoVersion:      parsed.Go,
		ModCachePath:   modCache,
		Replaces:       make(map[string]ModReplace),
		requirementMap: make(map[string]ModRequirement),
	}
	if parsed.Module != nil {
		info.ModuleName = parsed.Module.Path
	}

	for _, req := range parsed.Require {
		requirement := ModRequirement{
			Path:     req.Path,
			Version:  req.Version,
			Indirect: req.Indirect,
		}
		info.Requirements = append(info.Requirements, requirement)
		info.requirementMap[requirement.Path] = requirement
	}

	for _, rep := range parsed.Replace {
		replace := ModReplace{
			Path:    rep.New.Path,
			Version: rep.New.Version,
		}
		if rep.New.Path != "" && (strings.HasPrefix(rep.New.Path, ".") || filepath.IsAbs(rep.New.Path)) {
			localPath := rep.New.Path
			if !filepath.IsAbs(localPath) {
				localPath = filepath.Join(projectRoot, localPath)
			}
			replace.LocalPath = filepath.Clean(localPath)
			replace.Path = ""
			replace.Version = ""
		}
		info.Replaces[rep.Old.Path] = replace
	}

	return info, nil
}

func (m *ModuleInfo) ResolveModulePath(importPath string) (string, bool) {
	resolved, ok := m.ResolveImport(importPath)
	if !ok {
		return "", false
	}
	return resolved.PackageDir, true
}

func (m *ModuleInfo) ResolveImport(importPath string) (*ResolvedImport, bool) {
	if m == nil {
		return nil, false
	}

	modulePath := m.matchModulePath(importPath)
	if modulePath == "" {
		return nil, false
	}

	packageSuffix := strings.TrimPrefix(importPath, modulePath)
	packageSuffix = strings.TrimPrefix(packageSuffix, "/")

	baseDir, ok := m.resolveModuleDir(modulePath)
	if !ok {
		return nil, false
	}

	packageDir := baseDir
	if packageSuffix != "" {
		packageDir = filepath.Join(baseDir, filepath.FromSlash(packageSuffix))
	}

	info, err := os.Stat(packageDir)
	if err != nil || !info.IsDir() {
		return nil, false
	}

	return &ResolvedImport{
		ModulePath:  modulePath,
		PackagePath: importPath,
		PackageDir:  packageDir,
	}, true
}

func (m *ModuleInfo) matchModulePath(importPath string) string {
	longest := ""
	for modulePath := range m.requirementMap {
		if isImportWithinModule(importPath, modulePath) && len(modulePath) > len(longest) {
			longest = modulePath
		}
	}
	for modulePath := range m.Replaces {
		if isImportWithinModule(importPath, modulePath) && len(modulePath) > len(longest) {
			longest = modulePath
		}
	}
	return longest
}

func (m *ModuleInfo) resolveModuleDir(modulePath string) (string, bool) {
	if replace, ok := m.Replaces[modulePath]; ok {
		if replace.LocalPath != "" {
			return replace.LocalPath, true
		}

		targetPath := replace.Path
		targetVersion := replace.Version
		if targetPath == "" {
			targetPath = modulePath
		}
		if targetVersion == "" {
			if req, ok := m.requirementMap[modulePath]; ok {
				targetVersion = req.Version
			}
		}
		return moduleCacheDir(m.ModCachePath, targetPath, targetVersion)
	}

	req, ok := m.requirementMap[modulePath]
	if !ok {
		return "", false
	}

	return moduleCacheDir(m.ModCachePath, req.Path, req.Version)
}

func moduleCacheDir(modCachePath, modulePath, version string) (string, bool) {
	if modCachePath == "" || modulePath == "" || version == "" {
		return "", false
	}
	escapedPath := escapeModulePath(modulePath)
	escapedVersion := escapeModuleVersion(version)
	return filepath.Join(modCachePath, filepath.FromSlash(escapedPath+"@"+escapedVersion)), true
}

func resolveGoModCache(projectRoot string) (string, error) {
	if env := strings.TrimSpace(os.Getenv("GOMODCACHE")); env != "" {
		return env, nil
	}
	output, err := runGoCommand(projectRoot, "env", "GOMODCACHE")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func runGoCommand(projectRoot string, args ...string) ([]byte, error) {
	cmd := exec.Command("go", args...)
	cmd.Dir = projectRoot

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("%s", msg)
	}

	return stdout.Bytes(), nil
}

func escapeModulePath(value string) string {
	return escapeModuleCacheString(value)
}

func escapeModuleVersion(value string) string {
	return escapeModuleCacheString(value)
}

func escapeModuleCacheString(value string) string {
	var b strings.Builder
	b.Grow(len(value))
	for _, r := range value {
		if r >= 'A' && r <= 'Z' {
			b.WriteByte('!')
			b.WriteRune(r + ('a' - 'A'))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func isImportWithinModule(importPath, modulePath string) bool {
	return importPath == modulePath || strings.HasPrefix(importPath, modulePath+"/")
}
