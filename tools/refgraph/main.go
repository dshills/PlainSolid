// Command refgraph measures how separable the files of one large package are.
// It type-checks the package with its in-package tests, records every
// cross-file reference to a package-level object, method or struct field, and
// partitions the files into those other packages reach and those they do not.
//
//	go run ./tools/refgraph -tags research -pkg brep/trimmed -exclude examples -out specs/brep-trimmed-split
//	go run ./tools/refgraph -pkg brep/trimmed -edges edges.tsv
//	go run ./tools/refgraph -tags research -pkg brep/trimmed -exclude examples -check research
//
// -check is the standing gate: it fails unless the named build tag marks
// exactly the research files.
//
// The rules are in specs/brep-trimmed-split/PLAN.md. Run it from the module
// root. -out writes core-files.txt, research-files.txt, research-tests.txt,
// mixed-tests.txt, outside-files.txt and mixed-moves.tsv; the last lists the
// byte range of each declaration to take out of a mixed test file. For the
// tests of other packages it writes dependent-tests.txt and
// dependent-moves.tsv in the same way. -edges writes one line per
// reference: using file and declaration, defining file and declaration,
// kind, name and count. Kind "recv" means a method is declared in a different
// file from its receiver type, so the two files must stay in one package.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	pkg := flag.String("pkg", "", "package directory relative to the module root")
	exclude := flag.String("exclude", "", "comma-separated directories whose files are not roots")
	out := flag.String("out", "", "directory for the file lists")
	edges := flag.String("edges", "", "file for the reference list")
	flag.StringVar(&buildTags, "tags", "", "comma-separated build tags, as for go build")
	check := flag.String("check", "", "build tag that research files must carry; exit 1 if the tags and the classification disagree")
	flag.Parse()
	if *pkg == "" || flag.NArg() != 0 {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(*pkg, *exclude, *out, *edges, *check); err != nil {
		fmt.Fprintln(os.Stderr, "refgraph:", err)
		os.Exit(1)
	}
}

func run(pkg, exclude, out, edges, check string) error {
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	mod, err := exec.Command("go", "list", "-m").Output()
	if err != nil {
		return fmt.Errorf("go list -m: %v", err)
	}
	importPath := strings.TrimSpace(string(mod)) + "/" + filepath.ToSlash(pkg)
	dir := filepath.Join(root, pkg)
	a, err := analyze(root, dir, importPath)
	if err != nil {
		return err
	}
	if edges != "" {
		var lines []string
		for r, n := range a.refs {
			lines = append(lines, fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s\t%d", r.useFile, r.useDecl, r.defFile, r.defDecl, r.kind, r.name, n))
		}
		if err := writeLines(edges, lines); err != nil {
			return err
		}
	}
	outside, err := scanOutside(root, root, dir, importPath)
	if err != nil {
		return err
	}
	var excluded []string
	if exclude != "" {
		excluded = strings.Split(exclude, ",")
	}
	c, err := classify(a, a.exports, outside, dir, importPath, excluded)
	if err != nil {
		return err
	}
	fmt.Printf("production files %d: core %d, research %d; core-to-research references %d\n", len(a.prod), len(c.Core), len(c.Research), c.CoreToResearch)
	fmt.Printf("in-package test files %d: research %d, mixed %d; external-package test files %d: research %d\n",
		len(a.tests), len(c.ResearchTests), len(c.MixedTests), len(a.xtests), len(c.ResearchXTests))
	fmt.Printf("files elsewhere naming a research symbol: %d\n", len(c.Outside))
	deps, err := classifyDependents(root, dir, importPath, c.Research)
	if err != nil {
		return err
	}
	var depFiles, depMoves []string
	for _, d := range deps {
		fmt.Printf("%s: %d research test files, %d mixed\n", d.Dir, len(d.ResearchTests), len(d.MixedTests))
		for _, f := range d.ResearchTests {
			depFiles = append(depFiles, d.Dir+"/"+f)
		}
		for _, m := range d.Moves {
			depMoves = append(depMoves, fmt.Sprintf("%s/%s\t%09d\t%d\t%s\t%t", d.Dir, m.File, m.From, m.To, m.Name, m.Partial))
		}
	}
	if check != "" {
		if err := checkTags(root, dir, check, c, deps); err != nil {
			return err
		}
		fmt.Printf("every research file carries //go:build %s and no other file does\n", check)
	}
	if out == "" {
		return nil
	}
	var moves []string
	for _, m := range c.Moves {
		moves = append(moves, fmt.Sprintf("%s\t%09d\t%d\t%s\t%t", m.File, m.From, m.To, m.Name, m.Partial))
	}
	for name, lines := range map[string][]string{
		"mixed-moves.tsv": moves, "dependent-tests.txt": depFiles, "dependent-moves.tsv": depMoves,
		"core-files.txt": c.Core, "research-files.txt": c.Research, "mixed-tests.txt": c.MixedTests,
		"research-tests.txt": append(append([]string{}, c.ResearchTests...), c.ResearchXTests...), "outside-files.txt": c.Outside,
	} {
		if err := writeLines(filepath.Join(out, name), lines); err != nil {
			return err
		}
	}
	return nil
}

func writeLines(path string, lines []string) error {
	sort.Strings(lines)
	text := strings.Join(lines, "\n")
	if text != "" {
		text += "\n"
	}
	return os.WriteFile(path, []byte(text), 0o644)
}

// checkTags verifies that the build tag marks exactly the research files: all
// research production and test files of the package, the research test files
// of its dependents, and nothing that is core. It also fails while any test
// file still mixes core and research declarations.
func checkTags(root, dir, tag string, c classification, deps []dependent) error {
	tagged := func(path string) (bool, error) {
		data, err := os.ReadFile(path)
		if err != nil {
			return false, err
		}
		first, _, _ := strings.Cut(string(data), "\n")
		return strings.TrimSpace(first) == "//go:build "+tag, nil
	}
	var problems []string
	expect := func(path string, want bool) error {
		got, err := tagged(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		if got && !want {
			problems = append(problems, rel+": tagged, but it is core")
		}
		if !got && want {
			problems = append(problems, rel+": research, but not tagged")
		}
		return nil
	}
	research := map[string]bool{}
	for _, group := range [][]string{c.Research, c.ResearchTests, c.ResearchXTests} {
		for _, f := range group {
			research[f] = true
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".go") {
			if err := expect(filepath.Join(dir, e.Name()), research[e.Name()]); err != nil {
				return err
			}
		}
	}
	for _, f := range c.MixedTests {
		problems = append(problems, filepath.Join(filepath.Base(dir), f)+": mixes core and research declarations")
	}
	for _, d := range deps {
		for _, f := range d.ResearchTests {
			if err := expect(filepath.Join(root, d.Dir, f), true); err != nil {
				return err
			}
		}
		for _, f := range d.MixedTests {
			problems = append(problems, d.Dir+"/"+f+": mixes core and research declarations")
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("%d files disagree with the classification:\n  %s", len(problems), strings.Join(problems, "\n  "))
	}
	return nil
}
