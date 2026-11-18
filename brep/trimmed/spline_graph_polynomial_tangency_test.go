//go:build research

package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

func nativePolynomialTangencyFixture(t *testing.T, axis int, denominator, scale int64) (*geom.BSplineSurface, *geom.BSplineSurface, *geom.BSplineSurface) {
	t.Helper()
	// g-f=scale*(denominator*u-1)^2*(1+u^2)/64 with distinct source degrees.
	f := []*big.Rat{big.NewRat(1, 4), big.NewRat(3, 8), big.NewRat(-3, 8), big.NewRat(1, 8)}
	g := []*big.Rat{
		big.NewRat(16+scale, 64), big.NewRat(24-2*scale*denominator, 64),
		big.NewRat(-24+scale*(denominator*denominator+1), 64),
		big.NewRat(8-2*scale*denominator, 64), big.NewRat(scale*denominator*denominator, 64),
	}
	return twoPolynomialRoofFixture(t, axis, f, g)
}

func polynomialTangencyEnvelopeVolume(op parametric.BooleanOp) float64 {
	// Integrating the original quadratic gap gives negative material
	// 105(g-f)^3/768 in each free-coordinate fiber between the branches.
	// Add the independently integrated net gap for the Fuse envelope.
	if op == parametric.Union {
		return 59352555935.0 / 19193135104
	}
	return 57654372961.0 / 19193135104
}

func TestNativePolynomialTangencyEnvelope(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	for _, axis := range []int{0, 1} {
		base, a, b := nativePolynomialTangencyFixture(t, axis, 2, 1)
		for order, roofs := range [][2]*geom.BSplineSurface{{a, b}, {b, a}} {
			for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union} {
				r, e := CombineSplineGraphPolynomialTangency(ctx, base, roofs[0], roofs[1], axis, op, 1e-9, PolynomialTangencyEnvelopeOptions{})
				if e != nil {
					t.Fatal(axis, order, op, e)
				}
				checkNativePolynomialTangency(t, r, axis, order, op)
				mass, e := r.Boundary().MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
				want := polynomialTangencyEnvelopeVolume(op)
				if e != nil || math.Abs(mass.Volume-want) > 1e-9 {
					t.Fatal("analytical original-roof volume", axis, order, op, mass.Volume, want, e)
				}
				var raw bytes.Buffer
				if e = r.WriteJSON(ctx, &raw); e != nil {
					t.Fatal(e)
				}
				loaded, e := ReadPolynomialTangencyEnvelopeJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, PolynomialTangencyEnvelopeOptions{})
				if e != nil || loaded.History() != r.History() {
					t.Fatal("tangent vertex or source ownership changed on reload", e)
				}
				checkNativePolynomialTangency(t, loaded, axis, order, op)
				for i, source := range r.sources {
					if !geom.SameSplineSurface(source, loaded.sources[i]) {
						t.Fatal("native recipe replaced original spline support", i)
					}
				}
				var replay bytes.Buffer
				if e = loaded.WriteJSON(ctx, &replay); e != nil || !bytes.Equal(raw.Bytes(), replay.Bytes()) {
					t.Fatal("native tangent proof is not canonical", e)
				}
				if axis == 0 && order == 0 && op == parametric.Intersection {
					outline, e := ValidateFaceTrimLoops(ctx, r.solid.faces[r.history.RoofFaces[1]], trimvalidate.Options{})
					if e != nil {
						t.Fatal(e)
					}
					t.Logf("generic tangent wedge trim status=%v reason=%q", outline.Status, outline.Reason)
					if outline.Status != trimvalidate.Certified {
						var ordinary bytes.Buffer
						if e := WriteJSON(&ordinary, r.Boundary()); e == nil || ordinary.Len() != 0 {
							t.Fatal("generic native writer bypassed its own trim proof", e)
						}
					}
				}
			}
		}
	}
}

func checkNativePolynomialTangency(t *testing.T, r *PolynomialTangencyEnvelope, axis, order int, op parametric.BooleanOp) {
	t.Helper()
	s, h := r.Boundary(), r.History()
	topology := s.Topology()
	if len(topology.Shells) != 1 || topology.EulerCharacteristic != 2 || topology.CollapsedEdges != 0 || len(s.faces) != 13 || s.faces[h.Base].Surface != r.sources[0] {
		t.Fatal("closed native tangent envelope", topology, h)
	}
	uv, pt := r.Contact()
	actual, e := geom.RepresentedExactPoint(pt)
	if e != nil || uv[axis].Cmp(big.NewRat(1, 2)) != 0 || uv[1-axis].Cmp(big.NewRat(23, 64)) != 0 || s.vertices[h.Vertices[4]] != actual {
		t.Fatal("exact common tangent vertex", h, uv, pt, e)
	}
	for i, face := range h.RoofFaces {
		sign := [4]int{1, -1, -1, 1}[i] * (1 - 2*order)
		want := 1
		if (sign > 0) == (op == parametric.Union) {
			want = 0
		}
		if h.RoofOwners[i] != want || s.faces[face].Surface != r.sources[1+want] || s.faces[face].Reversed {
			t.Fatal("source owner sign did not select the expected roof", i, h)
		}
	}
	for i, id := range h.Edges {
		edge := s.edges[id]
		if (edge.Start == h.Vertices[4]) == (edge.End == h.Vertices[4]) {
			t.Fatal("original edge lost common tangency identity", i, edge)
		}
		model := edge.Curve.(surfaceTrimCurve)
		q := model.curve.(ExactBezierUV)
		for _, parameter := range []*big.Rat{big.NewRat(1, 3), big.NewRat(2, 3)} {
			p, _, e := q.EvaluateRational(parameter)
			if e != nil {
				t.Fatal(e)
			}
			x := p[axis]
			f := new(big.Rat)
			for _, q := range []*big.Rat{big.NewRat(1, 8), big.NewRat(-3, 8), big.NewRat(3, 8), big.NewRat(1, 4)} {
				f.Mul(f, x)
				f.Add(f, q)
			}
			if i >= 2 {
				d := new(big.Rat).Sub(new(big.Rat).Mul(big.NewRat(2, 1), x), big.NewRat(1, 1))
				factor := new(big.Rat).Add(big.NewRat(1, 1), new(big.Rat).Mul(x, x))
				f.Add(f, new(big.Rat).Quo(new(big.Rat).Mul(factor, new(big.Rat).Mul(d, d)), big.NewRat(64, 1)))
			}
			if p[1-axis].Cmp(f) != 0 {
				t.Fatal("native curve is not the original tangent branch", i, p, f)
			}
		}
		var owners [2]int
		var directions [2]int
		for j, face := range h.RoofFaces {
			for _, coedge := range s.faces[face].Outer {
				if coedge.Edge != id {
					continue
				}
				owners[h.RoofOwners[j]]++
				expected := model.curve
				if coedge.Reversed {
					directions[1]++
					expected = reversePCurve(expected)
				} else {
					directions[0]++
				}
				if !matchesTangencyTrim(coedge.Curve, expected) {
					t.Fatal("original owner p-curve changed", i, face)
				}
			}
		}
		if owners != ([2]int{1, 1}) || directions != ([2]int{1, 1}) {
			t.Fatal("expected two opposite original coedge uses", owners, directions)
		}
	}
}

