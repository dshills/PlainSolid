// Command refgraph type-checks one package, with its in-package tests, and
// prints every cross-file reference to a package-level object, method or
// struct field. It measures how separable the files of a large package are.
//
//	go list -export -deps -test -f '{{if .Export}}{{.ImportPath}}={{.Export}}{{end}}' ./brep/trimmed > exports.txt
//	go run ./tools/refgraph brep/trimmed plainsolid/brep/trimmed exports.txt > edges.tsv
//
// Each output line is: using file, defining file, kind, name, exported, count.
// Kind is func, type, var, const, method, field, or recv. A recv line means a
// method is declared in a different file from its receiver type; the two files
// must stay in one package.
package main

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/build"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintln(os.Stderr, "usage: refgraph <pkgdir> <importpath> <exports.txt>")
		os.Exit(2)
	}
	dir, path := os.Args[1], os.Args[2]
	exports := map[string]string{}
	f, err := os.Open(os.Args[3])
	if err != nil {
		panic(err)
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		k, v, ok := strings.Cut(sc.Text(), "=")
		if ok && !strings.Contains(k, " ") {
			exports[k] = v
		}
	}
	bp, err := build.ImportDir(dir, 0)
	if err != nil {
		panic(err)
	}
	fset := token.NewFileSet()
	var files []*ast.File
	for _, name := range append(append([]string{}, bp.GoFiles...), bp.TestGoFiles...) {
		af, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.SkipObjectResolution)
		if err != nil {
			panic(err)
		}
		files = append(files, af)
	}
	lookup := func(p string) (io.ReadCloser, error) {
		e, ok := exports[p]
		if !ok {
			return nil, fmt.Errorf("no export data for %s", p)
		}
		return os.Open(e)
	}
	nerr := 0
	conf := types.Config{Importer: importer.ForCompiler(fset, "gc", lookup), Error: func(err error) {
		if nerr < 5 {
			fmt.Fprintln(os.Stderr, "typecheck:", err)
		}
		nerr++
	}}
	info := &types.Info{Uses: map[*ast.Ident]types.Object{}, Defs: map[*ast.Ident]types.Object{}}
	pkg, _ := conf.Check(path, fset, files, info)
	fmt.Fprintln(os.Stderr, "files", len(files), "xtest", len(bp.XTestGoFiles), "type errors", nerr)
	base := func(p token.Pos) string { return filepath.Base(fset.Position(p).Filename) }
	type key struct{ use, def, kind, name string }
	count := map[key]int{}
	classify := func(obj types.Object) string {
		switch o := obj.(type) {
		case *types.Func:
			if sig, ok := o.Type().(*types.Signature); ok && sig.Recv() != nil {
				return "method"
			}
			if o.Parent() == pkg.Scope() {
				return "func"
			}
		case *types.Var:
			if o.IsField() {
				return "field"
			}
			if o.Parent() == pkg.Scope() {
				return "var"
			}
		case *types.TypeName:
			if o.Parent() == pkg.Scope() {
				return "type"
			}
		case *types.Const:
			if o.Parent() == pkg.Scope() {
				return "const"
			}
		}
		return ""
	}
	for id, obj := range info.Uses {
		if obj == nil || obj.Pkg() != pkg || !obj.Pos().IsValid() {
			continue
		}
		kind := classify(obj)
		if kind == "" {
			continue
		}
		u, d := base(id.Pos()), base(obj.Pos())
		if u != d {
			count[key{u, d, kind, obj.Name()}]++
		}
	}
	// A method must live in the package of its receiver type.
	for _, af := range files {
		for _, decl := range af.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Recv == nil || len(fd.Recv.List) == 0 {
				continue
			}
			t := fd.Recv.List[0].Type
			if s, ok := t.(*ast.StarExpr); ok {
				t = s.X
			}
			if ix, ok := t.(*ast.IndexExpr); ok {
				t = ix.X
			}
			id, ok := t.(*ast.Ident)
			if !ok {
				continue
			}
			if obj := pkg.Scope().Lookup(id.Name); obj != nil {
				u, d := base(fd.Pos()), base(obj.Pos())
				if u != d {
					count[key{u, d, "recv", id.Name}]++
				}
			}
		}
	}
	var out strings.Builder
	for k, n := range count {
		fmt.Fprintf(&out, "%s\t%s\t%s\t%s\t%t\t%d\n", k.use, k.def, k.kind, k.name, ast.IsExported(k.name), n)
	}
	if _, err := os.Stdout.WriteString(out.String()); err != nil {
		panic(err)
	}
}
