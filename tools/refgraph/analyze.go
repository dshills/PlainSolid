package main

import (
	"bufio"
	"bytes"
	"fmt"
	"go/ast"
	"go/build"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// ref is one cross-file reference inside the analyzed package. Decl names are
// the enclosing top-level declarations; a method is "Receiver.Method".
type ref struct {
	useFile, useDecl string
	defFile, defDecl string
	kind, name       string
}

// analysis is the type-checked reference graph of one package directory.
type analysis struct {
	name    string              // package name
	prod    []string            // production file names
	tests   []string            // in-package test file names
	xtests  []string            // external-package test file names
	refs    map[ref]int         // cross-file references with counts
	decls   map[string][]string // file name to its top-level declaration names
	defFile map[string]string   // package-level name to its defining file
	exports map[string]string   // import path to export data file
}

type span struct {
	pos, end token.Pos
	name     string
}

// exportData asks the go command for the export files of a package's
// dependencies, including those of its tests.
func exportData(moduleDir, importPath string) (map[string]string, error) {
	cmd := exec.Command("go", "list", "-export", "-deps", "-test", "-f", "{{if .Export}}{{.ImportPath}}={{.Export}}{{end}}", importPath)
	cmd.Dir = moduleDir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go list %s: %v\n%s", importPath, err, stderr.String())
	}
	exports := map[string]string{}
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		k, v, ok := strings.Cut(sc.Text(), "=")
		if ok && !strings.Contains(k, " ") {
			exports[k] = v
		}
	}
	return exports, sc.Err()
}

func declSpans(f *ast.File) []span {
	var spans []span
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			name := d.Name.Name
			if r := receiverName(d); r != "" {
				name = r + "." + name
			}
			spans = append(spans, span{d.Pos(), d.End(), name})
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					spans = append(spans, span{s.Pos(), s.End(), s.Name.Name})
				case *ast.ValueSpec:
					spans = append(spans, span{s.Pos(), s.End(), s.Names[0].Name})
				}
			}
		}
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].pos < spans[j].pos })
	return spans
}

func receiverName(d *ast.FuncDecl) string {
	if d.Recv == nil || len(d.Recv.List) == 0 {
		return ""
	}
	t := d.Recv.List[0].Type
	if s, ok := t.(*ast.StarExpr); ok {
		t = s.X
	}
	switch x := t.(type) {
	case *ast.IndexExpr:
		t = x.X
	case *ast.IndexListExpr:
		t = x.X
	}
	if id, ok := t.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

func enclosing(spans []span, p token.Pos) string {
	i := sort.Search(len(spans), func(i int) bool { return spans[i].pos > p }) - 1
	if i >= 0 && p < spans[i].end {
		return spans[i].name
	}
	return ""
}

// analyze type-checks the package in dir together with its in-package tests.
func analyze(moduleDir, dir, importPath string) (*analysis, error) {
	exports, err := exportData(moduleDir, importPath)
	if err != nil {
		return nil, err
	}
	bp, err := build.ImportDir(dir, 0)
	if err != nil {
		return nil, err
	}
	a := &analysis{name: bp.Name, prod: bp.GoFiles, tests: bp.TestGoFiles, xtests: bp.XTestGoFiles,
		refs: map[ref]int{}, decls: map[string][]string{}, defFile: map[string]string{}, exports: exports}
	fset := token.NewFileSet()
	var files []*ast.File
	spans := map[string][]span{}
	for _, name := range append(append([]string{}, bp.GoFiles...), bp.TestGoFiles...) {
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		files = append(files, f)
		spans[name] = declSpans(f)
		for _, s := range spans[name] {
			a.decls[name] = append(a.decls[name], s.name)
		}
	}
	lookup := func(p string) (io.ReadCloser, error) {
		e, ok := exports[p]
		if !ok {
			return nil, fmt.Errorf("no export data for %s", p)
		}
		return os.Open(e)
	}
	var typeErrs []error
	conf := types.Config{Importer: importer.ForCompiler(fset, "gc", lookup), Error: func(err error) { typeErrs = append(typeErrs, err) }}
	info := &types.Info{Uses: map[*ast.Ident]types.Object{}}
	pkg, _ := conf.Check(importPath, fset, files, info)
	if len(typeErrs) > 0 {
		return nil, fmt.Errorf("%d type errors in %s, first: %v", len(typeErrs), importPath, typeErrs[0])
	}
	base := func(p token.Pos) string { return filepath.Base(fset.Position(p).Filename) }
	for _, name := range pkg.Scope().Names() {
		a.defFile[name] = base(pkg.Scope().Lookup(name).Pos())
	}
	for id, obj := range info.Uses {
		if obj == nil || obj.Pkg() != pkg || !obj.Pos().IsValid() {
			continue
		}
		kind := classifyObject(pkg, obj)
		u, d := base(id.Pos()), base(obj.Pos())
		if kind == "" || u == d {
			continue
		}
		a.refs[ref{u, enclosing(spans[u], id.Pos()), d, enclosing(spans[d], obj.Pos()), kind, obj.Name()}]++
	}
	// A method must live in the package of its receiver type.
	for _, f := range files {
		for _, decl := range f.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			r := receiverName(fd)
			obj := pkg.Scope().Lookup(r)
			if r == "" || obj == nil {
				continue
			}
			if u, d := base(fd.Pos()), base(obj.Pos()); u != d {
				a.refs[ref{u, r + "." + fd.Name.Name, d, r, "recv", r}]++
			}
		}
	}
	return a, nil
}

