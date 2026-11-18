package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func rectangularPolynomialStock(t *testing.T, axis, profile int) (*geom.BSplineSurface, *geom.BSplineSurface, *geom.BSplineSurface) {
	t.Helper()
	_, physical, _ := nativeCubicCrossingTangencyFixture(t, axis, profile)
	a := independentProjectedEnvelopeRoof(t, physical)
	b := independentAlignedEnvelopeRoof(t, axis, 1, 25.0/8)
	physicalXY := func(s *geom.BSplineSurface) *geom.BSplineSurface {
		d := s.Data()
		for i, p := range d.Poles {
			d.Poles[i] = geom.Vec3{X: 2 + 2*p.X, Y: -1 + 3*p.Y, Z: p.Z}
		}
		out, e := geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
		if e != nil {
			t.Fatal(e)
		}
		return out
	}
	// Independently authored base with reflected and exchanged axes.
	// In normalized physical coordinates (x,y), its UV chart is
	// (10-2y, 2-4x) and its height is 1/8+x/4+3y/16.
	var poles []geom.Vec3
	for _, y := range []float64{1, 0} {
		for _, x := range []float64{1, 0} {
			poles = append(poles, geom.Vec3{X: 2 + 2*x, Y: -1 + 3*y, Z: 1.0/8 + x/4 + 3*y/16})
		}
	}
	base, e := geom.NewBSplineSurface(1, 1, 2, 2, poles, []float64{8, 8, 10, 10}, []float64{-2, -2, 2, 2}, nil)
	if e != nil {
		t.Fatal(e)
	}
	return base, physicalXY(a), physicalXY(b)
}

func rectangularPolynomialEnvelopeMeasures(t *testing.T, axis, profile int, op parametric.BooleanOp) (area, volume float64) {
	t.Helper()
	fuse := op == parametric.Union
	exact := big.NewRat(3778437266204723, 1257841302175744)
	if fuse {
		exact = [2]*big.Rat{big.NewRat(11673156157917031, 3773523906527232), big.NewRat(11671563727488871, 3773523906527232)}[profile]
	}
	// The physical affine change preserves each roof height and multiplies
	// the XY area by six. Subtract the independently integrated sloping base.
	exact.Sub(exact, big.NewRat(11, 32)).Mul(exact, big.NewRat(6, 1))
	volume, _ = exact.Float64()
	lengths := [2]float64{2, 3}
	slopes := [2]float64{1.0 / 4, 3.0 / 16}
	area = polynomialPairSourceEnvelopeAreaScaled(fuse, []float64{0, .25, .75, 1}, cubicCrossingTangencyBranches(profile),
		lengths[axis], lengths[1-axis], [3]float64{1.0 / 8, slopes[axis], slopes[1-axis]})
	return area, volume
}

func TestNativeRectangularPolynomialEnvelope(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	o := PolynomialCubicCrossingTangencyEnvelopeOptions{}
	for profile := 0; profile < 2; profile++ {
		for axis := 0; axis < 2; axis++ {
			base, a, b := rectangularPolynomialStock(t, axis, profile)
			for order, roofs := range [][2]*geom.BSplineSurface{{a, b}, {b, a}} {
				for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union} {
					envelope, e := CombineRectangularPolynomialEnvelope(ctx, base, roofs[0], roofs[1], axis, op, 1e-9, o)
					if e != nil {
						t.Fatal("exact nonunit original stocks", profile, axis, order, op, e)
					}
					checkRectangularPolynomialEnvelope(t, envelope, profile, axis, order, op)
					var raw bytes.Buffer
					if e := envelope.WriteJSON(ctx, &raw); e != nil {
						t.Fatal(e)
					}
					loaded, e := ReadRectangularPolynomialEnvelopeJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, o)
					if e != nil || loaded.History() != envelope.History() || !reflect.DeepEqual(loaded.Contacts(), envelope.Contacts()) {
						t.Fatal("physical station identities survive source reload", profile, axis, order, op, e)
					}
					checkRectangularPolynomialEnvelope(t, loaded, profile, axis, order, op)
					for i, source := range envelope.sources {
						if !geom.SameSplineSurface(source, loaded.sources[i]) {
							t.Fatal("source recipe changed original base or roof coefficients", i)
						}
					}
					a, v := rectangularPolynomialEnvelopeMeasures(t, axis, profile, op)
					mass, e := loaded.Boundary().MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
					if e != nil || math.Abs(mass.Volume-v) > 1e-8 || math.Abs(mass.Area-a) > 1e-8 {
						t.Fatal("independently integrated physical mass of nonunit stock", profile, axis, order, op, mass.Volume, v, mass.Area, a, e)
					}
					var replay bytes.Buffer
					if e := loaded.WriteJSON(ctx, &replay); e != nil || !bytes.Equal(raw.Bytes(), replay.Bytes()) {
						t.Fatal("canonical source recipe for original footprint", e)
					}
				}
			}
		}
	}
}

