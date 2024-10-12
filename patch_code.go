package main

import (
	"fmt"
	"go/ast"
	"log"
	"os"
	"strings"
	"unicode"

	"golang.org/x/tools/go/packages"
)

// Returns the resolved name of the package
func (build *ObfBuild) patchPackage(pattern string) string {
	// Make sure package is not processed twice
	if _, ok := build.ProcessedPackages[pattern]; ok {
		fmt.Println("Fast skipped processing", pattern)
		return pattern
	}

	cfg := &packages.Config{Mode: packages.NeedName | packages.NeedSyntax | packages.NeedFiles}
	resolvedPkgs, err := packages.Load(cfg, pattern)
	if err != nil {
		log.Fatalf("Failed to load package %s: %s", pattern, err)
	}
	resolvedPkg := resolvedPkgs[0]

	// Check package again this time using the resolved ID
	if _, ok := build.ProcessedPackages[resolvedPkg.ID]; ok {
		fmt.Println("Skipped processing", resolvedPkg.PkgPath)
		return resolvedPkg.Name
	}
	build.ProcessedPackages[resolvedPkg.ID] = true

	fmt.Println("Processing", resolvedPkg.PkgPath)
	defer func() {
		fmt.Println("Finished processing", resolvedPkg.PkgPath)
	}()

	// Check if this package is in the base module and so should be obfuscated
	isInBaseModule := resolvedPkg.PkgPath == build.BaseModule || strings.HasPrefix(resolvedPkg.PkgPath, build.BaseModule+"/")

	// If it is not the base module, identifiers are simply added to the exclude list to not be obfuscated.
	if !isInBaseModule {
		build.ignoreIdentsInPackage(resolvedPkg.Syntax)
		return resolvedPkg.Name
	}

	// Since it is a base module, identifiers need to be added to the list to be obfuscated.

	pkg := Package{
		Name:  resolvedPkg.Name,
		Files: make([]File, len(resolvedPkg.Syntax)),
	}

	for i, file := range resolvedPkg.Syntax {
		filePath := resolvedPkg.Fset.File(file.Pos()).Name()
		content, err := os.ReadFile(filePath)
		if err != nil {
			log.Fatalf("failed to read %s: %s", filePath, err)
		}

		f := File{
			Content:      string(content),
			Replacements: make([]*ast.Ident, 0),
		}

		importIdents := make(map[string]bool)

		ast.Inspect(file, func(n ast.Node) bool {
			switch t := n.(type) {
			case *ast.ImportSpec: // Collect names of imports
				// Remove quotation marks around path
				path := t.Path.Value[1 : len(t.Path.Value)-1]
				name := build.patchPackage(path)

				if t.Name != nil {
					importIdents[t.Name.Name] = true
				} else {
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
				f.Replacements = append(f.Replacements, t)
			}

			return true
		})

		pkg.Files[i] = f
	}

	build.Packages[pattern] = pkg
	return resolvedPkg.Name
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

// should account for public or private
/*func (build *ObfBuild) getIdentReplacement(ident string) string {
	if _, ok := build.ExcludedIdents[ident]; ok {
		return ident
	}
	newIdent, ok := build.IdentReplacements[ident]
	if ok {
		return newIdent
	}
	newIdent = build.NameGen.Next()
	build.IdentReplacements[ident] = newIdent
	return newIdent
}
*/
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
