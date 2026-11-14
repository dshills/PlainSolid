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

func TestOCCTSphereOverlapAxisFamilies(t *testing.T) {
	program := os.Getenv("PLAINSOLID_OCCT_SPHERE_OVERLAP")
	if program == "" {
		t.Skip("set PLAINSOLID_OCCT_SPHERE_OVERLAP to pinned oracle")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "axis.jsonl")
	if b, e := exec.CommandContext(ctx, program, path, "--axis").CombinedOutput(); e != nil {
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
	if len(refs) != 9 {
		t.Fatal("records", len(refs))
	}
	for _, tc := range []struct {
		name                 string
		op                   parametric.BooleanOp
		x, d, offset, radius int64
		kind                 geom.SphereAxisMinimumKind
		slot                 int
	}{
		{"unionCapA", parametric.Union, -3, 25, 3, 5, geom.SphereAxisRetainedCap, 0},
		{"unionCapB", parametric.Union, 3, 25, 3, 5, geom.SphereAxisRetainedCap, 1},
		{"unionCircle", parametric.Union, 0, 16, 3, 5, geom.SphereAxisSharedCircle, 2},
		{"unionPole", parametric.Union, -10, 4, 3, 5, geom.SphereAxisPole, 0},
		{"commonPoles", parametric.Intersection, 0, 4, 3, 5, geom.SphereAxisPole, 0},
		{"commonCenter", parametric.Intersection, -3, 1, 3, 5, geom.SphereAxisPole, 1},
		{"cutCap", parametric.Difference, 3, 25, 3, 5, geom.SphereAxisRetainedCap, 1},
		{"cutCenter", parametric.Difference, -3, 1, 3, 5, geom.SphereAxisPole, 1},
		{"irrationalCircle", parametric.Union, 0, 3, 1, 2, geom.SphereAxisSharedCircle, 2},
	} {
		s, e := parametric.NewSpherePairSet(ctx, parametric.Ball{Center: geom.Vec3{X: -float64(tc.offset)}, Radius: float64(tc.radius)}, parametric.Ball{Center: geom.Vec3{X: float64(tc.offset)}, Radius: float64(tc.radius)}, tc.op, 1e-9)
		if e != nil {
			t.Fatal(e)
		}
		p := [3]*big.Rat{big.NewRat(tc.x, 1), big.NewRat(0, 1), big.NewRat(0, 1)}
		g, e := s.NearestOverlapAxisBoundaryRational(ctx, p, geom.SurfacePatchOptions{})
		if e != nil || g.Geometry.SquaredDistance.Cmp(big.NewRat(tc.d, 1)) != 0 || g.Geometry.Families[tc.slot].Kind != tc.kind || !g.Geometry.Families[tc.slot].Nearest {
			t.Fatal(tc.name, g, e)
		}
		ref, ok := refs[tc.name]
		tol := 3e-7 * math.Max(1, float64(tc.d))
		if !ok || ref.Solutions < 1 || math.Abs(ref.SquaredDistance-float64(tc.d)) > tol {
			t.Fatal("independent distance", tc.name, ref)
		}
		d := math.Pow(ref.Point[0]-float64(tc.x), 2) + ref.Point[1]*ref.Point[1] + ref.Point[2]*ref.Point[2]
		if math.Abs(d-ref.SquaredDistance) > tol {
			t.Fatal("independent representative")
		}
		// OCCT only provides representative feet; it does not prove family completeness.
		if tc.kind == geom.SphereAxisSharedCircle {
			if math.Abs(ref.Point[0]) > 1e-6 || math.Abs(ref.Point[1]*ref.Point[1]+ref.Point[2]*ref.Point[2]-float64(tc.radius*tc.radius-tc.offset*tc.offset)) > 1e-6 {
				t.Fatal("circle representative", ref)
			}
		}
	}
}