func checkRectangularPolynomialEnvelope(t *testing.T, r *AlignedPolynomialEnvelope, profile, axis, order int, op parametric.BooleanOp) {
	t.Helper()
	s, h := r.Boundary(), r.History()
	top := s.Topology()
	if len(top.Shells) != 1 || top.EulerCharacteristic != 2 || top.CollapsedEdges != 0 || len(s.faces) != 14 || s.faces[h.Base].Surface != r.sources[0] ||
		s.faces[h.Base].Reversed {
		t.Fatal("closed native nonunit envelope on original reflected base", top)
	}
	modelToUnit := func(p geom.Vec3) geom.Vec3 {
		return geom.Vec3{X: (p.X - 2) / 2, Y: (p.Y + 1) / 3, Z: p.Z}
	}
	baseUV := func(p geom.Vec3) geom.Vec2 {
		q := modelToUnit(p)
		return geom.Vec2{X: 10 - 2*q.Y, Y: 2 - 4*q.X}
	}
	roofUV := func(owner int, p geom.Vec3) geom.Vec2 {
		return projectedFixtureUV(owner, axis, modelToUnit(p))
	}
	edgeEnds := func(u Use) (geom.Vec3, geom.Vec3) {
		e := s.edges[u.Edge]
		if u.Reversed {
			e.Start, e.End = e.End, e.Start
		}
		return s.vertices[e.Start], s.vertices[e.End]
	}
	orders := [2][2]int{{3, 2}, {2, 3}}[profile]
	for i, c := range r.Contacts() {
		p, e := geom.RepresentedExactPoint(c.Point)
		q := modelToUnit(p)
		uv := [2]float64{q.X, q.Y}
		if e != nil || c.Order != orders[i] || uv[axis] != float64(1+2*i)/4 || s.vertices[c.Vertex] != p {
			t.Fatal("original exact station in physical model", i, c, e)
		}
	}
	for _, use := range s.faces[h.Base].Outer {
		p, q := edgeEnds(use)
		if use.UVStart != baseUV(p) || use.UVEnd != baseUV(q) || !use.Reversed {
			t.Fatal("base cap has original UVs", use)
		}
	}
	for i, face := range h.RoofFaces {
		f := s.faces[face]
		isB := (i == 0 || i == 4) == (op == parametric.Intersection)
		owner := 0
		if isB {
			owner = 1
		}
		if h.RoofOwners[i] != (owner^order) || f.Surface != r.sources[1+(owner^order)] || f.Reversed != (!isB || axis == 0) {
			t.Fatal("original signed roof selection on the physical rectangle", i, owner)
		}
		for _, use := range f.Outer {
			p, q := edgeEnds(use)
			if use.UVStart != roofUV(owner, p) || use.UVEnd != roofUV(owner, q) {
				t.Fatal("native roof trace with original chart", i, use.Edge)
			}
		}
	}
	for _, face := range s.faces[h.Base+1:] {
		low, top := face.Outer[0], face.Outer[2]
		baseTrim, ok := graphWallSourceTrim(low.Curve, false, true)
		p, q := edgeEnds(low)
		if !ok || baseTrim.surface != r.sources[0] || baseTrim.a != baseUV(p) || baseTrim.b != baseUV(q) {
			t.Fatal("original base wall pcurve")
		}
		roofTrim, ok := graphWallSourceTrim(top.Curve, true, true)
		owner := 0
		if roofTrim.surface != r.sources[1+order] {
			owner = 1
		}
		q, p = edgeEnds(top)
		if !ok || roofTrim.surface != r.sources[1+(owner^order)] || roofTrim.a != roofUV(owner, p) || roofTrim.b != roofUV(owner, q) {
			t.Fatal("wall top uses original roof chart")
		}
	}
}

