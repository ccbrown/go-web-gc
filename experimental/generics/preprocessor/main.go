package main

import (
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/build"
	"go/importer"
	"go/parser"
	"go/printer"
	"go/scanner"
	"go/token"
	"go/types"
	"os"
	"strings"

	"github.com/ccbrown/wasm-go-playground/experimental/generics/preprocessor/astcopy"
	"golang.org/x/tools/go/ast/astutil"
)

var (
	fset       = token.NewFileSet()
	errorCount = 0
)

func parse(filename string) (*ast.File, error) {
	return parser.ParseFile(fset, filename, nil, parser.AllErrors)
}

func checkPkgFiles(files []*ast.File) *types.Info {
	type bailout struct{}

	// if checkPkgFiles is called multiple times, set up conf only once
	conf := types.Config{
		FakeImportC: true,
		Error: func(err error) {
			if errorCount >= 10 {
				panic(bailout{})
			}
			report(err)
		},
		Importer: importer.ForCompiler(fset, "source", nil),
		Sizes:    types.SizesFor(build.Default.Compiler, build.Default.GOARCH),
	}

	defer func() {
		switch p := recover().(type) {
		case nil, bailout:
			// normal return or early exit
		default:
			// re-panic
			panic(p)
		}
	}()

	const path = "pkg" // any non-empty string will do for now
	info := &types.Info{
		Defs:               map[*ast.Ident]types.Object{},
		Types:              map[ast.Expr]types.TypeAndValue{},
		Uses:               map[*ast.Ident]types.Object{},
		TypeArguments:      map[*ast.Ident][]types.Type{},
		NamedTypeArguments: map[*types.Named][]types.Type{},
	}
	conf.Check(path, fset, files, info)
	return info
}

func report(err error) {
	scanner.PrintError(os.Stderr, err)
	if list, ok := err.(scanner.ErrorList); ok {
		errorCount += len(list)
		return
	}
	errorCount++
}

func generatedName(name string, targs []types.Type) string {
	typeStrings := make([]string, len(targs))
	for i, targ := range targs {
		typeStrings[i] = targ.String()
	}
	return name + "_" + hex.EncodeToString([]byte(strings.Join(typeStrings, ";")))
}

func isParameterized(obj types.Object) bool {
	p, ok := obj.(interface {
		IsParameterized() bool
	})
	return ok && p.IsParameterized()
}

var generatedDecls = map[string]ast.Decl{}
var functionTemplates = map[string]*ast.FuncDecl{}
var typeTemplates = map[string]*ast.TypeSpec{}

