package parser

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"strings"

	"gopodview/internal/model"
)

type ProjectParser struct {
	Root       string
	fset       *token.FileSet
	Pods       map[string]*model.Pod
	srcMap     map[string][]byte
	sourcePath map[string]string
}

func NewProjectParser(root string) *ProjectParser {
	return &ProjectParser{
		Root:       root,
		fset:       token.NewFileSet(),
		Pods:       make(map[string]*model.Pod),
		srcMap:     make(map[string][]byte),
		sourcePath: make(map[string]string),
	}
}

func (p *ProjectParser) ParseFile(relPath string) (*model.Pod, error) {
	absPath := filepath.Join(p.Root, relPath)
	return p.parseFile(absPath, relPath, false, "")
}

func (p *ProjectParser) ParseExternalFile(absPath, displayPath, modulePath string) (*model.Pod, error) {
	displayPath = path.Clean(displayPath)
	return p.parseFile(absPath, displayPath, true, modulePath)
}

func (p *ProjectParser) SourceForPod(podPath string) ([]byte, string, bool) {
	src, ok := p.srcMap[podPath]
	if !ok {
		return nil, "", false
	}
	return src, p.sourcePath[podPath], true
}

func (p *ProjectParser) parseFile(absPath, displayPath string, isExternal bool, modulePath string) (*model.Pod, error) {
	src, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}
	p.srcMap[displayPath] = src
	p.sourcePath[displayPath] = absPath

	f, err := parser.ParseFile(p.fset, absPath, src, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	pod := &model.Pod{
		Path:       displayPath,
		Package:    f.Name.Name,
		FileName:   filepath.Base(absPath),
		IsExternal: isExternal,
		ModulePath: modulePath,
	}

	for _, imp := range f.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)
		pod.Imports = append(pod.Imports, importPath)
	}

	pod.Containers = p.extractContainers(f, displayPath, src)
	p.Pods[displayPath] = pod
	return pod, nil
}

func (p *ProjectParser) extractContainers(f *ast.File, podPath string, src []byte) []*model.Container {
	var containers []*model.Container

	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			containers = append(containers, p.parseFuncDecl(d, podPath, src))
		case *ast.GenDecl:
			containers = append(containers, p.parseGenDecl(d, podPath, src)...)
		}
	}
	return containers
}

func (p *ProjectParser) parseFuncDecl(d *ast.FuncDecl, podPath string, src []byte) *model.Container {
	startPos := p.fset.Position(d.Pos())
	endPos := p.fset.Position(d.End())

	name := d.Name.Name
	if d.Recv != nil && len(d.Recv.List) > 0 {
		recvType := exprToString(d.Recv.List[0].Type)
		name = recvType + "." + d.Name.Name
	}

	sig := p.funcSignature(d)
	sourceCode := string(src[startPos.Offset:endPos.Offset])

	return &model.Container{
		Name:       name,
		Type:       model.ContainerFunc,
		Pod:        podPath,
		StartLine:  startPos.Line,
		EndLine:    endPos.Line,
		Signature:  sig,
		SourceCode: sourceCode,
	}
}

func (p *ProjectParser) funcSignature(d *ast.FuncDecl) string {
	var buf bytes.Buffer
	buf.WriteString("func ")
	if d.Recv != nil && len(d.Recv.List) > 0 {
		buf.WriteString("(")
		printer.Fprint(&buf, p.fset, d.Recv.List[0].Type)
		buf.WriteString(") ")
	}
	buf.WriteString(d.Name.Name)
	printer.Fprint(&buf, p.fset, d.Type)
	return buf.String()
}

func (p *ProjectParser) parseGenDecl(d *ast.GenDecl, podPath string, src []byte) []*model.Container {
	var containers []*model.Container

	switch d.Tok {
	case token.TYPE:
		singleSpec := !d.Lparen.IsValid() && len(d.Specs) == 1
		for _, spec := range d.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			ct := containerTypeFromTypeSpec(ts)

			var startPos, endPos token.Position
			if singleSpec {
				startPos = p.fset.Position(d.Pos())
				endPos = p.fset.Position(d.End())
			} else {
				startPos = p.fset.Position(ts.Pos())
				endPos = p.fset.Position(ts.End())
			}

			sourceCode := string(src[startPos.Offset:endPos.Offset])
			if !singleSpec {
				sourceCode = "type " + sourceCode
			}

			containers = append(containers, &model.Container{
				Name:       ts.Name.Name,
				Type:       ct,
				Pod:        podPath,
				StartLine:  startPos.Line,
				EndLine:    endPos.Line,
				Signature:  keywordFor(ct) + " " + ts.Name.Name,
				SourceCode: sourceCode,
			})
		}

	case token.CONST, token.VAR:
		ct := model.ContainerConst
		if d.Tok == token.VAR {
			ct = model.ContainerVar
		}

		if d.Lparen.IsValid() {
			startPos := p.fset.Position(d.Pos())
			endPos := p.fset.Position(d.End())
			var names []string
			for _, spec := range d.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, n := range vs.Names {
					names = append(names, n.Name)
				}
			}
			groupName := strings.Join(names, ", ")
			if len(groupName) > 60 {
				groupName = groupName[:57] + "..."
			}

			containers = append(containers, &model.Container{
				Name:       groupName,
				Type:       ct,
				Pod:        podPath,
				StartLine:  startPos.Line,
				EndLine:    endPos.Line,
				Signature:  d.Tok.String() + " (" + groupName + ")",
				SourceCode: string(src[startPos.Offset:endPos.Offset]),
			})
		} else {
			for _, spec := range d.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, n := range vs.Names {
					startPos := p.fset.Position(vs.Pos())
					endPos := p.fset.Position(vs.End())
					containers = append(containers, &model.Container{
						Name:       n.Name,
						Type:       ct,
						Pod:        podPath,
						StartLine:  startPos.Line,
						EndLine:    endPos.Line,
						Signature:  d.Tok.String() + " " + n.Name,
						SourceCode: string(src[startPos.Offset:endPos.Offset]),
					})
				}
			}
		}
	}
	return containers
}

func containerTypeFromTypeSpec(ts *ast.TypeSpec) model.ContainerType {
	switch ts.Type.(type) {
	case *ast.InterfaceType:
		return model.ContainerInterface
	case *ast.StructType:
		return model.ContainerStruct
	default:
		return model.ContainerStruct
	}
}

func keywordFor(ct model.ContainerType) string {
	switch ct {
	case model.ContainerFunc:
		return "func"
	case model.ContainerStruct:
		return "type struct"
	case model.ContainerInterface:
		return "type interface"
	case model.ContainerConst:
		return "const"
	case model.ContainerVar:
		return "var"
	default:
		return ""
	}
}

func exprToString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return "*" + exprToString(e.X)
	case *ast.SelectorExpr:
		return exprToString(e.X) + "." + e.Sel.Name
	case *ast.IndexExpr:
		return exprToString(e.X)
	case *ast.IndexListExpr:
		return exprToString(e.X)
	default:
		return ""
	}
}