func classifyObject(pkg *types.Package, obj types.Object) string {
	top := obj.Parent() == pkg.Scope()
	switch o := obj.(type) {
	case *types.Func:
		if sig, ok := o.Type().(*types.Signature); ok && sig.Recv() != nil {
			return "method"
		}
		if top {
			return "func"
		}
	case *types.Var:
		if o.IsField() {
			return "field"
		}
		if top {
			return "var"
		}
	case *types.TypeName:
		if top {
			return "type"
		}
	case *types.Const:
		if top {
			return "const"
		}
	}
	return ""
}

// outsideUse lists, for one Go file outside the package, the files of the
// analyzed package that declare what it uses: functions, types, methods and
// fields alike.
type outsideUse struct {
	path string // relative to the scan root
	test bool
	defs []string
}

// uses type-checks files of one package and returns, per file, the base names
// of the files in the package at importPath that declare the objects it uses.
// Type errors are tolerated: an external test may rely on names that only the
// test build of its package exports, and the objects that do resolve still
// count.
func uses(exports map[string]string, dir string, names []string, path, importPath string) (map[string][]string, error) {
	if len(names) == 0 {
		return nil, nil
	}
	fset := token.NewFileSet()
	var files []*ast.File
	for _, name := range names {
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	lookup := func(p string) (io.ReadCloser, error) {
		e, ok := exports[p]
		if !ok {
			return nil, fmt.Errorf("no export data for %s", p)
		}
		return os.Open(e)
	}
	conf := types.Config{Importer: importer.ForCompiler(fset, "gc", lookup), Error: func(error) {}}
	info := &types.Info{Uses: map[*ast.Ident]types.Object{}, Defs: map[*ast.Ident]types.Object{}, Types: map[ast.Expr]types.TypeAndValue{}}
	_, _ = conf.Check(path, fset, files, info)
	seen := map[string]map[string]bool{}
	add := func(pos token.Pos, obj types.Object) {
		if obj == nil || obj.Pkg() == nil || obj.Pkg().Path() != importPath || !obj.Pos().IsValid() {
			return
		}
		u := filepath.Base(fset.Position(pos).Filename)
		if seen[u] == nil {
			seen[u] = map[string]bool{}
		}
		seen[u][filepath.Base(fset.Position(obj.Pos()).Filename)] = true
	}
	for id, obj := range info.Uses {
		add(id.Pos(), obj)
	}
	// A value of a package type used as an interface needs the methods of that
	// interface, although no method is named at the point of use. Where a
	// conversion happens is not recorded, so pair every package type a file
	// mentions with every interface it mentions and keep the methods of each
	// pair that fits. This can only keep too much.
	named := map[string]map[*types.Named]bool{}
	ifaces := map[string][]*types.Interface{}
	var note func(file string, t types.Type, depth int)
	note = func(file string, t types.Type, depth int) {
		if t == nil || depth > 3 {
			return
		}
		switch x := types.Unalias(t).(type) {
		case *types.Pointer:
			note(file, x.Elem(), depth+1)
		case *types.Slice:
			note(file, x.Elem(), depth+1)
		case *types.Signature:
			for _, tuple := range []*types.Tuple{x.Params(), x.Results()} {
				for i := 0; i < tuple.Len(); i++ {
					note(file, tuple.At(i).Type(), depth+1)
				}
			}
		case *types.Named:
			if o := x.Obj(); o.Pkg() != nil && o.Pkg().Path() == importPath && x.TypeParams().Len() == 0 {
				if named[file] == nil {
					named[file] = map[*types.Named]bool{}
				}
				named[file][x] = true
			}
			if i, ok := x.Underlying().(*types.Interface); ok && i.NumMethods() > 0 {
				ifaces[file] = append(ifaces[file], i)
			}
		case *types.Interface:
			if x.NumMethods() > 0 {
				ifaces[file] = append(ifaces[file], x)
			}
		}
	}
	for e, tv := range info.Types {
		note(filepath.Base(fset.Position(e.Pos()).Filename), tv.Type, 0)
	}
	for id, obj := range info.Defs {
		if obj != nil {
			note(filepath.Base(fset.Position(id.Pos()).Filename), obj.Type(), 0)
		}
	}
	// Production files of one package are paired with each other; a test file
	// is paired only with itself.
	group := func(file string) string {
		if strings.HasSuffix(file, "_test.go") {
			return file
		}
		return ""
	}
	groupNamed := map[string]map[*types.Named]bool{}
	groupIfaces := map[string][]*types.Interface{}
	members := map[string][]string{}
	for _, name := range names {
		g := group(name)
		members[g] = append(members[g], name)
		if groupNamed[g] == nil {
			groupNamed[g] = map[*types.Named]bool{}
		}
		for n := range named[name] {
			groupNamed[g][n] = true
		}
		groupIfaces[g] = append(groupIfaces[g], ifaces[name]...)
	}
	for g, set := range groupNamed {
		for n := range set {
			ptr := types.NewPointer(n)
			for _, i := range groupIfaces[g] {
				if !types.Implements(ptr, i) {
					continue
				}
				for k := 0; k < i.NumMethods(); k++ {
					m := i.Method(k)
					obj, _, _ := types.LookupFieldOrMethod(ptr, true, m.Pkg(), m.Name())
					for _, file := range members[g] {
						if named[file][n] {
							add(fileStart(fset, files, file), obj)
						}
					}
				}
			}
		}
	}
	out := map[string][]string{}
	for u, set := range seen {
		for d := range set {
			out[u] = append(out[u], d)
		}
		sort.Strings(out[u])
	}
	return out, nil
}

// fileStart returns a position inside the named file.
func fileStart(fset *token.FileSet, files []*ast.File, name string) token.Pos {
	for _, f := range files {
		if filepath.Base(fset.Position(f.Pos()).Filename) == name {
			return f.Pos()
		}
	}
	return token.NoPos
}

// scanOutside type-checks every package under root that imports importPath,
// other than the one in pkgDir, and records what each of its files uses.
func scanOutside(moduleDir, root, pkgDir, importPath string) ([]outsideUse, error) {
	rel, err := filepath.Rel(moduleDir, root)
	if err != nil {
		return nil, err
	}
	pattern := "./" + filepath.ToSlash(rel) + "/..."
	exports, err := exportData(moduleDir, pattern)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command("go", "list", "-f", `{{.ImportPath}}|{{.Dir}}|{{join .Imports ","}},{{join .TestImports ","}},{{join .XTestImports ","}}`, pattern)
	cmd.Dir = moduleDir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	listing, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go list %s: %v\n%s", pattern, err, stderr.String())
	}
	var out []outsideUse
	for _, line := range strings.Split(strings.TrimSpace(string(listing)), "\n") {
		parts := strings.SplitN(line, "|", 3)
		if len(parts) != 3 || parts[1] == pkgDir {
			continue
		}
		imports := false
		for _, imp := range strings.Split(parts[2], ",") {
			imports = imports || imp == importPath
		}
		if !imports {
			continue
		}
		bp, err := build.ImportDir(parts[1], 0)
		if err != nil {
			return nil, err
		}
		groups := []struct {
			names []string
			path  string
		}{{append(append([]string{}, bp.GoFiles...), bp.TestGoFiles...), parts[0]}, {bp.XTestGoFiles, parts[0] + "_test"}}
		for _, g := range groups {
			found, err := uses(exports, parts[1], g.names, g.path, importPath)
			if err != nil {
				return nil, err
			}
			for name, defs := range found {
				r, err := filepath.Rel(root, filepath.Join(parts[1], name))
				if err != nil {
					return nil, err
				}
				out = append(out, outsideUse{filepath.ToSlash(r), strings.HasSuffix(name, "_test.go"), defs})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].path < out[j].path })
	return out, nil
}

// selectedNames returns the names a file selects from importPath.
func selectedNames(fset *token.FileSet, path, importPath, pkgName string) ([]string, error) {
	f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	local := ""
	for _, imp := range f.Imports {
		if strings.Trim(imp.Path.Value, `"`) != importPath {
			continue
		}
		local = pkgName
		if imp.Name != nil {
			local = imp.Name.Name
		}
	}
	if local == "" || local == "_" {
		return nil, nil
	}
	seen := map[string]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.SelectorExpr:
			if id, ok := x.X.(*ast.Ident); ok && id.Name == local && local != "." {
				seen[x.Sel.Name] = true
			}
		case *ast.Ident:
			if local == "." {
				seen[x.Name] = true
			}
		}
		return true
	})
	names := make([]string, 0, len(seen))
	for n := range seen {
		names = append(names, n)
	}
	sort.Strings(names)
	return names, nil
}
