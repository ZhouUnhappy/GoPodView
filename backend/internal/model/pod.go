package model

type Pod struct {
	Path       string       `json:"path"`
	Package    string       `json:"package"`
	FileName   string       `json:"fileName"`
	Imports    []string     `json:"imports"`
	Containers []*Container `json:"containers"`
	DependsOn  []string     `json:"dependsOn"`
	DependedBy []string     `json:"dependedBy"`
	IsExternal bool         `json:"isExternal"`
	ModulePath string       `json:"modulePath,omitempty"`
}

type FileTreeNode struct {
	Name     string          `json:"name"`
	Path     string          `json:"path"`
	IsDir    bool            `json:"isDir"`
	Children []*FileTreeNode `json:"children,omitempty"`
}
