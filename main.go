package main

import (
	"errors"
	"fmt"
	"go/ast"
	"log"
	"os"
	"path/filepath"

	"golang.org/x/mod/modfile"
)

// https://pkg.go.dev/go/ast#pkg-examples

//obf:protect
const EXAMPLE_SECRET = "abc"

func main() {
	buildPath, err := os.MkdirTemp("", "go-obf-build")
	if err != nil {
		log.Fatalf("Failed to create build directory: %s", err)
	}

	build := &ObfBuild{
		NameGen:           NewIdentGen(CHARSET_LOWERCASE),
		OutPath:           buildPath,
		Packages:          make(map[string]Package),
		ExcludedIdents:    make(map[string]bool),
		ProcessedPackages: make(map[string]bool),
	}

	build.patchModule()
	build.patchPackage(".")

	fmt.Println("Obfuscated in", buildPath)
}

type Package struct {
	Name         string
	Replacements []*ast.Ident
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
