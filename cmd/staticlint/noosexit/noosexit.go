// Package noosexit provides an analyzer that forbids calling os.Exit directly in main.
package noosexit

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

// Analyzer reports direct os.Exit calls in the main function of package main.
var Analyzer = &analysis.Analyzer{
	Name: "noosexit",
	Doc:  "forbids direct os.Exit calls in main function of package main",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	if pass.Pkg.Name() != "main" {
		return nil, nil
	}
	for _, file := range pass.Files {
		if ast.IsGenerated(file) {
			continue
		}
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && isMain(fn) {
				checkBody(pass, fn.Body)
			}
		}
	}
	return nil, nil
}

func isMain(fn *ast.FuncDecl) bool {
	return fn.Recv == nil && fn.Name.Name == "main" && fn.Body != nil
}

func checkBody(pass *analysis.Pass, body *ast.BlockStmt) {
	ast.Inspect(body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok && isOSExit(pass, call) {
			pass.Reportf(call.Pos(), "direct call to os.Exit in main is forbidden")
		}
		return true
	})
}

func isOSExit(pass *analysis.Pass, call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	fn, ok := pass.TypesInfo.Uses[sel.Sel].(*types.Func)
	return ok && fn.Pkg() != nil && fn.Pkg().Path() == "os" && fn.Name() == "Exit"
}
