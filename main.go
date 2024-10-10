package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"golang.org/x/mod/modfile"
)

// https://pkg.go.dev/go/ast#pkg-examples

//obf:protect
const EXAMPLE_SECRET = "abc"

func main() {
	gen := &IdentGen{}
	for i := 0; i < 52*52+500; i++ {
		fmt.Println(gen.Next())
	}
	os.Exit(0)

	buildPath, err := os.MkdirTemp("", "go-obf-build")
	if err != nil {
		log.Fatalf("Failed to create build directory: %s", err)
	}

	build := &ObfBuild{
		OutPath: buildPath,
	}

	build.patchModule()

	fmt.Println("Obfuscated in", buildPath)
}

type ObfBuild struct {
	OutPath string
}

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
