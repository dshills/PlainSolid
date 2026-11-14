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

func TestOCCTSphereCavityNearest(t *testing.T) {
	program := os.Getenv("PLAINSOLID_OCCT_SPHERE_CAVITY")
	if program == "" {
		t.Skip("set PLAINSOLID_OCCT_SPHERE_CAVITY to pinned standalone oracle")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "cavity.jsonl")
	if b, e := exec.CommandContext(ctx, program, path).CombinedOutput(); e != nil {
		t.Fatalf("%v: %s", e, b)
	}
	f, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	type row struct {
		Name                          string
		SquaredDistance, Volume, Area float64
		Point                         [3]float64
		Shells, Solids, Solutions     int
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
			t.Fatal("duplicate reference")
		}
		refs[v.Name] = v
	}
	if len(refs) != 5 {
		t.Fatal("wrong oracle records")
	}
	body := refs["cavity"]
	if body.Shells != 2 || body.Solids != 1 || math.Abs(body.Volume-496*math.Pi/3) > 1e-6 || math.Abs(body.Area-104*math.Pi) > 1e-6 {
		t.Fatal("independent cavity geometry", body)
	}
	s, e := parametric.NewSpherePairSet(ctx, parametric.Ball{Radius: 5}, parametric.Ball{Center: geom.Vec3{X: 1}, Radius: 1}, parametric.Difference, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		name   string
		p      [3]float64
		owners geom.BoreBoundaryMask
	}{{"outer", [3]float64{3.5, .5, 0}, geom.SpherePairFirst}, {"inner", [3]float64{3.25, .5, 0}, geom.SpherePairSecond}, {"tie", [3]float64{3.5, 0, 0}, geom.SpherePairFirst | geom.SpherePairSecond}, {"center", [3]float64{1, 0, 0}, geom.SpherePairSecond}} {
		p := [3]*big.Rat{}
		for j := range p {
			p[j] = new(big.Rat).SetFloat64(tc.p[j])
		}
		got, e := s.NearestCavityBoundaryRational(ctx, p, geom.SphereCavityDistanceOptions{})
		if e != nil || got.Geometry.Owners != tc.owners {
			t.Fatal(tc.name, got, e)
		}
		expected := math.Min(math.Pow(math.Hypot(tc.p[0], tc.p[1])-5, 2), math.Pow(math.Hypot(tc.p[0]-1, tc.p[1])-1, 2))
		ref, ok := refs[tc.name]
		if !ok || ref.Solutions < 1 {
			t.Fatal("missing reference")
		}
		lo, _ := got.Geometry.Lower.Float64()
		hi, _ := got.Geometry.Upper.Float64()
		tol := 3e-7 * math.Max(1, expected)
		if ref.SquaredDistance < lo-tol || ref.SquaredDistance > hi+tol || math.Abs(ref.SquaredDistance-expected) > tol {
			t.Fatal("independent boundary distance", tc.name, ref, lo, hi)
		}
		d := 0.
		for j := range p {
			d += math.Pow(ref.Point[j]-tc.p[j], 2)
		}
		if math.Abs(d-ref.SquaredDistance) > tol {
			t.Fatal("reference witness")
		}
		if tc.name == "tie" && len(got.TiedFaces) != 2 {
			t.Fatal("native original two owners")
		}
		if tc.name == "center" && !got.Geometry.Candidates[1].WholeSphere {
			t.Fatal("exact full minimizer family")
		}
		// Finite OCCT representatives do not establish exhaustive ties or continua.
	}
}
