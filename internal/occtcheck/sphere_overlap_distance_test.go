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

func TestOCCTSphereOverlapNearest(t *testing.T) {
	program := os.Getenv("PLAINSOLID_OCCT_SPHERE_OVERLAP")
	if program == "" {
		t.Skip("set PLAINSOLID_OCCT_SPHERE_OVERLAP to pinned oracle")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "overlap.jsonl")
	if b, e := exec.CommandContext(ctx, program, path).CombinedOutput(); e != nil {
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
	if len(refs) != 6 {
		t.Fatal("records", len(refs))
	}
	r := big.NewRat
	for _, tc := range []struct {
		name        string
		op          parametric.BooleanOp
		x, y, truth *big.Rat
		rim         bool
		owners      geom.BoreBoundaryMask
	}{
		{"unionRim", parametric.Union, r(0, 1), r(9, 4), r(49, 16), true, 3},
		{"commonRim", parametric.Intersection, r(0, 1), r(36, 5), r(256, 25), true, 3},
		{"cutRim", parametric.Difference, r(3, 1), r(9, 2), r(37, 4), true, 3},
		{"unionFaces", parametric.Union, r(0, 1), r(36, 5), r(196, 25), false, 3},
		{"commonFaces", parametric.Intersection, r(0, 1), r(9, 4), r(25, 16), false, 3},
		{"cutFace", parametric.Difference, r(0, 1), r(9, 4), r(25, 16), false, 2},
	} {
		s, e := parametric.NewSpherePairSet(ctx, parametric.Ball{Center: geom.Vec3{X: -3}, Radius: 5}, parametric.Ball{Center: geom.Vec3{X: 3}, Radius: 5}, tc.op, 1e-9)
		if e != nil {
			t.Fatal(e)
		}
		p := [3]*big.Rat{tc.x, tc.y, r(0, 1)}
		g, e := s.NearestOverlapBoundaryRational(ctx, p, geom.SurfacePatchOptions{})
		if e != nil || g.Geometry.SquaredDistance.Cmp(tc.truth) != 0 || g.Geometry.Owners != tc.owners || g.Geometry.Candidates[2].Nearest != tc.rim {
			t.Fatal(tc.name, g, e)
		}
		ref, ok := refs[tc.name]
		expected, _ := tc.truth.Float64()
		tol := 3e-7 * math.Max(1, expected)
		if !ok || ref.Solutions < 1 || math.Abs(ref.SquaredDistance-expected) > tol {
			t.Fatal("independent OCCT", tc.name, ref, expected)
		}
		d := 0.
		for j := range p {
			v, _ := p[j].Float64()
			d += math.Pow(ref.Point[j]-v, 2)
		}
		if math.Abs(d-ref.SquaredDistance) > tol {
			t.Fatal("independent witness")
		}
		if tc.rim {
			if len(g.TiedFaces) != 2 || math.Abs(ref.Point[0]) > 1e-6 || math.Abs(ref.Point[1]-4) > 1e-6 || math.Abs(ref.Point[2]) > 1e-6 {
				t.Fatal("isolated seam foot", ref, g)
			}
		}
	}
}
