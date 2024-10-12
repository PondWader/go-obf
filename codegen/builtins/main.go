package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"log"
	"net/http"
	"os"
	"reflect"
	"strings"
)

func main() {
	resp, err := http.Get("https://raw.githubusercontent.com/golang/go/refs/heads/master/src/builtin/builtin.go")
	if err != nil {
		log.Fatalf("failed to load builtin.go: %s", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("failed to load builtin.go: %s", err)
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "builtin.go", body, parser.SkipObjectResolution)
	if err != nil {
		log.Fatalf("failed to parse builtin.go: %s", err)
	}

	builtins := make([]string, 0, len(f.Decls))

	for _, decl := range f.Decls {
		switch t := decl.(type) {
		case *ast.FuncDecl:
			builtins = append(builtins, t.Name.Name)
		case *ast.GenDecl:
			for _, spec := range t.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					builtins = append(builtins, typeSpec.Name.Name)
				} else if valueSpec, ok := spec.(*ast.ValueSpec); ok {
					for _, name := range valueSpec.Names {
						builtins = append(builtins, name.Name)
					}
				}
			}
		default:
			log.Fatalf("got unexpected declaration: %v %s", t, reflect.TypeOf(t))
		}
	}

	arrayStr := "[...]string{\n	\"" + strings.Join(builtins, "\",\n	\"") + "\",\n}"
	fmt.Println("Generated: " + arrayStr)
	os.WriteFile("builtins.go", []byte("package main\n\nvar builtins = "+arrayStr+"\n"), 0644)
}
