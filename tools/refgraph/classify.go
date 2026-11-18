package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// classification partitions a package into the files other packages reach
// ("core") and the files they do not ("research"), and says which test files
// can only build with the research files present.
type classification struct {
	Core, Research []string // production file names
	// A test declaration is core when a test that needs no research code is
	// declared there or reaches it; every other test declaration is research.
	// ResearchTests are in-package test files holding only research
	// declarations. MixedTests hold both kinds and must be split; Moves lists
	// what to take out of them.
	ResearchTests, MixedTests []string
	Moves                     []move
	ResearchXTests            []string // external-package test files in the directory
	// Outside are files elsewhere under the scan root that name a research
	// symbol: tests, and files under an excluded directory.
	Outside        []string
	CoreToResearch int // references from a core file to a research file
}

// move is one byte range to take out of a mixed test file. Partial is set
// when only some declarations of a parenthesized group are research; such a
// group has to be split by hand.
type move struct {
	File     string
	From, To int
	Name     string
	Partial  bool
}

// groupMoves turns the declarations to remove from one file into byte ranges,
// moving a parenthesized group as a whole when all of it goes.
func groupMoves(file string, all, out []span) []move {
	total := map[int]int{}
	for _, s := range all {
		if s.group {
			total[s.groupFrom]++
		}
	}
	leaving := map[int]int{}
	for _, s := range out {
		if s.group {
			leaving[s.groupFrom]++
		}
	}
	var moves []move
	done := map[int]bool{}
	for _, s := range out {
		if !s.group {
			moves = append(moves, move{File: file, From: s.from, To: s.to, Name: s.name})
			continue
		}
		if done[s.groupFrom] {
			continue
		}
		done[s.groupFrom] = true
		moves = append(moves, move{File: file, From: s.groupFrom, To: s.groupTo, Name: s.name, Partial: leaving[s.groupFrom] != total[s.groupFrom]})
	}
	return moves
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

	c.ResearchTests, c.MixedTests, c.Moves = splitTests(a, research, nil)

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

// splitTests classifies the test declarations of an analyzed group of files.
// A declaration is research when it uses a research production file of its own
// package, when seed reports that it uses research code of another package,
// or when it reaches such a declaration. It returns the test files that hold
// only research declarations, the files that hold both kinds, and the byte
// ranges to take out of the latter.
func splitTests(a *analysis, research map[string]bool, seed func(file, name string) bool) (researchTests, mixedTests []string, moves []move) {
	prod := map[string]bool{}
	for _, f := range a.prod {
		prod[f] = true
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
	if seed != nil {
		for _, f := range a.tests {
			for _, name := range a.decls[f] {
				if seed(f, name) {
					bad[decl{f, name}] = true
				}
			}
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
	// Core declarations: tests that need no research code, and everything
	// they reach. A method goes with its receiver type unless it is research.
	isTest := map[string]bool{}
	for _, f := range a.tests {
		isTest[f] = true
	}
	needed := map[decl]bool{}
	var work []decl
	mark := func(d decl) {
		if !needed[d] && !bad[d] {
			needed[d] = true
			work = append(work, d)
		}
	}
	for _, f := range a.tests {
		for _, name := range a.decls[f] {
			if isTestFunc(name) || name == "init" || name == "_" {
				mark(decl{f, name})
			}
		}
	}
	for grew := true; grew; {
		for len(work) > 0 {
			d := work[len(work)-1]
			work = work[:len(work)-1]
			for _, u := range uses[d] {
				mark(u)
			}
		}
		grew = false
		for _, f := range a.tests {
			for _, name := range a.decls[f] {
				recv, _, isMethod := strings.Cut(name, ".")
				d := decl{f, name}
				if isMethod && !needed[d] && !bad[d] && isTest[a.defFile[recv]] && needed[decl{a.defFile[recv], recv}] {
					mark(d)
					grew = true
				}
			}
		}
	}
	for _, f := range a.tests {
		var out []span
		kept := 0
		for _, s := range a.spans[f] {
			if needed[decl{f, s.name}] {
				kept++
			} else {
				out = append(out, s)
			}
		}
		switch {
		case len(out) == 0:
		case kept == 0:
			researchTests = append(researchTests, f)
		default:
			mixedTests = append(mixedTests, f)
			moves = append(moves, groupMoves(f, a.spans[f], out)...)
		}
	}
	return researchTests, mixedTests, moves
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
		for _, s := range declSpans(fset, f) {
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

// dependent is one group of test files in another package: its in-package
// tests, or its external-package tests.
type dependent struct {
	Dir                       string // relative to the module root
	ResearchTests, MixedTests []string
	Moves                     []move
}

// classifyDependents applies the declaration rules to the tests of every
// package that uses the analyzed one. Research there means using something a
// research file of the analyzed package declares.
func classifyDependents(moduleDir, pkgDir, importPath string, research []string) ([]dependent, error) {
	isResearch := map[string]bool{}
	for _, f := range research {
		isResearch[f] = true
	}
	exports, err := exportData(moduleDir, "./...")
	if err != nil {
		return nil, err
	}
	cmd := exec.Command("go", "list", "-tags", buildTags, "-f", `{{.ImportPath}}|{{.Dir}}|{{join .TestImports ","}},{{join .XTestImports ","}}`, "./...")
	cmd.Dir = moduleDir
	listing, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go list: %v", err)
	}
	var out []dependent
	for _, line := range strings.Split(strings.TrimSpace(string(listing)), "\n") {
		parts := strings.SplitN(line, "|", 3)
		if len(parts) != 3 || parts[1] == pkgDir || !strings.Contains(","+parts[2]+",", ","+importPath+",") {
			continue
		}
		bp, err := buildContext().ImportDir(parts[1], 0)
		if err != nil {
			return nil, err
		}
		rel, err := filepath.Rel(moduleDir, parts[1])
		if err != nil {
			return nil, err
		}
		groups := []struct {
			prod, tests []string
			path        string
		}{{bp.GoFiles, bp.TestGoFiles, parts[0]}, {nil, bp.XTestGoFiles, parts[0] + "_test"}}
		for _, g := range groups {
			if len(g.tests) == 0 {
				continue
			}
			a, err := analyzeFiles(exports, parts[1], g.path, g.prod, g.tests, importPath, false)
			if err != nil {
				return nil, err
			}
			seed := func(file, name string) bool {
				for _, f := range a.ext[[2]string{file, name}] {
					if isResearch[f] {
						return true
					}
				}
				return false
			}
			d := dependent{Dir: filepath.ToSlash(rel)}
			d.ResearchTests, d.MixedTests, d.Moves = splitTests(a, nil, seed)
			if len(d.ResearchTests)+len(d.MixedTests) > 0 {
				out = append(out, d)
			}
		}
	}
	return out, nil
}
