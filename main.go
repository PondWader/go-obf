package main

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"log"
	"os"
	"path/filepath"
	"unicode"

	"golang.org/x/mod/modfile"
)

// https://pkg.go.dev/go/ast#pkg-examples

//obf:protect
const EXAMPLE_SECRET = "abc"

var TYPE_IDENTIFIERS = []string{
	"bool",
	"uint", "u8", "u16", "u32", "u64", "uintptr",
	"int", "i8", "i16", "i32", "i64",
	"float32", "float64",
	"complex64", "complex128",
	"string", "struct", "byte", "rune",
	"map", "chan",
}

func main() {
	buildDir := os.Getenv("OBF_BUILD_DIR")
	if buildDir == "" {
		buildPath, err := os.MkdirTemp("", "go-obf-build")
		if err != nil {
			log.Fatalf("failed to create build directory: %s", err)
		}
		buildDir = buildPath
	} else {
		if err := os.MkdirAll(buildDir, 0600); err != nil {
			log.Fatalf("failed to create build directory: %s", err)
		}
	}

	excludedIdents := make(map[string]bool)
	for _, typeIdent := range TYPE_IDENTIFIERS {
		excludedIdents[typeIdent] = true
	}

	build := &ObfBuild{
		NameGen:           NewIdentGen(CHARSET_LOWERCASE),
		OutPath:           buildDir,
		Packages:          make(map[string]Package),
		ExcludedIdents:    excludedIdents,
		ProcessedPackages: make(map[string]bool),
	}

	build.patchModule()
	build.patchPackage(".")

	for path, pkg := range build.Packages {
		dirPath := filepath.Join(buildDir, path)
		if err := os.MkdirAll(dirPath, 0600); err != nil {
			log.Fatalf("failed to make dir %s: %s", dirPath, err)
		}

		fileNameGen := NewIdentGen(CHARSET_ALPHABET)

		for _, file := range pkg.Files {
			transform := NewTransform(file.Fset, file.Ast, file.Content)
			build.ApplyReplacements(transform, file.Replacements)
			fileName := filepath.Join(dirPath, fileNameGen.Next()+".go")
			err := os.WriteFile(fileName, []byte(transform.content), 0600)
			if err != nil {
				log.Fatalf("failed to write to %s: %s", fileName, err)
			}
		}
	}

	fmt.Println("Obfuscated in", buildDir)
}

func (build ObfBuild) ApplyReplacements(transform *CodeTransform, replacements []*ast.Ident) {
	publicIdentGen := NewIdentGen(CHARSET_LOWERCASE)
	privateIdentGen := NewIdentGen(CHARSET_UPPERCASE)

	identMap := make(map[string]string)

	for _, replacement := range replacements {
		name := replacement.Name

		if _, ok := build.ExcludedIdents[name]; ok {
			continue
		}

		replacementIdent, ok := identMap[name]
		if !ok {
			if unicode.IsUpper(rune(name[0])) {
				replacementIdent = privateIdentGen.Next()
			} else {
				replacementIdent = publicIdentGen.Next()
			}
			identMap[name] = replacementIdent
		}

		transform.Replace(replacement, replacementIdent)
	}
}

type File struct {
	Content      string
	Replacements []*ast.Ident
	Fset         *token.FileSet
	Ast          *ast.File
}

type Package struct {
	Name  string
	Files []File
}

type ObfBuild struct {
	BaseModule        string
	NameGen           IdentGen
	OutPath           string
	Packages          map[string]Package
	ExcludedIdents    map[string]bool
	ProcessedPackages map[string]bool
}

// Changes module name to "I"
func (build *ObfBuild) patchModule() {
	modData, err := os.ReadFile("go.mod")
	if errors.Is(err, os.ErrNotExist) {
		log.Fatal("could not find go.mod file")
	} else if err != nil {
		log.Fatalf("failed to read go.mod: %s", err)
	}

	file, err := modfile.Parse("go.mod", modData, nil)
	if err != nil {
		log.Fatalf("failed to parse go.mod: %s", err)
	}

	// Store original module name in build data
	build.BaseModule = file.Module.Mod.Path

	// Set module name to "I"
	file.AddModuleStmt("I")

	out, err := file.Format()
	if err != nil {
		log.Fatalf("failed to format go.mod: %s", err)
	}
	err = os.WriteFile(filepath.Join(build.OutPath, "go.mod"), out, 0600)
	if err != nil {
		log.Fatalf("failed to write go.mod: %s", err)
	}
}
