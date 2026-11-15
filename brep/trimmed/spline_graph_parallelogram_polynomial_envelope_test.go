package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"reflect"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func obliquePolynomialStock(t *testing.T, axis, profile int) (*geom.BSplineSurface, *geom.BSplineSurface, *geom.BSplineSurface) {
	t.Helper()
	_, physical, _ := nativeCubicCrossingTangencyFixture(t, axis, profile)
	a := independentProjectedEnvelopeRoof(t, physical)
	b := independentAlignedEnvelopeRoof(t, axis, 1, 25.0/8)
	toWorld := func(u, v, z float64) geom.Vec3 {
		return geom.Vec3{X: 2 + 2*u + v, Y: -1 + u + 3*v, Z: z}
	}
	physicalXY := func(s *geom.BSplineSurface) *geom.BSplineSurface {
		d := s.Data()
		for i, p := range d.Poles {
			d.Poles[i] = toWorld(p.X, p.Y, p.Z)
		}
		out, e := geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
		if e != nil {
			t.Fatal(e)
		}
		return out
	}
	// The base charts (10-2v,2-4u) are authored independently of both roofs.
	var poles []geom.Vec3
	for _, v := range []float64{1, 0} {
		for _, u := range []float64{1, 0} {
			poles = append(poles, toWorld(u, v, 1.0/8+u/4+3*v/16))
		}
	}
	base, e := geom.NewBSplineSurface(1, 1, 2, 2, poles, []float64{8, 8, 10, 10}, []float64{-2, -2, 2, 2}, nil)
	if e != nil {
		t.Fatal(e)
	}
	return base, physicalXY(a), physicalXY(b)
}

func obliquePolynomialEnvelopeMeasures(axis, profile int, op parametric.BooleanOp) (area, volume float64) {
	fuse := op == parametric.Union
	exact := big.NewRat(3778437266204723, 1257841302175744)
	if fuse {
		exact = [2]*big.Rat{big.NewRat(11673156157917031, 3773523906527232), big.NewRat(11671563727488871, 3773523906527232)}[profile]
	}
	// The independent original roof integrals subtract the sloping base.
	// The physical area determinant is 2*3-1*1, with unchanged heights.
	exact.Sub(exact, big.NewRat(11, 32)).Mul(exact, big.NewRat(5, 1))
	volume, _ = exact.Float64()
	dirs := [2]geom.Vec2{{X: 2, Y: 1}, {X: 1, Y: 3}}
	slopes := [2]float64{1.0 / 4, 3.0 / 16}
	area = polynomialPairSourceEnvelopeAreaOnVectors(fuse, []float64{0, .25, .75, 1}, cubicCrossingTangencyBranches(profile),
		dirs[axis], dirs[1-axis], [3]float64{1.0 / 8, slopes[axis], slopes[1-axis]})
	return area, volume
}

func TestNativeParallelogramPolynomialEnvelope(t *testing.T) {
	ctx := context.Background()
	options := PolynomialCubicCrossingTangencyEnvelopeOptions{}
	for profile := 0; profile < 2; profile++ {
		for axis := 0; axis < 2; axis++ {
			base, a, b := obliquePolynomialStock(t, axis, profile)
			for order, roofs := range [][2]*geom.BSplineSurface{{a, b}, {b, a}} {
				for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union} {
					envelope, e := CombineParallelogramPolynomialEnvelope(ctx, base, roofs[0], roofs[1], axis, op, 1e-9, options)
					if e != nil {
						t.Fatal("exact native oblique walls and original caps", profile, axis, order, op, e)
					}
					checkParallelogramPolynomialEnvelope(t, envelope, profile, axis, order, op)
					var raw bytes.Buffer
					if e := envelope.WriteJSON(ctx, &raw); e != nil {
						t.Fatal(e)
					}
					loaded, e := ReadParallelogramPolynomialEnvelopeJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, options)
					if e != nil || loaded.History() != envelope.History() || !reflect.DeepEqual(loaded.Contacts(), envelope.Contacts()) {
						t.Fatal("exact oblique shape ledger survives source reload", profile, axis, order, op, e)
					}
					checkParallelogramPolynomialEnvelope(t, loaded, profile, axis, order, op)
					for i, source := range envelope.sources {
						if !geom.SameSplineSurface(source, loaded.sources[i]) {
							t.Fatal("reload must preserve original oblique source coefficients", i)
						}
					}
					wantArea, wantVolume := obliquePolynomialEnvelopeMeasures(axis, profile, op)
					mass, e := loaded.Boundary().MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
					if e != nil || math.Abs(mass.Area-wantArea) > 1e-8 || math.Abs(mass.Volume-wantVolume) > 1e-8 {
						t.Fatal("independent physical area and volume", profile, axis, order, op, mass.Area, wantArea, mass.Volume, wantVolume, e)
					}
					var replay bytes.Buffer
					if e := loaded.WriteJSON(ctx, &replay); e != nil || !bytes.Equal(raw.Bytes(), replay.Bytes()) {
						t.Fatal("canonical exact oblique source recipe", e)
					}
				}
			}
		}
	}
}

