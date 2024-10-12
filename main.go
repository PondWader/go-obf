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

	"github.com/PondWader/go-obf/pkg"
	"golang.org/x/mod/modfile"
)

func main() {
	pkg.Cow()

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

	// Start by excluding special names
	excludedIdents := map[string]bool{"main": true, "_": true}
	// Exclude builtins
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

	build.copyGoSum()
	build.patchModule()
	build.patchPackage(".")

	remapper := Remapper{
		Build:           build,
		PublicIdentGen:  NewIdentGen(CHARSET_UPPERCASE),
		PrivateIdentGen: NewIdentGen(CHARSET_LOWERCASE),
		identMap:        make(map[string]string),
	}

	pkgNameGen := NewIdentGen(CHARSET_LOWERCASE)
	pkgNames := make(map[string]string)

	for path, pkg := range build.Packages {
		var pkgName string
		var pkgPath string
		if path == "." {
			pkgName = "main"
			pkgPath = "."
		} else {
			pkgName = pkgNameGen.Next()
			pkgPath = pkgName
			pkgNames[path] = pkgName
		}

		dirPath := filepath.Join(buildDir, pkgPath)
		if err := os.MkdirAll(dirPath, 0600); err != nil {
			log.Fatalf("failed to make dir %s: %s", dirPath, err)
		}

		fileNameGen := NewIdentGen(CHARSET_ALPHABET)

		for _, file := range pkg.Files {
			transform := NewTransform(file.Fset, file.Ast, file.Content)
			transform.Replace(file.Ast.Name, pkgName)
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
		genLoop:
			for {
				if unicode.IsUpper(rune(name[0])) {
					replacementIdent = remapper.PublicIdentGen.Next()
				} else {
					replacementIdent = remapper.PrivateIdentGen.Next()
				}

				// Check that the identifier isn't reserved and if it is, skip it
				if _, ok := remapper.Build.ExcludedIdents[replacementIdent]; !ok {
					remapper.identMap[name] = replacementIdent
					break genLoop
				}
			}
		}

		transform.Replace(replacement, replacementIdent)
	}
}

//obf:preserve-fields
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
	BaseModuleImports []*ast.ImportSpec
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

func (build *ObfBuild) copyGoSum() {
	data, err := os.ReadFile("go.sum")
	if err != nil {
		log.Fatalf("Failed to read go.sum: %s", err)
	}
	err = os.WriteFile(filepath.Join(build.OutPath, "go.sum"), data, 0600)
	if err != nil {
		log.Fatalf("Failed to write go.sum: %s", err)
	}
}
