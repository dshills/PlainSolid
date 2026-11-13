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

// Independent OCCT qualification of a full left lobe and half right lobe.
// Native mixed-shell operation tests separately check the same analytic volume
// and topology. This does not claim exact native curved-mass integration.
func TestOCCTMixedSplineShellReference(t *testing.T) {
	program := os.Getenv("PLAINSOLID_OCCT_ORACLE")
	if program == "" {
		t.Skip("set PLAINSOLID_OCCT_ORACLE to optional reference executable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "mixed.jsonl")
	if out, e := exec.CommandContext(ctx, program, path, "mixed-spline-shell").CombinedOutput(); e != nil {
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
	r := 5. / 32
	overlap := .75 * math.Pi * math.Pow(r, 4)
	base := 7. / 8 * .5 * (1 + 25./1024 - 49./3072 - 1./48)
	for _, tc := range []struct {
		name          string
		volume        float64
		shells, euler int
	}{{"mixed-b", base, 1, 2}, {"mixed-common", overlap, 2, 4}, {"mixed-union", 23./32 + base - overlap, 1, 0}, {"mixed-cut", 23./32 - overlap, 1, 2}} {
		r := refs[tc.name]
		if math.Abs(r.Volume-tc.volume) > 1e-8 || r.Shells != tc.shells || refs[tc.name+"-topology"].Euler != tc.euler || r.Area <= 0 {
			t.Fatalf("%s: %+v want volume%.15g shells%d Euler%d", tc.name, r, tc.volume, tc.shells, tc.euler)
		}
		if math.Abs(r.Centroid[1]-.5) > 1e-7 {
			t.Fatal("Y symmetry", r)
		}

	}
	common := refs["mixed-common"]
	wantX := 41./96 - 16*r/(45*math.Pi)
	wantArea := 1.5*math.Pi*r*r + math.Pi/4*(math.Pow(1+4*r*r, 1.5)-1) + 4*r*r*r/3
	if math.Abs(common.Centroid[0]-wantX) > 1e-6 || math.Abs(common.Centroid[2]-r*r/3) > 1e-8 || math.Abs(common.Area-wantArea) > 1e-7 {
		t.Fatal("analytic half-lobe moments/area", common, wantX, wantArea)
	}
}