func checkParallelogramPolynomialEnvelope(t *testing.T, r *AlignedPolynomialEnvelope, profile, axis, order int, op parametric.BooleanOp) {
	t.Helper()
	s, h := r.Boundary(), r.History()
	top := s.Topology()
	if len(top.Shells) != 1 || top.EulerCharacteristic != 2 || top.CollapsedEdges != 0 || len(s.faces) != 14 ||
		s.faces[h.Base].Surface != r.sources[0] || s.faces[h.Base].Reversed {
		t.Fatal("original oblique shell and reflected base", top)
	}
	baseUV := func(p geom.Vec3) geom.Vec2 {
		q := obliqueFixtureUnitPoint(p)
		return geom.Vec2{X: 10 - 2*q.Y, Y: 2 - 4*q.X}
	}
	roofUV := func(owner int, p geom.Vec3) geom.Vec2 {
		return projectedFixtureUV(owner, axis, obliqueFixtureUnitPoint(p))
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
		q := obliqueFixtureUnitPoint(p)
		uv := [2]float64{q.X, q.Y}
		if e != nil || c.Order != orders[i] || uv[axis] != float64(1+2*i)/4 || s.vertices[c.Vertex] != p {
			t.Fatal("original physical contact point and exact chart station", i, c, e)
		}
	}
	for _, use := range s.faces[h.Base].Outer {
		start, end := edgeEnds(use)
		if !use.Reversed || use.UVStart != baseUV(start) || use.UVEnd != baseUV(end) {
			t.Fatal("original reflected base cap coedge", use)
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
			t.Fatal("same physical sector on original roof support", i, owner)
		}
		for _, use := range f.Outer {
			p, q := edgeEnds(use)
			if use.UVStart != roofUV(owner, p) || use.UVEnd != roofUV(owner, q) {
				t.Fatal("exact original roof chart trim", i, use.Edge)
			}
		}
	}
	for _, face := range s.faces[h.Base+1:] {
		wall, ok := face.Surface.(splineGraphAffineWall)
		if !ok {
			t.Fatal("oblique wall retains its exact unnormalized side vector")
		}
		lo, hi := face.Outer[0], face.Outer[2]
		p, q := edgeEnds(lo)
		baseTrim, ok := graphWallSourceTrim(lo.Curve, false, true)
		if !ok || baseTrim.surface != r.sources[0] || baseTrim.a != baseUV(p) || baseTrim.b != baseUV(q) ||
			wall.origin != p || wall.along != (geom.Vec3{X: q.X - p.X, Y: q.Y - p.Y}) {
			t.Fatal("oblique wall uses original base and exact physical edge", lo.Edge)
		}
		q, p = edgeEnds(hi)
		roofTrim, ok := graphWallSourceTrim(hi.Curve, true, true)
		owner := 0
		if roofTrim.surface != r.sources[1+order] {
			owner = 1
		}
		if !ok || roofTrim.surface != r.sources[1+(owner^order)] || roofTrim.a != roofUV(owner, p) || roofTrim.b != roofUV(owner, q) {
			t.Fatal("oblique wall uses its original roof chart", hi.Edge)
		}
	}
}

