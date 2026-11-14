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

func TestOCCTSphereOverlapAlgebraicNearest(t *testing.T) {
	program := os.Getenv("PLAINSOLID_OCCT_SPHERE_OVERLAP")
	if program == "" {
		t.Skip("set PLAINSOLID_OCCT_SPHERE_OVERLAP to pinned oracle")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "algebraic.jsonl")
	if b, e := exec.CommandContext(ctx, program, path, "--algebraic").CombinedOutput(); e != nil {
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
	for _, tc := range []struct {
		name                            string
		op                              parametric.BooleanOp
		x, y, z                         int64
		constant, coefficient, radicand float64
		rim                             bool
		owners                          geom.BoreBoundaryMask
	}{
		{"unionRim", parametric.Union, 0, 2, 1, 21, 8, 5, true, 3},
		{"commonRim", parametric.Intersection, 0, 7, 1, 66, 8, 50, true, 3},
		{"cutRim", parametric.Difference, 3, 4, 1, 42, 8, 17, true, 3},
		{"unionFaces", parametric.Union, 0, 7, 1, 84, 10, 59, false, 3},
		{"commonFaces", parametric.Intersection, 0, 2, 1, 39, 10, 14, false, 3},
		{"cutFace", parametric.Difference, 0, 2, 1, 39, 10, 14, false, 2},
	} {
		s, e := parametric.NewSpherePairSet(ctx, parametric.Ball{Center: geom.Vec3{X: -3}, Radius: 5}, parametric.Ball{Center: geom.Vec3{X: 3}, Radius: 5}, tc.op, 1e-9)
		if e != nil {
			t.Fatal(e)
		}
		p := [3]*big.Rat{big.NewRat(tc.x, 1), big.NewRat(tc.y, 1), big.NewRat(tc.z, 1)}
		g, e := s.NearestOverlapAlgebraicBoundaryRational(ctx, p, geom.SphereFullBoundaryDistanceOptions{})
		if e != nil || g.Geometry.Owners != tc.owners || g.Geometry.Candidates[2].Nearest != tc.rim {
			t.Fatal(tc.name, g, e)
		}
		expected := tc.constant - tc.coefficient*math.Sqrt(tc.radicand)
		ref, ok := refs[tc.name]
		tol := 3e-7 * math.Max(1, expected)
		lo, _ := g.Geometry.Lower.Float64()
		hi, _ := g.Geometry.Upper.Float64()
		if !ok || ref.Solutions < 1 || math.Abs(ref.SquaredDistance-expected) > tol || ref.SquaredDistance < lo-tol || ref.SquaredDistance > hi+tol {
			t.Fatal("independent OCCT", tc.name, ref, expected, lo, hi)
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
			rho := math.Hypot(float64(tc.y), float64(tc.z))
			if len(g.TiedFaces) != 2 || math.Abs(ref.Point[0]) > 1e-6 || math.Abs(ref.Point[1]-4*float64(tc.y)/rho) > 1e-6 || math.Abs(ref.Point[2]-4*float64(tc.z)/rho) > 1e-6 {
				t.Fatal("isolated algebraic seam foot", ref, g)
			}
		}
	}
}
