package trimmed

import (
	"bytes"
	"context"
	"math"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"slices"
	"testing"
)

func anisotropicBoreInner(t *testing.T, weight float64, end [2]float64) Solid {
	t.Helper()
	w := []*geom.WrappedSurfacePatch{anisotropicStationBand(t, 0, 2, [2]float64{.5, .25}, [2]float64{.25, .5}, weight), anisotropicStationBand(t, 2, 3, [2]float64{.25, .5}, end, weight)}
	s, _, e := NewWrappedAnisotropicStationLoft(context.Background(), w, 1e-8, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func TestAnisotropicStationBore(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	r := big.NewRat
	outer := anisotropicMassFixture(t, false, geom.Vec3{})
	inner := anisotropicBoreInner(t, .75, [2]float64{.375, .25})
	// Retain original ownership even when operand face numbering differs
	// from the reconstructed canonical source order.
	for _, operand := range []*Solid{&outer, &inner} {
		faces := operand.Faces()
		slices.Reverse(faces)
		q, e := NewSolidContext(ctx, operand.Vertices(), operand.Edges(), faces, 1e-8)
		if e != nil {
			t.Fatal(e)
		}
		*operand = q
	}
	o := geom.SurfacePatchOptions{}
	s, h, e := CutWrappedAnisotropicStationLofts(ctx, outer, inner, o)
	if e != nil {
		t.Fatal(e)
	}
	top := s.Topology()
	if len(top.Shells) != 1 || top.InnerWires != 2 || top.EulerCharacteristic != 0 || len(h.Bands) != 2 || len(h.InnerBands) != 2 {
		t.Fatalf("topology %+v %+v", top, h)
	}
	for _, band := range h.Bands {
		if band.SourceWalls[0] != band.ResultWall || s.faces[band.ResultWall].Surface != outer.faces[band.SourceWalls[0]].Surface || s.faces[band.ResultWall].Reversed {
			t.Fatal("outer ancestry")
		}
	}
	for _, band := range h.InnerBands {
		if s.faces[band.ResultWall].Surface != inner.faces[band.SourceWalls[1]].Surface || !s.faces[band.ResultWall].Reversed || !band.BasisPreserved {
			t.Fatal("inner ancestry")
		}
	}
	for i, id := range h.ResultCaps {
		if id != h.SourceCaps[0][i] || len(s.faces[id].Holes) != 1 {
			t.Fatal("annular caps")
		}
	}
	var raw bytes.Buffer
	if e = WriteJSON(&raw, s); e != nil {
		t.Fatal(e)
	}
	s, e = ReadJSONContext(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	p, e := s.ValidateWrappedAnisotropicStationBore(ctx, o)
	if e != nil || !p.Complete || len(p.OuterWalls) != 2 || len(p.InnerWalls) != 2 || p.MinimumScaleGap[0].Sign() <= 0 || p.MinimumScaleGap[1].Sign() <= 0 {
		t.Fatalf("reload proof %+v %v", p, e)
	}
	mass, e := s.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	// Independent affine scale-product integrals: outer43/6. Inner first
	// interval13/48 and second11/96, total37/96. Profile from Green quadrature.
	want := rationalQuarterProfileArea(.75, 8192) * (43.0/6 - 37.0/96)
	if math.Abs(mass.Volume-want) > 1e-7 {
		t.Fatalf("volume %g want %g", mass.Volume, want)
	}
	cases := []struct {
		p    [3]*big.Rat
		want parametric.Location
	}{
		{[3]*big.Rat{r(1, 4), r(-1, 2), r(1, 1)}, parametric.Outside},
		{[3]*big.Rat{r(1, 4), r(-1, 2), r(0, 1)}, parametric.Outside},
		{[3]*big.Rat{r(1, 1), r(-1, 2), r(1, 1)}, parametric.Inside},
		{[3]*big.Rat{r(3, 4), r(-1, 2), r(0, 1)}, parametric.Boundary},
		{[3]*big.Rat{r(1, 1), r(-1, 2), r(0, 1)}, parametric.Boundary},
		{[3]*big.Rat{r(1, 2), r(-1, 2), r(2, 1)}, parametric.Boundary},
	}
	for _, c := range cases {
		got, e := s.LocateWrappedAnisotropicStationBoreRational(ctx, c.p, WrappedRationalStationPointOptions{})
		if e != nil || got != c.want {
			t.Fatalf("point %v %v want %v error %v", c.p, got, c.want, e)
		}
	}
	var again bytes.Buffer
	if e = WriteJSON(&again, s); e != nil || !bytes.Equal(raw.Bytes(), again.Bytes()) {
		t.Fatal("deterministic reload", e)
	}
	for _, bad := range []Solid{anisotropicBoreInner(t, .8, [2]float64{.375, .25}), anisotropicBoreInner(t, .75, [2]float64{2, .25}), anisotropicBoreInner(t, .75, [2]float64{2.25, .25})} {
		q, h, e := CutWrappedAnisotropicStationLofts(ctx, outer, bad, o)
		if e == nil || len(q.faces) != 0 || len(h.Bands) != 0 {
			t.Fatal("unproved/tangent nesting accepted")
		}
	}
	// This tool is smaller at every OUTER station but protrudes at its own
	// z=2 station: only the merged partition exposes the violation.
	peakWalls := []*geom.WrappedSurfacePatch{anisotropicStationBand(t, 0, 2, [2]float64{.25, .25}, [2]float64{1.625, .5}, .75), anisotropicStationBand(t, 2, 3, [2]float64{1.625, .5}, [2]float64{.375, .25}, .75)}
	peak, _, e := NewWrappedAnisotropicStationLoft(ctx, peakWalls, 1e-8, o)
	if e != nil {
		t.Fatal(e)
	}
	if _, _, e = CutWrappedAnisotropicStationLofts(ctx, outer, peak, o); e == nil {
		t.Fatal("missed inner-only crossing station")
	}
	bad := s
	bad.faces = append([]Face(nil), s.faces...)
	bad.faces[p.InnerWalls[0]].Reversed = false
	if _, e = bad.ValidateWrappedAnisotropicStationBore(ctx, o); e == nil {
		t.Fatal("forged inner orientation")
	}
	if q, h, e := CutWrappedAnisotropicStationLofts(ctx, outer, inner, geom.SurfacePatchOptions{MaxOperations: 1}); e == nil || len(q.faces) != 0 || len(h.Bands) != 0 {
		t.Fatal("quota")
	}
	c, cancel := context.WithCancel(ctx)
	cancel()
	if _, _, e = CutWrappedAnisotropicStationLofts(c, outer, inner, o); e == nil {
		t.Fatal("cancel")
	}
	t.Logf("faces=%d edges=%d volume=%g expected=%g", len(s.faces), len(s.edges), mass.Volume, want)
}