func obliqueFixtureUnitPoint(p geom.Vec3) geom.Vec3 {
	x, y := p.X-2, p.Y+1
	return geom.Vec3{X: (3*x - y) / 5, Y: (2*y - x) / 5, Z: p.Z}
}

func TestObliqueSplineWallExactAgreement(t *testing.T) {
	if _, e := newSplineGraphAffineWall(geom.Vec3{X: 1}, geom.Vec3{X: math.Ldexp(1, 60)}); !errors.Is(e, geom.ErrUnrepresentable) {
		t.Fatal("native oblique side vector must be losslessly represented", e)
	}
	ctx := context.Background()
	base, a, b := obliquePolynomialStock(t, 0, 0)
	r, e := CombineParallelogramPolynomialEnvelope(ctx, base, a, b, 0, parametric.Intersection, 1e-9, PolynomialCubicCrossingTangencyEnvelopeOptions{})
	if e != nil {
		t.Fatal(e)
	}
	s := r.Boundary()
	wall := s.faces[r.History().Base+1]
	use := wall.Outer[2]
	model := s.edges[use.Edge]
	ok, e := compareSplineWallUse(ctx, model, wall.Surface, use, edgecheck.Options{})
	if e != nil || !ok {
		t.Fatal("original oblique roof isocurve and wall agree exactly", e)
	}
	trim, ok := graphWallSourceTrim(use.Curve, true, true)
	if !ok {
		t.Fatal("original oblique roof trim missing")
	}
	trim.a, trim.b = trim.b, trim.a
	use.Curve = reverseTrim{affineGraphWallTrim{trim}}
	ok, e = compareSplineWallUse(ctx, model, wall.Surface, use, edgecheck.Options{})
	if e != nil || ok {
		t.Fatal("changed wall owner direction must fail the exact agreement proof", e)
	}
}

func TestNativeParallelogramPolynomialEnvelopeRejections(t *testing.T) {
	ctx := context.Background()
	o := PolynomialCubicCrossingTangencyEnvelopeOptions{}
	base, a, b := obliquePolynomialStock(t, 0, 0)
	r, e := CombineParallelogramPolynomialEnvelope(ctx, base, a, b, 0, parametric.Union, 1e-9, o)
	if e != nil {
		t.Fatal(e)
	}
	var raw bytes.Buffer
	if e := r.WriteJSON(ctx, &raw); e != nil {
		t.Fatal(e)
	}
	if r, e := ReadRectangularPolynomialEnvelopeJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, o); r != nil || !errors.Is(e, geom.ErrInvalid) {
		t.Fatal("axis-aligned native recipe remains separate", e)
	}
	if r, e := CombineRectangularPolynomialEnvelope(ctx, base, a, b, 0, parametric.Intersection, 1e-9, o); r != nil || !errors.Is(e, geom.ErrDomain) {
		t.Fatal("axis-aligned constructor still rejects oblique footprint", e)
	}
	var recipe polynomialEnvelopeRecipe
	if e := json.Unmarshal(raw.Bytes(), &recipe); e != nil {
		t.Fatal(e)
	}
	recipe.Ledger.Contacts[0].Point[0] = "1/4"
	bad, e := json.Marshal(recipe)
	if e != nil {
		t.Fatal(e)
	}
	if r, e := ReadParallelogramPolynomialEnvelopeJSON(ctx, bytes.NewReader(bad), JSONLimits{}, o); r != nil || !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("native reproof checks absolute world coordinates", e)
	}
	d := b.Data()
	for i := range d.Poles {
		d.Poles[i].X += .25
	}
	shifted, e := geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
	if e != nil {
		t.Fatal(e)
	}
	if r, e := CombineParallelogramPolynomialEnvelope(ctx, base, a, shifted, 0, parametric.Intersection, 1e-9, o); r != nil || !errors.Is(e, geom.ErrDomain) {
		t.Fatal("independent affine roofs must cover the same oblique domain", e)
	}
}