func TestNativePolynomialTangencyEnvelopeReloadRejections(t *testing.T) {
	ctx := context.Background()
	base, a, b := nativePolynomialTangencyFixture(t, 0, 2, 1)
	r, e := CombineSplineGraphPolynomialTangency(ctx, base, a, b, 0, parametric.Intersection, 1e-9, PolynomialTangencyEnvelopeOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var raw bytes.Buffer
	if e = r.WriteJSON(ctx, &raw); e != nil {
		t.Fatal(e)
	}
	d := a.Data()
	d.Poles[(d.CountU-1)*d.CountV].Z += 1.0 / 1024
	altered, e := geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
	if e != nil {
		t.Fatal(e)
	}
	alteredSource, e := parametric.EncodeSurface(altered)
	if e != nil {
		t.Fatal(e)
	}
	for name, change := range map[string]func(*polynomialTangencyEnvelopeRecipe){
		"vertex":       func(r *polynomialTangencyEnvelopeRecipe) { r.Vertices[4]++ },
		"point":        func(r *polynomialTangencyEnvelopeRecipe) { r.Point[2] = "1/1" },
		"edge":         func(r *polynomialTangencyEnvelopeRecipe) { r.Edges[0], r.Edges[1] = r.Edges[1], r.Edges[0] },
		"owner":        func(r *polynomialTangencyEnvelopeRecipe) { r.RoofOwners[1] ^= 1 },
		"source order": func(r *polynomialTangencyEnvelopeRecipe) { r.Sources[1], r.Sources[2] = r.Sources[2], r.Sources[1] },
		"overlap":      func(r *polynomialTangencyEnvelopeRecipe) { r.Sources[2] = r.Sources[1] },
		"later knot":   func(r *polynomialTangencyEnvelopeRecipe) { r.Sources[1] = alteredSource },
		"schema":       func(r *polynomialTangencyEnvelopeRecipe) { r.Version++ },
	} {
		t.Run(name, func(t *testing.T) {
			var recipe polynomialTangencyEnvelopeRecipe
			if e := json.Unmarshal(raw.Bytes(), &recipe); e != nil {
				t.Fatal(e)
			}
			change(&recipe)
			modified, e := json.Marshal(recipe)
			if e != nil {
				t.Fatal(e)
			}
			if q, e := ReadPolynomialTangencyEnvelopeJSON(ctx, bytes.NewReader(modified), JSONLimits{}, PolynomialTangencyEnvelopeOptions{}); q != nil || e == nil {
				t.Fatal("tampered tangency identity accepted", e)
			}
		})
	}
	for _, limits := range []JSONLimits{{MaxBytes: 4}, {MaxControlPoints: 4}, {MaxEntities: 3}} {
		if q, e := ReadPolynomialTangencyEnvelopeJSON(ctx, bytes.NewReader(raw.Bytes()), limits, PolynomialTangencyEnvelopeOptions{}); q != nil || e == nil {
			t.Fatal("native guard ignored", limits, e)
		}
	}
	if q, e := ReadPolynomialTangencyEnvelopeJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, PolynomialTangencyEnvelopeOptions{Proof: geom.GraphPointContactOptions{MaxOperations: 1}}); q != nil || !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("native work guard ignored", e)
	}
	if q, e := CombineSplineGraphPolynomialTangency(ctx, base, a, b, 0, parametric.Difference, 1e-9, PolynomialTangencyEnvelopeOptions{}); q != nil || !errors.Is(e, geom.ErrInvalid) {
		t.Fatal("Cut was admitted as an ordinary envelope", e)
	}
	base, a, b = nativePolynomialTangencyFixture(t, 0, 3, 1)
	if q, e := CombineSplineGraphPolynomialTangency(ctx, base, a, b, 0, parametric.Union, 1e-9, PolynomialTangencyEnvelopeOptions{}); q != nil || !errors.Is(e, geom.ErrUnrepresentable) {
		t.Fatal("unrepresentable tangent vertex was accepted", e)
	}
}