func preprocess(node ast.Node, info *types.Info, parentTypeParams map[string]types.Type) ast.Node {
	// Remove type arguments from call sites
	node = astutil.Apply(node, func(c *astutil.Cursor) bool {
		switch node := c.Node().(type) {
		case *ast.CallExpr:
			if len(node.Args) > 0 {
				if t := info.Types[node.Args[0]]; t.IsType() || (t == types.TypeAndValue{}) {
					if ident, ok := node.Fun.(*ast.Ident); ok && isParameterized(info.Uses[ident]) {
						c.Replace(node.Fun)
					}
				}
			}
		}
		return true
	}, nil)

	type instantiation struct {
		name  string
		targs []types.Type
	}

	instantiatedFunctions := map[string]instantiation{}
	instantiatedTypes := map[string]instantiation{}

	// Replace call site identifiers
	node = astutil.Apply(node, func(c *astutil.Cursor) bool {
		switch node := c.Node().(type) {
		case *ast.CallExpr:
			if ident, ok := node.Fun.(*ast.Ident); ok && isParameterized(info.Uses[ident]) {
				if targs := info.TypeArguments[ident]; targs != nil {
					name := generatedName(ident.Name, targs)
					instantiatedFunctions[name] = instantiation{
						name:  ident.Name,
						targs: targs,
					}
					node.Fun = ast.NewIdent(name)
				}
			}
		}
		return true
	}, nil)

	// Replace named type identifiers
	node = astutil.Apply(node, func(c *astutil.Cursor) bool {
		switch node := c.Node().(type) {
		case *ast.Ident:
			if isParameterized(info.Uses[node]) {
				if targs := info.TypeArguments[node]; targs != nil {
					name := generatedName(node.Name, targs)
					if info.Types[node].IsValue() {
						instantiatedFunctions[name] = instantiation{
							name:  node.Name,
							targs: targs,
						}
					} else {
						instantiatedTypes[name] = instantiation{
							name:  node.Name,
							targs: targs,
						}
					}
					c.Replace(ast.NewIdent(name))
				}
			}
		}
		return true
	}, nil)

	// Generate functions
	for name, f := range instantiatedFunctions {
		if _, ok := generatedDecls[name]; ok {
			continue
		}

		for i, targ := range f.targs {
			if param, ok := targ.(*types.TypeParam); ok {
				f.targs[i] = parentTypeParams[param.String()]
			}
		}

		template := astcopy.FuncDecl(functionTemplates[f.name])
		template.Name = ast.NewIdent(name)
		decl := astutil.Apply(template, func(c *astutil.Cursor) bool {
			ident, ok := c.Node().(*ast.Ident)
			if !ok {
				return true
			}
			t := info.Types[ident]
			if !t.IsType() {
				return true
			}
			for i, tparam := range template.TParams.List {
				for j, tpident := range tparam.Names {
					if info.Defs[tpident] == info.Uses[ident] {
						newIdent := ast.NewIdent(f.targs[i+j].String())
						c.Replace(newIdent)
					}
				}
			}
			return true
		}, nil).(*ast.FuncDecl)
		parentTypeParams := map[string]types.Type{}
		for i, tparam := range template.TParams.List {
			for j, tpident := range tparam.Names {
				parentTypeParams[tpident.Name] = f.targs[i+j]
			}
		}
		decl = preprocess(decl, info, parentTypeParams).(*ast.FuncDecl)
		generatedDecls[name] = decl
	}

	// Generate types
	for len(instantiatedTypes) > 0 {
		temp := instantiatedTypes
		instantiatedTypes = map[string]instantiation{}
		for name, f := range temp {
			if _, ok := generatedDecls[name]; ok {
				continue
			}

			template := astcopy.TypeSpec(typeTemplates[f.name])
			spec := astutil.Apply(template, func(c *astutil.Cursor) bool {
				ident, ok := c.Node().(*ast.Ident)
				if !ok {
					return true
				}
				t := info.Types[ident]
				if !t.IsType() {
					return true
				}
				if info.Uses[ident] == info.Defs[template.Name] {
					c.Replace(ast.NewIdent(name))
					return true
				}
				for i, tparam := range template.TParams.List {
					for j, tpident := range tparam.Names {
						if info.Defs[tpident] == info.Uses[ident] {
							if named, ok := f.targs[i+j].(*types.Named); ok {
								if targs, ok := info.NamedTypeArguments[named]; ok {
									templateName := strings.Split(strings.Split(named.String(), ".")[1], "<")[0]
									name := generatedName(templateName, targs)
									instantiatedTypes[name] = instantiation{
										name:  templateName,
										targs: targs,
									}
									c.Replace(ast.NewIdent(name))
									return true
								}
							}
							c.Replace(ast.NewIdent(f.targs[i+j].String()))
						}
					}
				}
				return true
			}, nil).(*ast.TypeSpec)
			spec.Name = ast.NewIdent(name)
			generatedDecls[name] = &ast.GenDecl{
				Tok:   token.TYPE,
				Specs: []ast.Spec{spec},
			}
		}
	}

	return node
}

func main() {
	fout := os.Stdout
	if len(os.Args) > 2 {
		f, err := os.Create(os.Args[2])
		if err != nil {
			panic(err)
		}
		defer f.Close()
		fout = f

	}

	file, err := parse(os.Args[1])
	if err != nil {
		report(err)
	}

	var info *types.Info
	if errorCount == 0 {
		info = checkPkgFiles([]*ast.File{file})
	}

	if errorCount > 0 {
		os.Exit(2)
	}

	// Remove contracts. This has to be done first, otherwise astutil will choke.
	root := astutil.Apply(file, func(c *astutil.Cursor) bool {
		switch node := c.Node().(type) {
		case *ast.TypeSpec:
			if _, ok := node.Type.(*ast.ContractType); ok {
				c.Delete()
				return false
			}
		}
		return true
	}, nil)

	// Remove template definitions
	root = astutil.Apply(root, func(c *astutil.Cursor) bool {
		switch node := c.Node().(type) {
		case *ast.FuncDecl:
			if node.TParams != nil {
				functionTemplates[node.Name.Name] = node
				c.Delete()
				return false
			}
		case *ast.GenDecl:
			var newSpecs []ast.Spec
			for _, spec := range node.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok && typeSpec.TParams != nil {
					typeTemplates[typeSpec.Name.Name] = typeSpec
				} else {
					newSpecs = append(newSpecs, spec)
				}
			}
			node.Specs = newSpecs
			if len(newSpecs) == 0 {
				c.Delete()
				return false
			}
		}
		return true
	}, nil)

	root = preprocess(root, info, nil)

	printer.Fprint(fout, fset, root)
	for _, decl := range generatedDecls {
		fmt.Fprintf(fout, "\n")
		printer.Fprint(fout, fset, decl)
		fmt.Fprintf(fout, "\n")
	}
}
