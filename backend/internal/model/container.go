package model

type ContainerType string

const (
	ContainerFunc      ContainerType = "func"
	ContainerStruct    ContainerType = "struct"
	ContainerInterface ContainerType = "interface"
	ContainerConst     ContainerType = "const"
	ContainerVar       ContainerType = "var"
)

type Container struct {
	Name       string        `json:"name"`
	Type       ContainerType `json:"type"`
	Pod        string        `json:"pod"`
	StartLine  int           `json:"startLine"`
	EndLine    int           `json:"endLine"`
	Signature  string        `json:"signature"`
	SourceCode string        `json:"sourceCode,omitempty"`
	References []*Reference  `json:"references"`
}

type ReferenceType string

const (
	RefCall    ReferenceType = "call"
	RefTypeRef ReferenceType = "type_ref"
	RefEmbed   ReferenceType = "embed"
)

type Reference struct {
	ContainerName string        `json:"containerName"`
	PodPath       string        `json:"podPath"`
	Type          ReferenceType `json:"type"`
}
