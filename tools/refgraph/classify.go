package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strings"
)

// classification partitions a package into the files other packages reach
// ("core") and the files they do not ("research"), and says which test files
// can only build with the research files present.
type classification struct {
	Core, Research []string // production file names
	// ResearchTests are in-package test files holding at least one research
	// declaration. MixedTests is the subset that also holds a core test or a
	// helper that a core declaration in another file uses; split those first.
	ResearchTests, MixedTests []string
	ResearchXTests            []string // external-package test files in the directory
	// Outside are files elsewhere under the scan root that name a research
	// symbol: tests, and files under an excluded directory.
	Outside        []string
	CoreToResearch int // references from a core file to a research file
}

func isTestFunc(name string) bool {
	for _, p := range []string{"Test", "Fuzz", "Example", "Benchmark"} {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

// classify applies the rules in specs/brep-trimmed-split/PLAN.md. A root is
// anything a non-test file outside the package, and outside the excluded
// directories, uses from it: a function, type, method or field.
func classify(a *analysis, exports map[string]string, outside []outsideUse, dir, importPath string, exclude []string) (classification, error) {
	excluded := func(path string) bool {
		for _, e := range exclude {
			if path == e || strings.HasPrefix(path, e+"/") {
				return true
			}
		}
		return false
	}
	prod := map[string]bool{}
	for _, f := range a.prod {
		prod[f] = true
	}
	graph := map[string][]string{}
	for r := range a.refs {
		if prod[r.useFile] && prod[r.defFile] {
			graph[r.useFile] = append(graph[r.useFile], r.defFile)
		}
	}
	core := map[string]bool{}
	var stack []string
	for _, u := range outside {
		if u.test || excluded(u.path) {
			continue
		}
		for _, f := range u.defs {
			if prod[f] && !core[f] {
				core[f] = true
				stack = append(stack, f)
			}
		}
	}
	for len(stack) > 0 {
		f := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, d := range graph[f] {
			if !core[d] {
				core[d] = true
				stack = append(stack, d)
			}
		}
	}
	var c classification
	research := map[string]bool{}
	for _, f := range a.prod {
		if core[f] {
			c.Core = append(c.Core, f)
		} else {
			research[f] = true
			c.Research = append(c.Research, f)
		}
	}
	for r, n := range a.refs {
		if core[r.useFile] && research[r.defFile] {
			c.CoreToResearch += n
		}
	}

	// Test declarations, to a fixed point over references between them.
	type decl struct{ file, name string }
	uses := map[decl][]decl{} // test declaration to the test declarations it references
	bad := map[decl]bool{}
	for r := range a.refs {
		if prod[r.useFile] {
			continue
		}
		u := decl{r.useFile, r.useDecl}
		if research[r.defFile] {
			bad[u] = true
		} else if !prod[r.defFile] {
			uses[u] = append(uses[u], decl{r.defFile, r.defDecl})
		}
	}
	for changed := true; changed; {
		changed = false
		for u, ds := range uses {
			if bad[u] {
				continue
			}
			for _, d := range ds {
				if bad[d] {
					bad[u], changed = true, true
					break
				}
			}
		}
	}
	usedByCore := map[decl]bool{} // helpers a core declaration in another file relies on
	for u, ds := range uses {
		if bad[u] {
			continue
		}
		for _, d := range ds {
			usedByCore[d] = true
		}
	}
	for _, f := range a.tests {
		hasResearch, hasCore := false, false
		for _, name := range a.decls[f] {
			d := decl{f, name}
			if bad[d] {
				hasResearch = true
			} else if isTestFunc(name) || usedByCore[d] {
				hasCore = true
			}
		}
		if hasResearch {
			c.ResearchTests = append(c.ResearchTests, f)
			if hasCore {
				c.MixedTests = append(c.MixedTests, f)
			}
		}
	}

	xr, err := classifyXTests(a, exports, research, dir, importPath)
	if err != nil {
		return c, err
	}
	c.ResearchXTests = xr
	for _, u := range outside {
		if !u.test && !excluded(u.path) {
			continue
		}
		for _, f := range u.defs {
			if research[f] {
				c.Outside = append(c.Outside, u.path)
				break
			}
		}
	}
	sort.Strings(c.Outside)
	return c, nil
}

// classifyXTests marks an external-package test file as research when it uses
// something a research file declares, or a declaration of another such file
// that does. Uses are found by type-checking and, because such tests may rely
// on names only the test build exports, also by identifier.
func classifyXTests(a *analysis, exports map[string]string, research map[string]bool, dir, importPath string) ([]string, error) {
	typed, err := uses(exports, dir, a.xtests, importPath+"_test", importPath)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	declared := map[string]string{} // top-level name to its file
	idents := map[string]map[string]bool{}
	bad := map[string]bool{}
	for _, name := range a.xtests {
		for _, f := range typed[name] {
			bad[name] = bad[name] || research[f]
		}
		path := filepath.Join(dir, name)
		f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		for _, s := range declSpans(f) {
			declared[s.name] = name
		}
		idents[name] = map[string]bool{}
		ast.Inspect(f, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok {
				idents[name][id.Name] = true
			}
			return true
		})
		used, err := selectedNames(fset, path, importPath, a.name)
		if err != nil {
			return nil, err
		}
		for _, n := range used {
			if research[a.defFile[n]] {
				bad[name] = true
			}
		}
	}
	for changed := true; changed; {
		changed = false
		for _, name := range a.xtests {
			if bad[name] {
				continue
			}
			for id := range idents[name] {
				if owner, ok := declared[id]; ok && owner != name && bad[owner] {
					bad[name], changed = true, true
					break
				}
			}
		}
	}
	var out []string
	for _, name := range a.xtests {
		if bad[name] {
			out = append(out, name)
		}
	}
	return out, nil
}
