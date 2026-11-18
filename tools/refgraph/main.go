// Command refgraph measures how separable the files of one large package are.
// It type-checks the package with its in-package tests, records every
// cross-file reference to a package-level object, method or struct field, and
// partitions the files into those other packages reach and those they do not.
//
//	go run ./tools/refgraph -pkg brep/trimmed -exclude examples -out specs/brep-trimmed-split
//	go run ./tools/refgraph -pkg brep/trimmed -edges edges.tsv
//
// The rules are in specs/brep-trimmed-split/PLAN.md. Run it from the module
// root. -out writes core-files.txt, research-files.txt, research-tests.txt,
// mixed-tests.txt and outside-files.txt. -edges writes one line per
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
	flag.Parse()
	if *pkg == "" || flag.NArg() != 0 {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(*pkg, *exclude, *out, *edges); err != nil {
		fmt.Fprintln(os.Stderr, "refgraph:", err)
		os.Exit(1)
	}
}

func run(pkg, exclude, out, edges string) error {
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
	fmt.Printf("in-package test files %d: research %d (of which mixed %d); external-package test files %d: research %d\n",
		len(a.tests), len(c.ResearchTests), len(c.MixedTests), len(a.xtests), len(c.ResearchXTests))
	fmt.Printf("files elsewhere naming a research symbol: %d\n", len(c.Outside))
	if out == "" {
		return nil
	}
	for name, lines := range map[string][]string{
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
