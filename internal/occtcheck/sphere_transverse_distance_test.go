package occtcheck

import (
	"context"
	"encoding/json"
	"io"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
	"time"
)

func TestOCCTSphereTransverseNearest(t *testing.T) {
	program := os.Getenv("PLAINSOLID_OCCT_SPHERE_OVERLAP")
	if program == "" {
		t.Skip("set PLAINSOLID_OCCT_SPHERE_OVERLAP to pinned oracle")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "transverse.jsonl")
	if b, e := exec.CommandContext(ctx, program, path, "--transverse").CombinedOutput(); e != nil {
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
		var v row
		e := dec.Decode(&v)
		if e == io.EOF {
			break
		}
		if e != nil {
			t.Fatal(e)
		}
		if _, ok := refs[v.Name]; ok {
			t.Fatal("duplicate")
		}
		refs[v.Name] = v
	}
	if len(refs) != 7 {
		t.Fatal("records", len(refs))
	}
	for _, tc := range []struct {
		name                        string
		op                          geom.SphereSetOperation
		p                           [3]float64
		constant, root, coefficient float64
		rim                         bool
		owners                      geom.BoreBoundaryMask
	}{
		{"unionRim", geom.SphereSetUnion, [3]float64{2, 0, .5}, 99. / 4, 91. / 2, 2, true, 3},
		{"commonRim", geom.SphereSetIntersection, [3]float64{5, -3, .5}, 219. / 4, 728, 2, true, 3},
		{"cutRim", geom.SphereSetDifference, [3]float64{6, -2, 1}, 57, 728, 2, true, 3},
		{"unionFaces", geom.SphereSetUnion, [3]float64{5, -3, .5}, 237. / 4, 137. / 4, 10, false, 3},
		{"commonFaces", geom.SphereSetIntersection, [3]float64{2, 0, .5}, 117. / 4, 17. / 4, 10, false, 3},
		{"cutFace", geom.SphereSetDifference, [3]float64{2, 0, .5}, 117. / 4, 17. / 4, 10, false, 2},
	} {
		s, e := geom.NewSpherePairMaterial(geom.Vec3{}, 5, geom.Vec3{X: 2, Y: 2, Z: 1}, 5, tc.op)
		if e != nil {
			t.Fatal(e)
		}
		p := [3]*big.Rat{}
		for j := range p {
			p[j] = new(big.Rat).SetFloat64(tc.p[j])
		}
		unified, ue := s.NearestBoundaryRational(ctx, p, geom.SphereFullBoundaryDistanceOptions{})
		if ue != nil || unified.Kind != geom.SphereNearestTransverseIsolated {
			t.Fatal("unified domain", unified, ue)
		}
		g, e := s.NearestTransverseBoundaryRational(ctx, p, geom.SphereFullBoundaryDistanceOptions{})
		if e != nil || g.Owners != tc.owners || g.Candidates[2].Nearest != tc.rim {
			t.Fatal(tc.name, g, e)
		}
		expected := tc.constant - tc.coefficient*math.Sqrt(tc.root)
		ref, ok := refs[tc.name]
		tol := 3e-7 * math.Max(1, expected)
		if unified.Owners != g.Owners || unified.Lower.Cmp(g.Lower) != 0 || unified.Upper.Cmp(g.Upper) != 0 {
			t.Fatal("unified independent proof")
		}
		lo, _ := g.Lower.Float64()
		hi, _ := g.Upper.Float64()
		if !ok || ref.Solutions < 1 || math.Abs(ref.SquaredDistance-expected) > tol || ref.SquaredDistance < lo-tol || ref.SquaredDistance > hi+tol {
			t.Fatal("independent OCCT", tc.name, ref, expected, lo, hi)
		}
		d := 0.
		for j := range p {
			d += math.Pow(ref.Point[j]-tc.p[j], 2)
		}
		if math.Abs(d-ref.SquaredDistance) > tol {
			t.Fatal("independent witness")
		}
		if tc.rim {
			w := g.Candidates[2]
			S, _ := w.Radicand.Float64()
			for j := range p {
				c, _ := w.Center[j].Float64()
				n, _ := w.Numerator[j].Float64()
				if math.Abs(ref.Point[j]-c-n/math.Sqrt(S)) > 1e-6 {
					t.Fatal("isolated original seam foot", tc.name, ref)
				}
			}
		}
	}
	s, e := parametric.NewSpherePairSet(ctx, parametric.Ball{Center: geom.Vec3{X: -1}, Radius: 2}, parametric.Ball{Center: geom.Vec3{X: 1}, Radius: 2}, parametric.Union, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	g, e := s.NearestTransverseBoundaryRational(ctx, [3]*big.Rat{big.NewRat(0, 1), big.NewRat(1, 1), big.NewRat(1, 1)}, geom.SphereFullBoundaryDistanceOptions{})
	ref := refs["irrationalCircleFoot"]
	if e != nil || len(g.TiedFaces) != 2 || ref.Solutions < 1 || math.Abs(ref.SquaredDistance-(5-2*math.Sqrt(6))) > 3e-7 {
		t.Fatal("native irrational circle", g, e, ref)
	}
}
