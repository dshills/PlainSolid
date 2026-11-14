package occtcheck

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"testing"
	"time"
)

// Independent OCCT cone+native fillet+cut construction, never Go mesh input.
func TestOCCTConicalNearestBoundary(t *testing.T) {
	program := os.Getenv("PLAINSOLID_OCCT_CONICAL_FILLET")
	if program == "" {
		t.Skip("set PLAINSOLID_OCCT_CONICAL_FILLET to rebuilt standalone oracle")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "nearest.jsonl")
	if b, e := exec.CommandContext(ctx, program, path, "--nearest").CombinedOutput(); e != nil {
		t.Fatalf("%v: %s", e, b)
	}
	f, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	type row struct {
		Name            string
		SquaredDistance float64
		Point           [3]float64
		Solutions       int
	}
	refs := map[string]row{}
	dec := json.NewDecoder(f)
	for {
		var r row
		e := dec.Decode(&r)
		if e == io.EOF {
			break
		}
		if e != nil {
			t.Fatal(e)
		}
		if _, ok := refs[r.Name]; ok {
			t.Fatal("duplicate reference")
		}
		refs[r.Name] = r
	}
	if len(refs) != 14 {
		t.Fatal("wrong oracle version/records", len(refs))
	}
	o := geom.SurfacePatchOptions{}
	w, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 8, 6, o)
	if e != nil {
		t.Fatal(e)
	}
	w, _, e = geom.DraftWrappedFrustum(ctx, w, 0, 8, o)
	if e != nil {
		t.Fatal(e)
	}
	s, _, e := trimmed.NewWrappedFrustum(ctx, w, 1e-9, o)
	if e != nil {
		t.Fatal(e)
	}
	q, e := trimmed.PrepareFilletedFrustumMaterial(ctx, s, 15./16, 15./8, []trimmed.FilletedFrustumBoreCut{{Center: geom.Vec2{X: -2}, Radius: .5}, {Center: geom.Vec2{X: 2}, Radius: .5, Depth: 1}, {Center: geom.Vec2{X: 2}, Radius: .5, Depth: 4, FromUpper: true}}, o)
	if e != nil {
		t.Fatal(e)
	}
	var encoded bytes.Buffer
	if e = trimmed.WriteFilletedFrustumMaterialJSON(ctx, &encoded, q); e != nil {
		t.Fatal(e)
	}
	q, e = trimmed.ReadFilletedFrustumMaterialJSON(ctx, &encoded, trimmed.FilletedFrustumMaterialJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	r := big.NewRat
	options := trimmed.FilletedFrustumNestedDistanceOptions{Original: trimmed.FilletedFrustumToroidDistanceOptions{Rims: trimmed.FilletedFrustumRimDistanceOptions{Support: trimmed.FilletedFrustumDistanceOptions{SquaredTolerance: r(1, 1000000000000)}}}}
	for _, tc := range []struct {
		name  string
		p     [3]*big.Rat
		kind  trimmed.FilletedFrustumClosestKind
		truth float64
	}{
		{"nearest-nested-torus", [3]*big.Rat{r(8, 1), r(1, 1), r(3, 16)}, trimmed.FilletedFrustumClosestNestedToroid, math.Pow(math.Hypot(math.Sqrt(65)-123./16, -.75)-15./16, 2)},
		{"nearest-torus", [3]*big.Rat{r(77, 16), r(77, 12), r(3, 16)}, trimmed.FilletedFrustumClosestToroid, 3577./2304 - 5*math.Sqrt(97)/32},
		{"nearest-cone", [3]*big.Rat{r(10, 1), r(1, 1), r(3, 2)}, trimmed.FilletedFrustumClosestCone, math.Pow(math.Sqrt(101)-10, 2) * 9 / 25},
		{"nearest-bore", [3]*big.Rat{r(-19, 10), r(1, 10), r(3, 1)}, trimmed.FilletedFrustumClosestBore, .27 - math.Sqrt(.02)},
		{"nearest-floor", [3]*big.Rat{r(21, 10), r(1, 10), r(99, 100)}, trimmed.FilletedFrustumClosestRational, .0001},
		{"nearest-entry-rim", [3]*big.Rat{r(-19, 10), r(1, 10), r(-1, 8)}, trimmed.FilletedFrustumClosestRim, 457./1600 - math.Sqrt(.02)},
		{"nearest-floor-rim", [3]*big.Rat{r(5, 2), r(1, 2), r(9, 8)}, trimmed.FilletedFrustumClosestRim, 49./64 - math.Sqrt(.5)},
		{"nearest-meridian-family", [3]*big.Rat{r(369, 80), r(123, 20), r(15, 16)}, trimmed.FilletedFrustumClosestRational, 225. / 256},
		{"nearest-axis-rim-family", [3]*big.Rat{r(-2, 1), r(0, 1), r(-1, 8)}, trimmed.FilletedFrustumClosestRational, 17. / 64},
	} {
		reference, ok := refs[tc.name]
		if !ok || reference.Solutions < 1 || math.IsNaN(reference.SquaredDistance) || math.IsInf(reference.SquaredDistance, 0) {
			t.Fatal("invalid nearest reference", tc.name)
		}
		got, e := q.ClosestBoundaryRational(ctx, tc.p, options)
		if e != nil || got.Kind != tc.kind || !got.NearestProven || !got.Converged {
			t.Fatal(tc.name, got.Kind, e)
		}
		lo, _ := got.Lower.Float64()
		hi, _ := got.Upper.Float64()
		// OCCT is finite precision: absolute/relative squared distance allowance
		//3e-7*max(1,D), separate from the exact Go1e-12 interval contract.
		tolerance := 3e-7 * math.Max(1, tc.truth)
		if reference.SquaredDistance < lo-tolerance || reference.SquaredDistance > hi+tolerance || math.Abs(reference.SquaredDistance-tc.truth) > tolerance || tc.truth < lo-1e-12 || tc.truth > hi+1e-12 {
			t.Fatalf("%s OCCT %.17g exact[%g,%g] analytic%.17g", tc.name, reference.SquaredDistance, lo, hi, tc.truth)
		}
		measured := 0.
		for i, x := range reference.Point {
			v, _ := tc.p[i].Float64()
			measured += (x - v) * (x - v)
		}
		if math.Abs(measured-reference.SquaredDistance) > tolerance {
			t.Fatal("OCCT witness/value inconsistency", tc.name)
		}
		width, _ := new(big.Rat).Sub(got.Upper, got.Lower).Float64()
		t.Logf("%s D=%g exactWidth=%g OCCTsolutions=%d", tc.name, reference.SquaredDistance, width, reference.Solutions)
	}
}
