package occtcheck

import (
	"bufio"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// This independent analytic/topology qualification checks the OCCT fixture.
// The native operation's separate race test checks the matching two-loop shell
// arrangement; this test does not claim a direct native curved-mass comparison.
func TestOCCTTwoContourReference(t *testing.T) {
	program := os.Getenv("PLAINSOLID_OCCT_ORACLE")
	if program == "" {
		t.Skip("set PLAINSOLID_OCCT_ORACLE to optional reference executable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "two.jsonl")
	if out, e := exec.CommandContext(ctx, program, path, "two-contour-spline-shell").CombinedOutput(); e != nil {
		t.Fatalf("%v: %s", e, out)
	}
	f, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	type result struct {
		record
		Shells, Euler int
	}
	refs := map[string]result{}
	scan := bufio.NewScanner(f)
	for scan.Scan() {
		var r result
		if e = json.Unmarshal(scan.Bytes(), &r); e != nil {
			t.Fatal(e)
		}
		if _, ok := refs[r.Name]; ok {
			t.Fatal("duplicate")
		}
		refs[r.Name] = r
	}
	if e = scan.Err(); e != nil {
		t.Fatal(e)
	}
	if len(refs) != 8 {
		t.Fatal("reference count")
	}
	overlap := math.Pi * math.Pow(5./32, 4)
	base := 7. / 8 * .5 * (1 + 25./1024 - 49./3072 - 1./48)
	for _, tc := range []struct {
		name          string
		volume        float64
		shells, euler int
	}{{"two-b", base, 1, 2}, {"two-common", overlap, 2, 4}, {"two-union", 1 + base - overlap, 1, 0}, {"two-cut", 1 - overlap, 1, 2}} {
		r := refs[tc.name]
		if math.Abs(r.Volume-tc.volume) > 1e-8 || r.Shells != tc.shells || refs[tc.name+"-topology"].Euler != tc.euler || r.Area <= 0 {
			t.Fatalf("%s: %+v want volume%.15g shells%d Euler%d", tc.name, r, tc.volume, tc.shells, tc.euler)
		}
		if math.Abs(r.Centroid[0]-.5) > 1e-7 || math.Abs(r.Centroid[1]-.5) > 1e-7 {
			t.Fatal("symmetry", r)
		}
	}
	if math.Abs(refs["two-common"].Centroid[2]-math.Pow(5./32, 2)/3) > 1e-8 {
		t.Fatal("overlap centroid")
	}
}
