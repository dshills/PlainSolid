package intersect

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"testing"

	"plainsolid/geom"
)

func TestTubeBoundaryAnchorRationalRecipe(t *testing.T) {
	r := [4]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 3), big.NewRat(1, 2), big.NewRat(1, 3)}
	a, err := NewTubeBoundaryAnchor(true, r)
	if err != nil {
		t.Fatal(err)
	}
	r[1].SetInt64(99)
	data, err := json.Marshal(a)
	if err != nil || string(data) != `{"AtUpper":true,"RationalParameters":["1/2","1/3","1/2","1/3"]}` {
		t.Fatal(string(data), err)
	}
	var back TubeBoundaryAnchor
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		out, err := back.ExactOptions(geom.SurfacePairTubeOptions{FreeAxis: 2})
		if err != nil || !out.AtUpper || out.FreeAxis != 2 || out.Anchor[1].Cmp(big.NewRat(1, 3)) != 0 {
			t.Fatal("exact rational anchor provenance", out, err)
		}
		out.Anchor[1].SetInt64(5)
	}
	for _, raw := range []string{
		`{"Parameters":[0,0,0,0],"RationalParameters":["0","1/3","0","1/3"]}`,
		`{"RationalParameters":["0","2/6","0","1/3"]}`,
		`{"RationalParameters":["0","1e99999999","0","1/3"]}`,
		`{"RationalParameters":["0","1/3","0"]}`,
		`{"Parameters":[0,null,0,0]}`,
		`{"Parameters":[0,0,0,0],"Proven":true}`,
	} {
		if err := json.Unmarshal([]byte(raw), &back); err == nil {
			t.Fatal("accepted invalid proposal", raw)
		}
	}
	a.Parameters[1] = .25
	if _, err := a.ExactOptions(geom.SurfacePairTubeOptions{}); err == nil {
		t.Fatal("accepted ambiguous coordinate encodings")
	}
	legacy, err := NewTubeBoundaryAnchor(false, [4]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 4), big.NewRat(1, 8), new(big.Rat)})
	if err != nil {
		t.Fatal(err)
	}
	data, err = json.Marshal(legacy)
	if err != nil || string(data) != `{"AtUpper":false,"Parameters":[0.5,0.25,0.125,0]}` {
		t.Fatal("changed legacy anchor recipe", string(data), err)
	}
}

func TestBoundaryAnchoredSplinePairFullCoverageAndRecipe(t *testing.T) {
	ctx := context.Background()
	makePatch := func(p []geom.Vec3, x0, x1, y0, y1 float64) Patch {
		t.Helper()
		s, e := geom.NewBSplineSurface(1, 1, 2, 2, p,
			[]float64{x0, x0, x1, x1}, []float64{y0, y0, y1, y1}, nil)
		if e != nil {
			t.Fatal(e)
		}
		return Patch{Surface: s, Min: geom.Vec2{X: x0, Y: y0}, Max: geom.Vec2{X: x1, Y: y1}}
	}
	a := makePatch([]geom.Vec3{{}, {Y: .25}, {X: .125}, {X: .125, Y: .25, Z: 1.0 / 32}}, 0, .125, 0, .25)
	b := makePatch([]geom.Vec3{{X: -.25, Z: -.25}, {X: -.25, Y: .25, Z: -.5}, {X: .25, Z: .25}, {X: .25, Y: .25}}, -.25, .25, 0, .25)
	da, db := geom.ParameterRectangle{UMax: .125, VMax: .25}, geom.ParameterRectangle{UMin: -.25, UMax: .25, VMax: .25}
	anchor := &TubeBoundaryAnchor{}
	c := geom.SurfaceTubeCertificate{FreeAxis: 0, ProvenBranch: true, FreeParameter: geom.CertifiedScalarRange{Min: 0, Max: .125}}
	s := SplineSeedReport{Attempts: []SplineSeedCell{{Cell: geom.SurfaceSurfaceCell{DomainA: da, DomainB: db}, Tube: &c, BoundaryAnchor: anchor}}}
	o := SplinePairTraceOptions{Coverage: SplineTubeCoverageOptions{Isolation: geom.SurfaceSurfaceIsolationOptions{ParameterTolerance: 1}}}
	r, e := CertifySplinePairBranches(ctx, a, b, s, o)
	if e != nil || !r.Complete || !r.Coverage.CoversAllContacts || len(r.Curves) != 1 || len(r.Curves[0]) != 1 {
		t.Fatal(r.Issues, r.Coverage.Unresolved, e)
	}
	seg := r.Curves[0][0]
	first, e := seg.Evaluate(0)
	if e != nil {
		t.Fatal(e)
	}
	end := 0
	if first.Point != (geom.Vec3{}) {
		end = 1
	}
	var h [4]*big.Rat
	for i := range h {
		h[i] = new(big.Rat)
	}
	p, e := seg.CertifyEndpoint(ctx, end, h, geom.TubeEndpointOptions{})
	if e != nil || !p.Proven {
		t.Fatal(p, e)
	}
	recipe, _, _ := seg.TrimA().Recipe()
	d := recipe.Data()
	d.Cells[0].BoundaryAnchor.Parameters[3] = .01
	if _, e := RebuildTubeCurves(ctx, d); !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("tampered recipe accepted", e)
	}
	if _, e := RebuildTubeCurves(ctx, recipe.Data()); e != nil {
		t.Fatal("recipe mutation affected original", e)
	}
	s.Attempts[0].BoundaryAnchor = nil
	if q, e := CertifySplinePairBranches(ctx, a, b, s, o); e == nil || q.Complete {
		t.Fatal("certificate trusted with missing boundary evidence", e)
	}
}
