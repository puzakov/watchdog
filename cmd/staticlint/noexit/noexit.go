// Package noexit provides an analyzer that forbids direct calls to os.Exit
// inside the main function of the main package.
//
// # Analyzer name
//
//	noexit
//
// # Description
//
// The noexit analyzer checks that the main function in package main does
// not contain direct calls to os.Exit. This is useful for enforcing a
// clean program structure where main delegates to a run function that
// returns an error, allowing deferred cleanup to run naturally.
//
// # Rationale
//
// Direct os.Exit calls in main.main bypass deferred function calls,
// leading to resource leaks (open files, unclosed DB connections,
// unfinished audit log writes). The recommended pattern is to have
// main call a run() function that returns an error, log the error,
// and exit via return (or log.Fatal if immediate exit is required).
//
// # Example
//
//	// BAD — os.Exit in main.main:
//	func main() {
//	    if err := run(); err != nil {
//	        os.Exit(1) // ❌ noexit will report this
//	    }
//	}
//
//	// GOOD — no direct os.Exit:
//	func main() {
//	    if err := run(); err != nil {
//	        log.Fatal(err) // indirect exit via log.Fatal is allowed
//	    }
//	}
package noexit

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
)

// Analyzer defines the noexit analyzer that reports direct calls to os.Exit
// within the main function of the main package.
var Analyzer = &analysis.Analyzer{
	Name: "noexit",
	Doc:  "reports direct calls to os.Exit in main function of package main",
	Run:  run,
}

// run is the main analysis function. It traverses the AST of each file
// in the analyzed package, looking for os.Exit calls inside the main function.
func run(pass *analysis.Pass) (any, error) {
	if pass.Pkg.Name() != "main" {
		return nil, nil
	}

	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			funcDecl, ok := decl.(*ast.FuncDecl)
			if !ok || funcDecl.Name.Name != "main" || funcDecl.Recv != nil {
				continue
			}

			// Traverse only the body of the main function
			ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}

				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}

				ident, ok := sel.X.(*ast.Ident)
				if !ok || ident.Name != "os" || sel.Sel.Name != "Exit" {
					return true
				}

				pass.Reportf(call.Pos(), "direct call to os.Exit is forbidden in main function of package main")
				return true
			})
		}
	}

	return nil, nil
}
