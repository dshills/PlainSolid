package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func independentProjectedEnvelopeRoof(t *testing.T, physical *geom.BSplineSurface) *geom.BSplineSurface {
	t.Helper()
	// Author the polynomial tensor in U=4X-2, V=10-2Y. Keep its
	// authored physical coefficients, reversing the V-order in the
	// control net and knot vector. B's chart is authored separately.
	p := physical.Data()
	for i, u := range p.KnotsU {
		p.KnotsU[i] = 4*u - 2
	}
	kv := make([]float64, len(p.KnotsV))
	for i, v := range p.KnotsV {
		kv[len(kv)-1-i] = 10 - 2*v
	}
	poles := make([]geom.Vec3, len(p.Poles))
	weights := make([]float64, len(p.Weights))
	for u := 0; u < p.CountU; u++ {
		for v := 0; v < p.CountV; v++ {
			to, from := u*p.CountV+v, u*p.CountV+p.CountV-1-v
			poles[to] = p.Poles[from]
			if len(weights) != 0 {
				weights[to] = p.Weights[from]
			}
		}
	}
	s, e := geom.NewBSplineSurface(p.DegreeU, p.DegreeV, p.CountU, p.CountV, poles, p.KnotsU, kv, weights)
	if e != nil {
		t.Fatal(e)
	}
	return s
}

func TestNativeProjectedPolynomialEnvelope(t *testing.T) {
	ctx := context.Background()
	o := PolynomialCubicCrossingTangencyEnvelopeOptions{}
	for profile := 0; profile < 2; profile++ {
		for axis := 0; axis < 2; axis++ {
			base, physical, _ := nativeCubicCrossingTangencyFixture(t, axis, profile)
			a := independentProjectedEnvelopeRoof(t, physical)
			b := independentAlignedEnvelopeRoof(t, axis, 1, 25.0/8)
			if requireIdentitySplineChart(ctx, a, o.Section.Patch) == nil || requireIdentitySplineChart(ctx, b, o.Section.Patch) == nil {
				t.Fatal("both independent roof charts differ from the base", axis)
			}
			for order, roofs := range [][2]*geom.BSplineSurface{{a, b}, {b, a}} {
				for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union} {
					envelope, e := CombineProjectedPolynomialEnvelope(ctx, base, roofs[0], roofs[1], axis, op, 1e-9, o)
					if e != nil {
						t.Fatal("both original independent charts", axis, order, op, e)
					}
					checkProjectedPolynomialEnvelope(t, envelope, profile, axis, order, op)
					var seed bytes.Buffer
					if e := envelope.WriteJSON(ctx, &seed); e != nil {
						t.Fatal(e)
					}
					reloaded, e := ReadProjectedPolynomialEnvelopeJSON(ctx, bytes.NewReader(seed.Bytes()), JSONLimits{}, o)
					if e != nil || reloaded.History() != envelope.History() || !reflect.DeepEqual(reloaded.Contacts(), envelope.Contacts()) {
						t.Fatal("exact projected source ledger survived reload", axis, order, op, e)
					}
					for i, original := range envelope.sources {
						if !geom.SameSplineSurface(original, reloaded.sources[i]) {
							t.Fatal("reload changed original spline coefficients", i)
						}
					}
					checkProjectedPolynomialEnvelope(t, reloaded, profile, axis, order, op)
					wantArea, wantVolume := polynomialCubicCrossingTangencyEnvelopeMeasures(t, profile, op)
					got, e := reloaded.Boundary().MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
					if e != nil || math.Abs(got.Volume-wantVolume) > 1e-9 || math.Abs(got.Area-wantArea) > 1e-8 {
						t.Fatal("independent physical volume and area", axis, order, op, got.Volume, wantVolume, got.Area, wantArea, e)
					}
					var out bytes.Buffer
					if e := reloaded.WriteJSON(ctx, &out); e != nil || !bytes.Equal(seed.Bytes(), out.Bytes()) {
						t.Fatal("canonical projected source recipe", e)
					}
				}
			}
		}
	}
}

