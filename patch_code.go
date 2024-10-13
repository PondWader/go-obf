package main

import (
	"go/ast"
	"log"
	"os"
	"strings"
	"unicode"

	"golang.org/x/tools/go/packages"
)

// Returns the resolved name of the package and a boolean value on whether or not it is in the base module
func (build *ObfBuild) patchPackage(pattern string) (string, bool) {
	// Make sure package is not processed twice
	if isInBaseModule, ok := build.ProcessedPackages[pattern]; ok {
		slashSplit := strings.Split(pattern, "/")
		return slashSplit[len(slashSplit)-1], isInBaseModule
	}

	cfg := &packages.Config{Mode: packages.NeedName | packages.NeedSyntax | packages.NeedFiles}
	resolvedPkgs, err := packages.Load(cfg, pattern)
	if err != nil {
		log.Fatalf("Failed to load package %s: %s", pattern, err)
	}
	resolvedPkg := resolvedPkgs[0]

	// Check if this package is in the base module and so should be obfuscated
	isInBaseModule := resolvedPkg.PkgPath == build.BaseModule || strings.HasPrefix(resolvedPkg.PkgPath, build.BaseModule+"/")

	// Check package again this time using the resolved ID
	if _, ok := build.ProcessedPackages[resolvedPkg.ID]; ok {
		return resolvedPkg.Name, isInBaseModule
	}

	build.ProcessedPackages[resolvedPkg.ID] = isInBaseModule

	// If it is not the base module, identifiers are simply added to the exclude list to not be obfuscated.
	if !isInBaseModule {
		build.ignoreIdentsInPackage(resolvedPkg.Syntax)
		return resolvedPkg.Name, isInBaseModule
	}

	// Since it is a base module, identifiers need to be added to the list to be obfuscated.

	pkg := Package{
		Name:  resolvedPkg.Name,
		Files: make([]File, len(resolvedPkg.Syntax)),
	}

	fset := resolvedPkg.Fset

	for i, file := range resolvedPkg.Syntax {
		filePath := fset.File(file.Pos()).Name()
		content, err := os.ReadFile(filePath)
		if err != nil {
			log.Fatalf("failed to read %s: %s", filePath, err)
		}

		f := File{
			Content:           string(content),
			Replacements:      make([]*ast.Ident, 0),
			Fset:              fset,
			Ast:               file,
			BaseModuleImports: make([]*ast.ImportSpec, 0),
		}

		importIdents := make(map[string]bool)

		preserveFieldsLine := -1

		ast.Inspect(file, func(n ast.Node) bool {
			switch t := n.(type) {
			case *ast.ImportSpec: // Collect names of imports
				// Remove quotation marks around path
				path := t.Path.Value[1 : len(t.Path.Value)-1]
				name, inBaseModule := build.patchPackage(path)

				if t.Name != nil {
					name = t.Name.Name
				}

				// If the package is in a base module store the import so it can be modified
				if inBaseModule {
					f.BaseModuleImports = append(f.BaseModuleImports, t)
				} else {
					// Otherwise we want to leave the name unmodified
					build.ExcludedIdents[name] = true
					importIdents[name] = true
				}
				return false

			case *ast.SelectorExpr: // Stop import selectors from having names changed
				ident := getSelectorExprRootIdent(t)
				if ident != nil {
					if _, ok := importIdents[ident.Name]; ok {
						return false
					}
				}

			case *ast.Ident:
				// Skip identifier in package declaration since it's stored in the file
				if t == f.Ast.Name {
					return true
				}
				f.Replacements = append(f.Replacements, t)

			case *ast.Comment:
				line := fset.Position(t.Pos()).Line
				if strings.TrimSpace(t.Text) == "//obf:preserve-fields" {
					preserveFieldsLine = line
				}

			case *ast.StructType:
				line := fset.Position(t.Pos()).Line
				// If the struct follows a preserve fields directive then field names should be added to the excluded identifiers
				if line == preserveFieldsLine+1 {
					for _, field := range t.Fields.List {
						for _, name := range field.Names {
							build.ExcludedIdents[name.Name] = true
						}
					}
					return false
				}
			}

			return true
		})

		pkg.Files[i] = f
	}

	build.Packages[pattern] = pkg
	return resolvedPkg.Name, isInBaseModule
}

// Adds public identifiers in a packages files to the list of identifiers to be ignored.
func (build *ObfBuild) ignoreIdentsInPackage(syntax []*ast.File) {
	for _, file := range syntax {
		ast.Inspect(file, func(n ast.Node) bool {
			switch t := n.(type) {
			case *ast.FuncDecl: // Ignore bodies of functions
				if unicode.IsUpper(rune(t.Name.Name[0])) {
					build.ExcludedIdents[t.Name.Name] = true
				}
				return false
			case *ast.ImportSpec: // Ignore import identifiers
				// Remove quotation marks around path
				path := t.Path.Value[1 : len(t.Path.Value)-1]
				build.patchPackage(path)
				return false
			case *ast.Ident: // Capture identifiers
				if unicode.IsUpper(rune(t.Name[0])) {
					build.ExcludedIdents[t.Name] = true
				}
			}
			return true
		})
	}
}

// Gets the root ident of a selector expression if it exists.
// Used to not modify identifiers used in an import selector
func getSelectorExprRootIdent(selector *ast.SelectorExpr) *ast.Ident {
	if innerSelector, ok := selector.X.(*ast.SelectorExpr); ok {
		return getSelectorExprRootIdent(innerSelector)
	} else if ident, ok := selector.X.(*ast.Ident); ok {
		return ident
	}
	return nil
}
