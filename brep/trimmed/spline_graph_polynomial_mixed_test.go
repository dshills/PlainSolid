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

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

func nativePolynomialMixedFixture(t *testing.T, axis int, numerator, denominator int64) (*geom.BSplineSurface, *geom.BSplineSurface, *geom.BSplineSurface) {
	t.Helper()
	// g-f=(numerator-denominator*u)(2u-1)^2(1+u)/128.
	f := []*big.Rat{big.NewRat(1, 4), big.NewRat(3, 8), big.NewRat(-3, 8), big.NewRat(1, 8)}
	g := []*big.Rat{
		big.NewRat(32+numerator, 128), big.NewRat(48-3*numerator-denominator, 128), big.NewRat(-48+3*denominator, 128),
		big.NewRat(16+4*numerator, 128), big.NewRat(-4*denominator, 128),
	}
	return twoPolynomialRoofFixture(t, axis, f, g)
}

func polynomialMixedEnvelopeVolume(numerator int64, op parametric.BooleanOp) float64 {
	// The original height gap (105/128)(v-f)(v-g) integrates between
	// the branches to -105|g-f|^3/768. Integrate its absolute cubic
	// independently on the three exact free-coordinate slabs. Fuse adds
	// the net full-domain integral and subtracts that negative part.
	if numerator == 1 {
		if op == parametric.Union {
			return 707553348742991.0 / 228698418577408
		}
		return 686988440965297.0 / 228698418577408
	}
	if op == parametric.Union {
		return 7780551830432063.0 / 2515682604351488
	}
	return 7556874362525377.0 / 2515682604351488
}

func TestNativePolynomialMixedEnvelope(t *testing.T) {
	ctx := context.Background()
	for _, numerator := range []int64{1, 3} {
		for axis := 0; axis < 2; axis++ {
			base, a, b := nativePolynomialMixedFixture(t, axis, numerator, 4)
			for order, roofs := range [][2]*geom.BSplineSurface{{a, b}, {b, a}} {
				var patches [2]*geom.SurfacePatch
				for i, roof := range roofs {
					var e error
					patches[i], e = geom.NewSurfacePatch(roof, geom.SurfacePatchOptions{})
					if e != nil {
						t.Fatal(e)
					}
				}
				original, e := geom.DiscoverSplineGraphPolynomialMixedContact(ctx, patches[0], patches[1], axis, geom.GraphPointContactOptions{})
				if e != nil {
					t.Fatal(e)
				}
				for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union} {
					r, e := CombineSplineGraphPolynomialMixedContact(ctx, base, roofs[0], roofs[1], axis, op, 1e-9, PolynomialMixedEnvelopeOptions{})
					if e != nil {
						t.Fatal(numerator, axis, order, op, e)
					}
					mass, e := r.Boundary().MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
					want := polynomialMixedEnvelopeVolume(numerator, op)
					if e != nil || math.Abs(mass.Volume-want) > 1e-9 {
						t.Fatal("analytical original mixed volume", numerator, axis, order, op, mass.Volume, want, e)
					}
					var raw bytes.Buffer
					if e = r.WriteJSON(ctx, &raw); e != nil {
						t.Fatal(e)
					}
					loaded, e := ReadPolynomialMixedEnvelopeJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, PolynomialMixedEnvelopeOptions{})
					if e != nil || loaded.History() != r.History() || !reflect.DeepEqual(loaded.Contacts(), r.Contacts()) {
						t.Fatal("contact type and original boundary changed on reload", e)
					}
					for i, source := range r.sources {
						if !geom.SameSplineSurface(source, loaded.sources[i]) {
							t.Fatal("native recipe changed an original spline support", i)
						}
					}
					checkNativePolynomialMixedEnvelope(t, loaded, original, numerator, order, op)
					var rewritten bytes.Buffer
					if e = loaded.WriteJSON(ctx, &rewritten); e != nil || !bytes.Equal(raw.Bytes(), rewritten.Bytes()) {
						t.Fatal("canonical mixed contact reload", e)
					}
					if numerator == 1 && axis == 0 && order == 0 && op == parametric.Intersection {
						checkMixedEnvelopeOrdinaryWriter(t, r)
					}
				}
			}
		}
	}
}