func checkProjectedPolynomialEnvelope(t *testing.T, r *AlignedPolynomialEnvelope, profile, axis, order int, op parametric.BooleanOp) {
	t.Helper()
	s, h := r.Boundary(), r.History()
	top := s.Topology()
	if len(top.Shells) != 1 || top.EulerCharacteristic != 2 || top.CollapsedEdges != 0 || len(s.faces) != 14 ||
		s.faces[h.Base].Surface != r.sources[0] {
		t.Fatal("closed native projected shell", top)
	}
	orders := [2][2]int{{3, 2}, {2, 3}}[profile]
	for i, c := range r.Contacts() {
		p, e := geom.RepresentedExactPoint(c.Point)
		xy := [2]float64{p.X, p.Y}
		if e != nil || c.Order != orders[i] || xy[axis] != float64(1+2*i)/4 || s.vertices[c.Vertex] != p {
			t.Fatal("exact native contact in physical station order", i, c, e)
		}
	}
	mapUV := func(physicalOwner int, p geom.Vec3) geom.Vec2 {
		if physicalOwner == 0 {
			return geom.Vec2{X: 4*p.X - 2, Y: 10 - 2*p.Y}
		}
		xy := [2]float64{p.X, p.Y}
		return geom.Vec2{X: 5 - 3*xy[axis], Y: 2*xy[1-axis] - 1}
	}
	edgeEnds := func(u Use) (geom.Vec3, geom.Vec3) {
		e := s.edges[u.Edge]
		if u.Reversed {
			e.Start, e.End = e.End, e.Start
		}
		return s.vertices[e.Start], s.vertices[e.End]
	}
	for i, index := range h.RoofFaces {
		f := s.faces[index]
		isB := (i == 0 || i == 4) == (op == parametric.Intersection)
		physicalOwner := 0
		if isB {
			physicalOwner = 1
		}
		owner := physicalOwner ^ order
		if h.RoofOwners[i] != owner || f.Surface != r.sources[1+owner] || f.Reversed != (!isB || axis == 0) {
			t.Fatal("selected roof has original chart and orientation", i, owner)
		}
		for _, u := range f.Outer {
			a, b := edgeEnds(u)
			if u.UVStart != mapUV(physicalOwner, a) || u.UVEnd != mapUV(physicalOwner, b) {
				t.Fatal("independent original roof pcurve station", i, u.Edge)
			}
		}
	}
	var walls [2]int
	for _, f := range s.faces[h.Base+1:] {
		u := f.Outer[2]
		trim, ok := graphWallSourceTrim(u.Curve, true, true)
		if !ok {
			t.Fatal("top wall trim must use its own roof chart")
		}
		owner := 0
		if trim.surface != r.sources[1+order] {
			if trim.surface != r.sources[1+(order^1)] {
				t.Fatal("original wall roof")
			}
			owner = 1
		}
		b, a := edgeEnds(u)
		if trim.a != mapUV(owner, a) || trim.b != mapUV(owner, b) {
			t.Fatal("original wall pcurve range", owner, u.Edge)
		}
		walls[owner]++
	}
	want := [2]int{6, 2}
	if op == parametric.Intersection {
		want = [2]int{2, 6}
	}
	if walls != want {
		t.Fatal("complete original wall owner partition", walls, want)
	}
}

func TestNativeProjectedPolynomialEnvelopeRejections(t *testing.T) {
	ctx := context.Background()
	o := PolynomialCubicCrossingTangencyEnvelopeOptions{}
	base, physical, _ := nativeCubicCrossingTangencyFixture(t, 0, 0)
	a := independentProjectedEnvelopeRoof(t, physical)
	b := independentAlignedEnvelopeRoof(t, 0, 1, 25.0/8)
	r, e := CombineProjectedPolynomialEnvelope(ctx, base, a, b, 0, parametric.Union, 1e-9, o)
	if e != nil {
		t.Fatal(e)
	}
	var raw bytes.Buffer
	if e := r.WriteJSON(ctx, &raw); e != nil {
		t.Fatal(e)
	}
	if r, e := ReadAlignedPolynomialEnvelopeJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, o); r != nil || !errors.Is(e, geom.ErrInvalid) {
		t.Fatal("original aligned recipe identity stays separate", e)
	}
	var recipe polynomialEnvelopeRecipe
	if e := json.Unmarshal(raw.Bytes(), &recipe); e != nil {
		t.Fatal(e)
	}
	recipe.Ledger.RoofOwners[0] ^= 1
	bad, e := json.Marshal(recipe)
	if e != nil {
		t.Fatal(e)
	}
	if r, e := ReadProjectedPolynomialEnvelopeJSON(ctx, bytes.NewReader(bad), JSONLimits{}, o); r != nil || !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("reconstruction must rebind both original owners", e)
	}
	narrow := independentAlignedEnvelopeRoof(t, 0, .75, 25.0/8)
	if r, e := CombineProjectedPolynomialEnvelope(ctx, base, a, narrow, 0, parametric.Intersection, 1e-9, o); r != nil || !errors.Is(e, geom.ErrDomain) {
		t.Fatal("each independently projected roof must cover the base", e)
	}
}
