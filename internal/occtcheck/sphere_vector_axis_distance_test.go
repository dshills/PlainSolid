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
	"plainsolid/geom"
	"testing"
	"time"
)

func TestOCCTSphereVectorAxisFamilies(t *testing.T) {
	program := os.Getenv("PLAINSOLID_OCCT_SPHERE_OVERLAP")
	if program == "" {
		t.Skip("set PLAINSOLID_OCCT_SPHERE_OVERLAP to pinned oracle")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "vector-axis.jsonl")
	if b, e := exec.CommandContext(ctx, program, path, "--vector-axis").CombinedOutput(); e != nil {
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
	if len(refs) != 8 {
		t.Fatal("records", len(refs))
	}
	for _, tc := range []struct {
		name                        string
		op                          geom.SphereSetOperation
		x                           int64
		constant, coefficient, root float64
		kind                        geom.SphereAxisMinimumKind
		slot                        int
	}{
		{"unionCapA", geom.SphereSetUnion, 0, 25, 0, 0, geom.SphereAxisRetainedCap, 0},
		{"unionCapB", geom.SphereSetUnion, 2, 25, 0, 0, geom.SphereAxisRetainedCap, 1},
		{"unionCircle", geom.SphereSetUnion, 1, 22, 0, 0, geom.SphereAxisSharedCircle, 2},
		{"unionPole", geom.SphereSetUnion, -2, 37, 10, 12, geom.SphereAxisPole, 0},
		{"commonPoles", geom.SphereSetIntersection, 1, 28, 10, 3, geom.SphereAxisPole, 0},
		{"commonCenter", geom.SphereSetIntersection, 0, 37, 10, 12, geom.SphereAxisPole, 1},
		{"cutCap", geom.SphereSetDifference, 2, 25, 0, 0, geom.SphereAxisRetainedCap, 1},
		{"cutCenter", geom.SphereSetDifference, 0, 37, 10, 12, geom.SphereAxisPole, 1},
	} {
		s, e := geom.NewSpherePairMaterial(geom.Vec3{}, 5, geom.Vec3{X: 2, Y: 2, Z: 2}, 5, tc.op)
		if e != nil {
			t.Fatal(e)
		}
		p := [3]*big.Rat{big.NewRat(tc.x, 1), big.NewRat(tc.x, 1), big.NewRat(tc.x, 1)}
		g, e := s.NearestTransverseAxisBoundaryRational(ctx, p, geom.SphereFullBoundaryDistanceOptions{})
		if e != nil || g.Families[tc.slot].Kind != tc.kind || !g.Families[tc.slot].Nearest {
			t.Fatal(tc.name, g, e)
		}
		expected := tc.constant - tc.coefficient*math.Sqrt(tc.root)
		ref, ok := refs[tc.name]
		tol := 3e-7 * math.Max(1, expected)
		lo, _ := g.Lower.Float64()
		hi, _ := g.Upper.Float64()
		if !ok || ref.Solutions < 1 || math.Abs(ref.SquaredDistance-expected) > tol || ref.SquaredDistance < lo-tol || ref.SquaredDistance > hi+tol {
			t.Fatal("independent distance", tc.name, ref, expected)
		}
		d := 0.
		for _, v := range ref.Point {
			d += math.Pow(v-float64(tc.x), 2)
		}
		if math.Abs(d-ref.SquaredDistance) > tol {
			t.Fatal("independent representative")
		}
		// OCCT confirms distance and one foot, never completeness of an infinite set.
		if tc.kind == geom.SphereAxisSharedCircle {
			dot, norm := 0., 0.
			for _, v := range ref.Point {
				dot += 2 * (v - 1)
				norm += (v - 1) * (v - 1)
			}
			if math.Abs(dot) > 1e-6 || math.Abs(norm-22) > 1e-6 {
				t.Fatal("circle representative", ref)
			}
		}
	}
}