func TestNativeRectangularPolynomialEnvelopePositiveBaseChart(t *testing.T) {
	ctx := context.Background()
	base, a, b := rectangularPolynomialStock(t, 0, 0)
	positive, e := base.ReversedU()
	if e != nil {
		t.Fatal(e)
	}
	r, e := CombineRectangularPolynomialEnvelope(ctx, positive, a, b, 0, parametric.Intersection, 1e-9, PolynomialCubicCrossingTangencyEnvelopeOptions{})
	if e != nil {
		t.Fatal(e)
	}
	s := r.Boundary()
	face := s.faces[r.History().Base]
	if face.Surface != positive || !face.Reversed {
		t.Fatal("downward cap on positive original base chart")
	}
	for _, u := range face.Outer {
		edge := s.edges[u.Edge]
		p := s.vertices[edge.Start]
		expected := geom.Vec2{X: 8 + 2*(p.Y+1)/3, Y: 2 - 2*(p.X-2)}
		if u.Reversed || u.UVStart != expected {
			t.Fatal("original positive base pcurve", u.UVStart, expected)
		}
	}
	wantArea, wantVolume := rectangularPolynomialEnvelopeMeasures(t, 0, 0, parametric.Intersection)
	mass, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
	if e != nil || math.Abs(mass.Area-wantArea) > 1e-8 || math.Abs(mass.Volume-wantVolume) > 1e-8 {
		t.Fatal("independent mass with positive base chart", mass.Area, mass.Volume, e)
	}
}

func TestNativeRectangularPolynomialEnvelopeRejections(t *testing.T) {
	ctx := context.Background()
	o := PolynomialCubicCrossingTangencyEnvelopeOptions{}
	base, a, b := rectangularPolynomialStock(t, 0, 0)
	r, e := CombineRectangularPolynomialEnvelope(ctx, base, a, b, 0, parametric.Union, 1e-9, o)
	if e != nil {
		t.Fatal(e)
	}
	var raw bytes.Buffer
	if e := r.WriteJSON(ctx, &raw); e != nil {
		t.Fatal(e)
	}
	if r, e := ReadProjectedPolynomialEnvelopeJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, o); r != nil || !errors.Is(e, geom.ErrInvalid) {
		t.Fatal("nonunit recipe has distinct source identity", e)
	}
	var recipe polynomialEnvelopeRecipe
	if e := json.Unmarshal(raw.Bytes(), &recipe); e != nil {
		t.Fatal(e)
	}
	recipe.Ledger.Contacts[0].Point[0] = "1/4"
	changed, e := json.Marshal(recipe)
	if e != nil {
		t.Fatal(e)
	}
	if r, e := ReadRectangularPolynomialEnvelopeJSON(ctx, bytes.NewReader(changed), JSONLimits{}, o); r != nil || !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("reload must bind absolute physical model coordinates", e)
	}
	d := base.Data()
	for i := range d.Poles {
		d.Poles[i].X += d.Poles[i].Y / 8
	}
	skew, e := geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
	if e != nil {
		t.Fatal(e)
	}
	if r, e := CombineRectangularPolynomialEnvelope(ctx, skew, a, b, 0, parametric.Union, 1e-9, o); r != nil || !errors.Is(e, geom.ErrDomain) {
		t.Fatal("nonorthogonal base chart is outside the certified footprint family", e)
	}
	o.Proof.MaxOperations = 1
	if r, e := ReadRectangularPolynomialEnvelopeJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, o); r != nil || !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("native recipe requires fresh physical and owner proof", e)
	}
}
