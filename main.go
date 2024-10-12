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
	for _, builtin := range builtins {
		excludedIdents[builtin] = true
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

	remapper := Remapper{
		Build:           build,
		PublicIdentGen:  NewIdentGen(CHARSET_UPPERCASE),
		PrivateIdentGen: NewIdentGen(CHARSET_LOWERCASE),
		identMap:        make(map[string]string),
	}

	for path, pkg := range build.Packages {
		dirPath := filepath.Join(buildDir, path)
		if err := os.MkdirAll(dirPath, 0600); err != nil {
			log.Fatalf("failed to make dir %s: %s", dirPath, err)
		}

		fileNameGen := NewIdentGen(CHARSET_ALPHABET)

		for _, file := range pkg.Files {
			transform := NewTransform(file.Fset, file.Ast, file.Content)
			remapper.ApplyReplacements(transform, file.Replacements)
			fileName := filepath.Join(dirPath, fileNameGen.Next()+".go")
			err := os.WriteFile(fileName, []byte(transform.content), 0600)
			if err != nil {
				log.Fatalf("failed to write to %s: %s", fileName, err)
			}
		}
	}

	fmt.Println("Obfuscated in", buildDir)
}

type Remapper struct {
	Build           *ObfBuild
	PublicIdentGen  IdentGen
	PrivateIdentGen IdentGen

	identMap map[string]string
}

func (remapper *Remapper) ApplyReplacements(transform *CodeTransform, replacements []*ast.Ident) {
	build := remapper.Build

	for _, replacement := range replacements {
		name := replacement.Name

		if _, ok := build.ExcludedIdents[name]; ok {
			continue
		}

		replacementIdent, ok := remapper.identMap[name]
		if !ok {
			if unicode.IsUpper(rune(name[0])) {
				replacementIdent = remapper.PublicIdentGen.Next()
			} else {
				replacementIdent = remapper.PrivateIdentGen.Next()
			}
			remapper.identMap[name] = replacementIdent
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
