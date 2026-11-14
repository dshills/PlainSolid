package occtcheck

import (
	"context"
	"encoding/json"
	"errors"
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

func TestOCCTSphereSingleNearest(t *testing.T) {
	program := os.Getenv("PLAINSOLID_OCCT_SPHERE_SINGLE")
	if program == "" {
		t.Skip("set PLAINSOLID_OCCT_SPHERE_SINGLE to pinned oracle")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "single.jsonl")
	if b, e := exec.CommandContext(ctx, program, path).CombinedOutput(); e != nil {
		t.Fatalf("%v: %s", e, b)
	}
	f, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	type row struct {
		Name              string
		SquaredDistance   float64
		Point             [3]float64
		Solutions, Solids int
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
		name       string
		ra, rb, bx float64
		op         parametric.BooleanOp
		p          [3]float64
		truth      float64
		owners     geom.BoreBoundaryMask
		family     bool
	}{
		{"strictUnion", 5, 1, 1, parametric.Union, [3]float64{1, 1, 0}, 27 - 10*math.Sqrt(2), 1, false},
		{"internalUnion", 5, 1, 4, parametric.Union, [3]float64{5, 0, 0}, 0, 1, false},
		{"internalCommon", 5, 1, 4, parametric.Intersection, [3]float64{4, 0, 0}, 1, 2, true},
		{"identicalUnion", 5, 5, 0, parametric.Union, [3]float64{1, 1, 0}, 27 - 10*math.Sqrt(2), 3, false},
		{"identicalCommon", 5, 5, 0, parametric.Intersection, [3]float64{0, 0, 0}, 25, 3, true},
		{"externalCut", 2, 2, 4, parametric.Difference, [3]float64{2, 0, 0}, 0, 1, false},
		{"disjointCut", 2, 1, 5, parametric.Difference, [3]float64{0, 0, 0}, 4, 1, true},
	} {
		s, e := parametric.NewSpherePairSet(ctx, parametric.Ball{Radius: tc.ra}, parametric.Ball{Center: geom.Vec3{X: tc.bx}, Radius: tc.rb}, tc.op, 1e-9)
		if e != nil {
			t.Fatal(e)
		}
		p := [3]*big.Rat{}
		for j := range p {
			p[j] = new(big.Rat).SetFloat64(tc.p[j])
		}
		g, e := s.NearestBoundaryRational(ctx, p, geom.SphereFullBoundaryDistanceOptions{})
		if e != nil || g.Geometry.Kind != geom.SphereNearestSingle || g.Geometry.Owners != tc.owners || g.Geometry.Single.Witness.WholeSphere != tc.family || len(g.TiedFaces) != 1 {
			t.Fatal(tc.name, g, e)
		}
		ref, ok := refs[tc.name]
		tol := 3e-7 * math.Max(1, tc.truth)
		lo, _ := g.Geometry.Lower.Float64()
		hi, _ := g.Geometry.Upper.Float64()
		if !ok || ref.Solutions < 1 || math.Abs(ref.SquaredDistance-tc.truth) > tol || ref.SquaredDistance < lo-tol || ref.SquaredDistance > hi+tol {
			t.Fatal("independent distance", tc.name, ref)
		}
		d := 0.
		for j, v := range ref.Point {
			d += math.Pow(v-tc.p[j], 2)
		}
		if math.Abs(d-ref.SquaredDistance) > tol {
			t.Fatal("independent foot")
		}
	}
	for _, tc := range []struct {
		name       string
		ra, rb, bx float64
		op         geom.SphereSetOperation
	}{{"emptyCommon", 2, 2, 4, geom.SphereSetIntersection}, {"emptyCut", 5, 5, 0, geom.SphereSetDifference}} {
		s, _ := geom.NewSpherePairMaterial(geom.Vec3{}, tc.ra, geom.Vec3{X: tc.bx}, tc.rb, tc.op)
		g, e := s.NearestBoundaryRational(ctx, [3]*big.Rat{big.NewRat(0, 1), big.NewRat(0, 1), big.NewRat(0, 1)}, geom.SphereFullBoundaryDistanceOptions{})
		ref, ok := refs[tc.name]
		if !ok || ref.Solids != 0 || !errors.Is(e, geom.ErrDomain) || g.Lower != nil {
			t.Fatal("no boundary of empty regularized result", g, e, ref)
		}
	}
}
