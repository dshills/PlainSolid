package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func independentAlignedEnvelopeRoof(t *testing.T, axis int, width, knot float64) *geom.BSplineSurface {
	t.Helper()
	// Independently author B=3+XY/64 on (s,h), with the free model
	// coordinate width*(5-s)/3 and dependent coordinate (h+1)/2.
	var poles []geom.Vec3
	for _, free := range []float64{width, 0} {
		for _, dependent := range []float64{0, 1} {
			x, y := free, dependent
			if axis == 1 {
				x, y = y, x
			}
			poles = append(poles, geom.Vec3{X: x, Y: y, Z: 3 + x*y/64})
		}
	}
	s, e := geom.NewBSplineSurface(1, 1, 2, 2, poles, []float64{2, 2, 5, 5}, []float64{-1, -1, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	p, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	p, e = p.InsertUKnot(knot, 1, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	p, e = p.InsertVKnot(-3.0/8, 1, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if s, e = p.ToBSplineSurface(); e != nil {
		t.Fatal(e)
	}
	return s
}

func TestNativeAlignedPolynomialEnvelope(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	for axis := 0; axis < 2; axis++ {
		base, a, _ := nativeCubicCrossingTangencyFixture(t, axis, 0)
		b := independentAlignedEnvelopeRoof(t, axis, 1, 25.0/8)
		for order, roofs := range [][2]*geom.BSplineSurface{{a, b}, {b, a}} {
			for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union} {
				r, e := CombineAlignedPolynomialEnvelope(ctx, base, roofs[0], roofs[1], axis, op, 1e-9, PolynomialCubicCrossingTangencyEnvelopeOptions{})
				if e != nil {
					t.Fatal("native aligned envelope", axis, order, op, e)
				}
				assertNativeAlignedRoofOwners(t, r, axis, order, op)
				var raw bytes.Buffer
				if e = r.WriteJSON(ctx, &raw); e != nil {
					t.Fatal(e)
				}
				loaded, e := ReadAlignedPolynomialEnvelopeJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, PolynomialCubicCrossingTangencyEnvelopeOptions{})
				if e != nil || loaded.History() != r.History() || !reflect.DeepEqual(loaded.Contacts(), r.Contacts()) {
					t.Fatal("source reproof preserves arrangement", axis, order, op, e)
				}
				for i, source := range r.sources {
					if !geom.SameSplineSurface(source, loaded.sources[i]) {
						t.Fatal("reload changed owner chart", i)
					}
				}
				assertNativeAlignedRoofOwners(t, loaded, axis, order, op)
				wantArea, wantVolume := polynomialCubicCrossingTangencyEnvelopeMeasures(t, 0, op)
				got, e := loaded.Boundary().MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
				if e != nil || math.Abs(got.Volume-wantVolume) > 1e-9 || math.Abs(got.Area-wantArea) > 1e-8 {
					t.Fatal("independent original roof volume and area", axis, order, op, got.Volume, wantVolume, got.Area, wantArea, e)
				}
				var replay bytes.Buffer
				if e = loaded.WriteJSON(ctx, &replay); e != nil || !bytes.Equal(raw.Bytes(), replay.Bytes()) {
					t.Fatal("canonical aligned roof recipe", e)
				}
			}
		}
	}
}

func assertNativeAlignedRoofOwners(t *testing.T, r *AlignedPolynomialEnvelope, axis, order int, op parametric.BooleanOp) {
	t.Helper()
	s, h := r.Boundary(), r.History()
	topology := s.Topology()
	if len(topology.Shells) != 1 || topology.CollapsedEdges != 0 || topology.EulerCharacteristic != 2 || len(s.faces) != 14 {
		t.Fatal("native aligned closed shell", topology)
	}
	if s.faces[h.Base].Surface != r.sources[0] || r.Contacts()[0].Order != 3 || r.Contacts()[1].Order != 2 {
		t.Fatal("original base and typed junctions", h, r.Contacts())
	}
	mapped := func(p geom.Vec3) geom.Vec2 {
		xy := [2]float64{p.X, p.Y}
		return geom.Vec2{X: 5 - 3*xy[axis], Y: 2*xy[1-axis] - 1}
	}
	directedVertices := func(u Use) (geom.Vec3, geom.Vec3) {
		e := s.edges[u.Edge]
		if u.Reversed {
			return s.vertices[e.End], s.vertices[e.Start]
		}
		return s.vertices[e.Start], s.vertices[e.End]
	}
	for i, id := range h.RoofFaces {
		f := s.faces[id]
		isB := (i == 0 || i == 4) == (op == parametric.Intersection)
		wantOwner := order
		if isB {
			wantOwner ^= 1
		}
		if h.RoofOwners[i] != wantOwner || f.Surface != r.sources[1+wantOwner] || f.Reversed != (isB && axis == 0) {
			t.Fatal("signed source selection retains original support and orientation", i, h)
		}
		if isB {
			for _, use := range f.Outer {
				a, b := directedVertices(use)
				if use.UVStart != mapped(a) || use.UVEnd != mapped(b) {
					t.Fatal("owner-specific roof pcurve endpoints", id, use.Edge, use.UVStart, use.UVEnd)
				}
			}
		}
	}
	var walls int
	for _, f := range s.faces[h.Base+1:] {
		for _, use := range f.Outer {
			reversed, ok := use.Curve.(reverseTrim)
			if !ok {
				continue
			}
			owned, ok := reversed.differentiableTrim.(affineGraphWallTrim)
			if !ok {
				continue
			}
			a, b := directedVertices(use)
			if owned.surface != r.sources[1+(order^1)] || owned.a != mapped(b) || owned.b != mapped(a) {
				t.Fatal("owner-specific native wall isocurve", use.Edge, owned)
			}
			walls++
		}
	}
	wantWalls := 2
	if op == parametric.Intersection {
		wantWalls = 6
	}
	if walls != wantWalls {
		t.Fatal("complete original independent-chart wall trim ledger", walls, wantWalls)
	}
}

func TestNativeAlignedPolynomialEnvelopeRejections(t *testing.T) {
	ctx := context.Background()
	base, a, _ := nativeCubicCrossingTangencyFixture(t, 0, 0)
	b := independentAlignedEnvelopeRoof(t, 0, 1, 25.0/8)
	o := PolynomialCubicCrossingTangencyEnvelopeOptions{}
	good, e := CombineAlignedPolynomialEnvelope(ctx, base, a, b, 0, parametric.Union, 1e-9, o)
	if e != nil {
		t.Fatal(e)
	}
	var raw bytes.Buffer
	if e = good.WriteJSON(ctx, &raw); e != nil {
		t.Fatal(e)
	}
	var recipe polynomialEnvelopeRecipe
	if e := json.Unmarshal(raw.Bytes(), &recipe); e != nil {
		t.Fatal(e)
	}
	recipe.Ledger.RoofOwners[0] ^= 1
	tampered, e := json.Marshal(recipe)
	if e != nil {
		t.Fatal(e)
	}
	if r, e := ReadAlignedPolynomialEnvelopeJSON(ctx, bytes.NewReader(tampered), JSONLimits{}, o); r != nil || !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("reload must rebind original owner identity", e)
	}
	stock := independentAlignedEnvelopeRoof(t, 0, .75, 25.0/8)
	if e := json.Unmarshal(raw.Bytes(), &recipe); e != nil {
		t.Fatal(e)
	}
	recipe.Sources, e = encodePolynomialGraphSupports(ctx, [3]*geom.BSplineSurface{base, a, stock})
	if e != nil {
		t.Fatal(e)
	}
	tampered, e = json.Marshal(recipe)
	if e != nil {
		t.Fatal(e)
	}
	if r, e := ReadAlignedPolynomialEnvelopeJSON(ctx, bytes.NewReader(tampered), JSONLimits{}, o); r != nil || !errors.Is(e, geom.ErrDomain) {
		t.Fatal("native owner projection must cover the full base", e)
	}
	if r, e := ReadAlignedPolynomialEnvelopeJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{MaxEntities: 1}, o); r != nil || !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("native owner recipe respects topology limit", e)
	}
	var ordinary bytes.Buffer
	if e := WriteJSON(&ordinary, good.Boundary()); !errors.Is(e, geom.ErrInvalid) || ordinary.Len() != 0 {
		t.Fatal("the generic codec must retain its original scope", e)
	}
	if r, e := CombineAlignedPolynomialEnvelope(ctx, base, a, b, 0, parametric.Difference, 1e-9, o); r != nil || !errors.Is(e, geom.ErrInvalid) {
		t.Fatal("native aligned envelope operation", e)
	}
	o.Proof.MaxOperations = 1
	if r, e := ReadAlignedPolynomialEnvelopeJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, o); r != nil || !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("source reload must spend proof budget", e)
	}
}