func checkNativePolynomialMixedEnvelope(t *testing.T, r *PolynomialMixedEnvelope, proof geom.SplineGraphPolynomialMixedContact, numerator int64, order int, op parametric.BooleanOp) {
	t.Helper()
	h, s := r.History(), r.Boundary()
	topology := s.Topology()
	if len(topology.Shells) != 1 || topology.EulerCharacteristic != 2 || topology.CollapsedEdges != 0 || len(s.faces) != 14 ||
		s.faces[h.Base].Surface != r.sources[0] || len(s.faces[h.RoofFaces[2]].Outer) != 4 {
		t.Fatal("closed mixed shell with two subdivided lens branches", topology, h)
	}
	for i, face := range h.RoofFaces {
		sign := [5]int{1, -1, -1, -1, 1}[i] * (1 - 2*order)
		want := 1
		if (sign > 0) == (op == parametric.Union) {
			want = 0
		}
		if h.RoofOwners[i] != want || s.faces[face].Surface != r.sources[1+want] || s.faces[face].Reversed {
			t.Fatal("signed original roof ownership", i, h)
		}
	}
	seen := make(map[int]bool)
	axis := proof.FreeAxis
	for i, ids := range h.Edges {
		if (ids[1] >= 0) != (i == 1 || i == 4) {
			t.Fatal("interior lens requires two distinct subdivisions on each side", i, ids)
		}
		for _, id := range ids {
			if id < 0 {
				continue
			}
			if seen[id] {
				t.Fatal("native edge shared between distinct original restrictions", id)
			}
			seen[id] = true
			m := s.edges[id].Curve.(surfaceTrimCurve)
			checkNativeThreePolynomialBranch(t, m.curve, axis, proof.Power[proof.Edges[i].Branch])
			var owners, directions [2]int
			for j, fi := range h.RoofFaces {
				for _, use := range s.faces[fi].Outer {
					if use.Edge != id {
						continue
					}
					owners[h.RoofOwners[j]]++
					expected := m.curve
					if use.Reversed {
						expected = reversePCurve(expected)
						directions[1]++
					} else {
						directions[0]++
					}
					if !matchesTangencyTrim(use.Curve, expected) {
						t.Fatal("original roof coedge altered", i, j)
					}
				}
			}
			if owners != [2]int{1, 1} || directions != [2]int{1, 1} {
				t.Fatal("both original roof coedges in opposite directions", owners, directions)
			}
		}
	}
	for _, midpoint := range h.Midpoints {
		p := s.vertices[midpoint]
		if ([2]float64{p.X, p.Y})[axis] != float64(numerator+2)/8 {
			t.Fatal("exact original free station of the midpoint", midpoint, p)
		}
	}
	if len(seen) != 8 || h.Midpoints[0] == h.Midpoints[1] {
		t.Fatal("all original native edge and midpoint identities", h)
	}
	contacts := r.Contacts()
	for i, c := range contacts {
		node := proof.Nodes[i]
		source := proof.Vertices[node.Vertex]
		p, e := geom.RepresentedExactPoint(source.Point)
		if e != nil || c.Order != proof.ContactOrder[i] || c.DerivativeGap.Cmp(proof.DerivativeGap[i]) != 0 ||
			c.Vertex != h.Vertices[node.Vertex] || s.vertices[c.Vertex] != p ||
			!reflect.DeepEqual(c.Point, source.Point) || !reflect.DeepEqual(c.Parameters, source.Parameters) {
			t.Fatal("exact crossing or tangency certificate", i, c, e)
		}
		for j, source := range node.RayCycle {
			id := h.Edges[source][0]
			if h.Edges[source][1] >= 0 && proof.Edges[source].End == node.Vertex {
				id = h.Edges[source][1]
			}
			edge := s.edges[id]
			if c.Rays[j] != id || c.Sectors[j] != h.RoofFaces[node.Regions[j]] || (edge.Start == c.Vertex) == (edge.End == c.Vertex) {
				t.Fatal("original model ray and sector link", i, j, c)
			}
			rays := [2]int{c.Rays[j], c.Rays[(j+1)%4]}
			found := 0
			for _, use := range s.faces[c.Sectors[j]].Outer {
				if use.Edge == rays[0] || use.Edge == rays[1] {
					found++
				}
			}
			if found != 2 {
				t.Fatal("native face must occupy the certified sector between its rays", i, j, c)
			}
		}
	}
	if contacts[0].Vertex == contacts[1].Vertex || contacts[0].Order+contacts[1].Order != 3 {
		t.Fatal("crossing and tangency must have distinct native identities", contacts)
	}
	contacts[0].DerivativeGap.SetInt64(0)
	if r.Contacts()[0].DerivativeGap.Sign() == 0 {
		t.Fatal("returned contact mutated the retained proof")
	}
}

