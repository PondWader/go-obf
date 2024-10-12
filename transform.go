package main

import (
	"go/ast"
	"go/token"
)

type CodeTransform struct {
	content       string
	replaceOffset int

	fset *token.FileSet
	file *ast.File
}

func NewTransform(fset *token.FileSet, file *ast.File, content string) *CodeTransform {
	return &CodeTransform{
		fset:    fset,
		file:    file,
		content: content,
	}
}

func (t *CodeTransform) Walk(visit func(n ast.Node) bool) error {
	ast.Inspect(t.file, func(n ast.Node) bool {
		return visit(n)
	})

	return nil
}

func (t *CodeTransform) Replace(n ast.Node, v string) {
	start := t.fset.Position(n.Pos()).Offset + t.replaceOffset
	end := t.fset.Position(n.End()).Offset + t.replaceOffset

	t.content = t.content[:start] + v + t.content[end:]

	nodeLength := end - start
	t.replaceOffset += len(v) - nodeLength
}

func (t *CodeTransform) String() string {
	return t.content
}
