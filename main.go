package main

import (
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/token"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"unicode"

	"golang.org/x/mod/modfile"
)

func main() {
	output := flag.String("o", "", "Path of output file")
	goFlags := flag.String("go-flags", "", "Flags to pass to go")
	flag.Parse()
	if *output == "" {
		log.Fatalf("expected -o flag with output name")
	}

	args := flag.Args()

	if len(args) == 0 {
		log.Fatalf("expected build path as lastargument")
	}
	pkgPath := os.Args[len(os.Args)-1]

	buildDir := os.Getenv("OBF_BUILD_DIR")
	if buildDir == "" {
		buildPath, err := os.MkdirTemp("", "go-obf-build")
		if err != nil {
			log.Fatalf("failed to create build directory: %s", err)
		}
		buildDir = buildPath
	} else {
		if err := os.MkdirAll(buildDir, 0770); err != nil {
			log.Fatalf("failed to create build directory: %s", err)
		}
	}
	defer func() {
		os.RemoveAll(buildDir)
	}()

	// Start by excluding special names
	excludedIdents := map[string]bool{"main": true, "_": true}
	// Exclude builtins
	for _, builtin := range builtins {
		excludedIdents[builtin] = true
	}

	build := &ObfBuild{
		NameGen:           NewIdentGen(CHARSET_LOWERCASE),
		OutPath:           buildDir,
		Packages:          make([]Package, 0),
		ExcludedIdents:    excludedIdents,
		ProcessedPackages: make(map[string]bool),
	}

	build.copyGoSum()
	build.patchModule()
	build.patchPackage(filepath.Join(".", pkgPath))

	remapper := Remapper{
		Build:           build,
		PublicIdentGen:  NewIdentGen(CHARSET_UPPERCASE),
		PrivateIdentGen: NewIdentGen(CHARSET_LOWERCASE),
		identMap:        make(map[string]string),
	}

	pkgNames := make(map[string]string)

	for _, pkg := range build.Packages {
		path := pkg.Pattern

		var pkgName string
		var pkgPath string
		if path == "." {
			pkgName = "main"
			pkgPath = "."
		} else {
			pkgName = remapper.GetReplcement(pkg.Name)
			pkgPath = pkgName
			pkgNames[path] = pkgName
		}

		dirPath := filepath.Join(buildDir, pkgPath)
		if err := os.MkdirAll(dirPath, 0770); err != nil {
			log.Fatalf("failed to make dir %s: %s", dirPath, err)
		}

		fileNameGen := NewIdentGen(CHARSET_ALPHABET)

		for _, file := range pkg.Files {
			transform := NewTransform(file.Fset, file.Ast, file.Content)
			transform.Replace(file.Ast.Name, pkgName)

			// Replace imports of package within the base module with obfuscated names
			for _, importSpec := range file.BaseModuleImports {
				// Remove quotation marks around import path
				importPath := importSpec.Path.Value[1 : len(importSpec.Path.Value)-1]
				newPkgName, ok := pkgNames[importPath]
				if !ok {
					continue
				}
				newPath := "I/" + newPkgName
				transform.Replace(importSpec.Path, `"`+newPath+`"`)
			}

			remapper.ApplyReplacements(transform, file.Replacements)
			fileName := filepath.Join(dirPath, fileNameGen.Next()+".go")
			content := []byte(transform.content + file.AppendContent)
			err := os.WriteFile(fileName, content, 0660)
			if err != nil {
				log.Fatalf("failed to write to %s: %s", fileName, err)
			}
		}

		for name, data := range pkg.Embeds {
			if err := os.WriteFile(filepath.Join(dirPath, name), data, 0660); err != nil {
				log.Fatalf("failed to write embed file %s", name)
			}
		}
	}

	fmt.Println("Obfuscated in", buildDir)
	fmt.Println("Building...")

	outFile, err := filepath.Abs(*output)
	if err != nil {
		log.Fatalf("failed to resolve %s", outFile)
	}

	cmd := exec.Command("go", "build", "-trimpath", "-ldflags", "-w -s -buildid=", "-buildvcs=false", "-o", outFile, filepath.Join(buildDir, pkgPath))
	cmd.Env = append(os.Environ(), "GOFLAGS="+*goFlags)

	cmd.Dir = buildDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}

type Remapper struct {
	Build           *ObfBuild
	PublicIdentGen  IdentGen
	PrivateIdentGen IdentGen

	identMap map[string]string
}

func (remapper *Remapper) ApplyReplacements(transform *CodeTransform, replacements []FileReplacement) {
	build := remapper.Build

	for _, replacement := range replacements {
		if replacement.Ident != nil {
			name := replacement.Ident.Name

			if _, ok := build.ExcludedIdents[name]; ok {
				continue
			}

			replacementIdent := remapper.GetReplcement(name)

			transform.Replace(replacement.Ident, replacementIdent)
		} else {
			transform.Replace(replacement.Node, replacement.NewVal)
		}
	}
}

func (remapper *Remapper) GetReplcement(name string) string {
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
	return replacementIdent
}

// Either ident should be set or it should be nil and Node and NewVal set
type FileReplacement struct {
	Ident *ast.Ident

	Node   ast.Node
	NewVal string
}

//obf:preserve-fields
type File struct {
	Content           string
	Replacements      []FileReplacement
	Fset              *token.FileSet
	Ast               *ast.File
	BaseModuleImports []*ast.ImportSpec
	AppendContent     string
}

type Package struct {
	Name    string
	Pattern string
	Files   []File
	Embeds  map[string][]byte
}

type ObfBuild struct {
	BaseModule        string
	NameGen           IdentGen
	OutPath           string
	Packages          []Package
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
	err = os.WriteFile(filepath.Join(build.OutPath, "go.mod"), out, 0660)
	if err != nil {
		log.Fatalf("failed to write go.mod: %s", err)
	}
}

func (build *ObfBuild) copyGoSum() {
	data, err := os.ReadFile("go.sum")
	if err != nil {
		log.Fatalf("Failed to read go.sum: %s", err)
	}
	err = os.WriteFile(filepath.Join(build.OutPath, "go.sum"), data, 0660)
	if err != nil {
		log.Fatalf("Failed to write go.sum: %s", err)
	}
}