func checkMixedEnvelopeOrdinaryWriter(t *testing.T, r *PolynomialMixedEnvelope) {
	t.Helper()
	face := r.solid.faces[r.history.RoofFaces[2]]
	outline, e := ValidateFaceTrimLoops(context.Background(), face, trimvalidate.Options{})
	if e != nil {
		t.Fatal(e)
	}
	t.Logf("ordinary lens trim status=%v reason=%q", outline.Status, outline.Reason)
	if outline.Status != trimvalidate.Certified {
		var ordinary bytes.Buffer
		if e = WriteJSON(&ordinary, r.Boundary()); e == nil || ordinary.Len() != 0 {
			t.Fatal("ordinary native writer must require its own trim proof", e)
		}
	}
}

func TestNativePolynomialMixedEnvelopeReloadRejections(t *testing.T) {
	ctx := context.Background()
	base, a, b := nativePolynomialMixedFixture(t, 0, 1, 4)
	r, e := CombineSplineGraphPolynomialMixedContact(ctx, base, a, b, 0, parametric.Intersection, 1e-9, PolynomialMixedEnvelopeOptions{})
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
	source, e := parametric.EncodeSurface(altered)
	if e != nil {
		t.Fatal(e)
	}
	for name, change := range map[string]func(*polynomialMixedEnvelopeRecipe){
		"contact type":   func(r *polynomialMixedEnvelopeRecipe) { r.Ledger.Contacts[1].Order = 1 },
		"gap derivative": func(r *polynomialMixedEnvelopeRecipe) { r.Ledger.Contacts[0].DerivativeGap = "0" },
		"same vertex":    func(r *polynomialMixedEnvelopeRecipe) { r.Ledger.Contacts[1].Vertex = r.Ledger.Contacts[0].Vertex },
		"sector cycle":   func(r *polynomialMixedEnvelopeRecipe) { r.Ledger.Contacts[1].Sectors[0] = 5 },
		"lens edge":      func(r *polynomialMixedEnvelopeRecipe) { r.Ledger.Edges[1] = r.Ledger.Edges[1][:1] },
		"owner":          func(r *polynomialMixedEnvelopeRecipe) { r.Ledger.RoofOwners[2] ^= 1 },
		"source":         func(r *polynomialMixedEnvelopeRecipe) { r.Sources[1] = source },
		"tangent kind":   func(r *polynomialMixedEnvelopeRecipe) { r.Kind = nativePolynomialTangencyEnvelope },
	} {
		t.Run(name, func(t *testing.T) {
			var recipe polynomialMixedEnvelopeRecipe
			if e := json.Unmarshal(raw.Bytes(), &recipe); e != nil {
				t.Fatal(e)
			}
			change(&recipe)
			modified, e := json.Marshal(recipe)
			if e != nil {
				t.Fatal(e)
			}
			if out, e := ReadPolynomialMixedEnvelopeJSON(ctx, bytes.NewReader(modified), JSONLimits{}, PolynomialMixedEnvelopeOptions{}); out != nil || e == nil {
				t.Fatal("changed mixed contact proof accepted", e)
			}
		})
	}
	for _, limits := range []JSONLimits{{MaxBytes: 4}, {MaxControlPoints: 4}, {MaxEntities: 3}} {
		if out, e := ReadPolynomialMixedEnvelopeJSON(ctx, bytes.NewReader(raw.Bytes()), limits, PolynomialMixedEnvelopeOptions{}); out != nil || e == nil {
			t.Fatal("native recipe bound ignored", limits, e)
		}
	}
	lowBudget := PolynomialMixedEnvelopeOptions{Proof: geom.GraphPointContactOptions{MaxOperations: 1}}
	if out, e := ReadPolynomialMixedEnvelopeJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, lowBudget); out != nil || !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("proof work bound ignored", e)
	}
	if out, e := CombineSplineGraphPolynomialMixedContact(ctx, base, a, b, 0, parametric.Difference, 1e-9, PolynomialMixedEnvelopeOptions{}); out != nil || !errors.Is(e, geom.ErrInvalid) {
		t.Fatal("ordinary mixed envelope accepted Cut", e)
	}
	base, a, b = nativePolynomialMixedFixture(t, 0, 1, 3)
	if out, e := CombineSplineGraphPolynomialMixedContact(ctx, base, a, b, 0, parametric.Intersection, 1e-9, PolynomialMixedEnvelopeOptions{}); out != nil || !errors.Is(e, geom.ErrUnrepresentable) {
		t.Fatal("native mixed envelope rounded the crossing", e)
	}
}