func TestAffineGraphWallTrimRejectsWrongOwnerDirection(t *testing.T) {
	ctx := context.Background()
	base, a, _ := nativeCubicCrossingTangencyFixture(t, 0, 0)
	b := independentAlignedEnvelopeRoof(t, 0, 1, 25.0/8)
	r, e := CombineAlignedPolynomialEnvelope(ctx, base, a, b, 0, parametric.Intersection, 1e-9, PolynomialCubicCrossingTangencyEnvelopeOptions{})
	if e != nil {
		t.Fatal(e)
	}
	s := r.Boundary()
	checked := 0
	for _, face := range s.faces[r.history.Base+1:] {
		for _, use := range face.Outer {
			reversed, ok := use.Curve.(reverseTrim)
			if !ok {
				continue
			}
			owner, ok := reversed.differentiableTrim.(affineGraphWallTrim)
			if !ok {
				continue
			}
			model := s.edges[use.Edge]
			match, e := compareSplineWallUse(ctx, model, face.Surface, use, edgecheck.Options{})
			if e != nil || !match {
				t.Fatal("valid owner wall agreement", use.Edge, e)
			}
			owner.a, owner.b = owner.b, owner.a
			use.Curve = reverseTrim{owner}
			match, e = compareSplineWallUse(ctx, model, face.Surface, use, edgecheck.Options{})
			if e != nil || match {
				t.Fatal("swapped original owner wall range must not certify", use.Edge, e)
			}
			checked++
		}
	}
	if checked != 6 {
		t.Fatal("checked every affine roof wall face", checked)
	}
}
