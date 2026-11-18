package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// The fixture covers a file made core by closure, a method that is core only
// because another package calls it, a method that is core only because another
// package uses the type as an interface, a method on a core type that nothing
// outside calls, a helper file shared by core and research
// tests, a test that reaches research code only through two other files,
// external-package tests, and an excluded examples root.
func TestClassifyFixture(t *testing.T) {
	root, err := filepath.Abs("testdata/fixture")
	if err != nil {
		t.Fatal(err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	const path = "plainsolid/tools/refgraph/testdata/fixture/shape"
	dir := filepath.Join(root, "shape")
	a, err := analyze(wd, dir, path)
	if err != nil {
		t.Fatal(err)
	}
	outside, err := scanOutside(wd, root, dir, path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := classify(a, a.exports, outside, dir, path, []string{"examples"})
	if err != nil {
		t.Fatal(err)
	}
	want := classification{
		Core:           []string{"core.go", "iface_method.go", "method_core.go", "scale.go"},
		Research:       []string{"research.go", "research2.go", "research_method.go"},
		ResearchTests:  []string{"chain_a_test.go", "chain_b_test.go", "looks_core_test.go"},
		MixedTests:     []string{"helpers_test.go"},
		ResearchXTests: []string{"ext_test.go"},
		Outside:        []string{"consumer/use_test.go", "examples/demo/main.go"},
	}
	if len(got.Moves) != 1 || got.Moves[0].File != "helpers_test.go" || got.Moves[0].Name != "mkFancy" || got.Moves[0].Partial {
		t.Fatalf("moves: got %+v, want mkFancy out of helpers_test.go", got.Moves)
	}
	got.Moves = nil
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("classification\n got %+v\nwant %+v", got, want)
	}

	// Without the exclusion the example is a root and Fancy becomes core.
	all, err := classify(a, a.exports, outside, dir, path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"research_method.go"}; !reflect.DeepEqual(all.Research, want) {
		t.Fatalf("research without exclusion: got %v, want %v", all.Research, want)
	}
	if len(all.ResearchTests) != 0 || len(all.ResearchXTests) != 0 || len(all.Outside) != 0 {
		t.Fatalf("no test should be research without exclusion: %+v", all)
	}
}
