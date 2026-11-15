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
	"plainsolid/geom"
)

func nativeQuarticTangencyFixture(t *testing.T, axis, profile int) (*geom.BSplineSurface, *geom.BSplineSurface, *geom.BSplineSurface) {
	t.Helper()
	f := []*big.Rat{big.NewRat(1, 4), big.NewRat(3, 8), big.NewRat(-3, 8), big.NewRat(1, 8)}
	// Subtract f to obtain (u-1/2)^4/2, 3(u-3/8)^4/4,
	// or (5u-1)^4/1024 for the nonbinary control.
	g := [3][]*big.Rat{
		{big.NewRat(9, 32), big.NewRat(1, 8), big.NewRat(3, 8), big.NewRat(-7, 8), big.NewRat(1, 2)},
		{big.NewRat(4339, 16384), big.NewRat(111, 512), big.NewRat(33, 128), big.NewRat(-1, 1), big.NewRat(3, 4)},
		{big.NewRat(257, 1024), big.NewRat(91, 256), big.NewRat(-117, 512), big.NewRat(-93, 256), big.NewRat(625, 1024)},
	}
	return twoPolynomialRoofFixture(t, axis, f, g[profile])
}

func polynomialQuarticTangencyEnvelopeMeasures(profile int, op parametric.BooleanOp) (area, volume float64) {
	// Integrate the original quadratic height gap on the two signed slabs
	// with (105/128)(v³/3-(f+g)v²/2+fgv) as its dependent primitive.
	volumes := [2][2]float64{
		{327581661.0 / 109051904, 337235427.0 / 109051904},
		{43967126587620643.0 / 14636698788954112, 45251556705256157.0 / 14636698788954112},
	}
	station, scale := [2]float64{.5, .375}[profile], [2]float64{.5, .75}[profile]
	fuse := op == parametric.Union
	index := 0
	if fuse {
		index = 1
	}
	area = polynomialPairSourceEnvelopeArea(fuse, []float64{0, station, 1}, func(u float64) (f, g, df, dg float64) {
		f = ((u/8-3.0/8)*u+3.0/8)*u + .25
		df = (3*u/8-.75)*u + .375
		x := u - station
		g = f + scale*x*x*x*x
		dg = df + 4*scale*x*x*x
		return f, g, df, dg
	})
	return area, volumes[profile][index]
}

func TestNativePolynomialQuarticTangencyEnvelope(t *testing.T) {
	ctx := context.Background()
	for profile := 0; profile < 2; profile++ {
		for axis := 0; axis < 2; axis++ {
			base, a, b := nativeQuarticTangencyFixture(t, axis, profile)
			for order, roofs := range [][2]*geom.BSplineSurface{{a, b}, {b, a}} {
				var patches [2]*geom.SurfacePatch
				for i, roof := range roofs {
					var e error
					patches[i], e = geom.NewSurfacePatch(roof, geom.SurfacePatchOptions{})
					if e != nil {
						t.Fatal(e)
					}
				}
				proof, e := geom.DiscoverSplineGraphPolynomialQuarticTangency(ctx, patches[0], patches[1], axis, geom.GraphPointContactOptions{})
				if e != nil {
					t.Fatal(e)
				}
				for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union} {
					r, e := CombineSplineGraphPolynomialQuarticTangency(ctx, base, roofs[0], roofs[1], axis, op, 1e-9, PolynomialQuarticTangencyEnvelopeOptions{})
					if e != nil {
						t.Fatal(profile, axis, order, op, e)
					}
					var raw bytes.Buffer
					if e = r.WriteJSON(ctx, &raw); e != nil {
						t.Fatal(e)
					}
					loaded, e := ReadPolynomialQuarticTangencyEnvelopeJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, PolynomialQuarticTangencyEnvelopeOptions{})
					if e != nil || loaded.History() != r.History() || !reflect.DeepEqual(loaded.Contact(), r.Contact()) {
						t.Fatal("source reload changed native owner and order-four contact ledgers", profile, axis, order, op, e)
					}
					for i, source := range r.sources {
						if !geom.SameSplineSurface(source, loaded.sources[i]) {
							t.Fatal("reloaded native support changed original coefficients", i)
						}
					}
					checkNativePolynomialQuarticTangencyEnvelope(t, loaded, proof, order, op)
					wantArea, wantVolume := polynomialQuarticTangencyEnvelopeMeasures(profile, op)
					got, e := loaded.Boundary().MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
					if e != nil || math.Abs(got.Volume-wantVolume) > 1e-9 || math.Abs(got.Area-wantArea) > 1e-8 {
						t.Fatal("independent original roof volume and area", profile, axis, order, op, got.Volume, wantVolume, got.Area, wantArea, e)
					}
					var replay bytes.Buffer
					if e = loaded.WriteJSON(ctx, &replay); e != nil || !bytes.Equal(raw.Bytes(), replay.Bytes()) {
						t.Fatal("canonical quartic-tangency source recipe", e)
					}

				}
			}
		}
	}
}

func checkNativePolynomialQuarticTangencyEnvelope(t *testing.T, r *PolynomialQuarticTangencyEnvelope, proof geom.SplineGraphPolynomialQuarticTangency, order int, op parametric.BooleanOp) {
	t.Helper()
	s, h := r.Boundary(), r.History()
	topology := s.Topology()
	if len(topology.Shells) != 1 || topology.EulerCharacteristic != 2 || topology.CollapsedEdges != 0 || len(s.faces) != 13 ||
		s.faces[h.Base].Surface != r.sources[0] {
		t.Fatal("closed single shell with all four original roof regions", topology, h)
	}
	for i, face := range h.RoofFaces {
		owner := 1
		sign := [4]int{1, -1, -1, 1}[i] * (1 - 2*order)
		if (sign > 0) == (op == parametric.Union) {
			owner = 0
		}
		if h.RoofOwners[i] != owner || s.faces[face].Surface != r.sources[1+owner] || s.faces[face].Reversed {
			t.Fatal("signed selection on the two original roofs", i, h)
		}
	}

	_, xyz, e := proof.RepresentedVertices()
	if e != nil {
		t.Fatal(e)
	}
	axis := proof.FreeAxis
	degrees := make([]int, len(s.vertices))
	for _, edge := range s.edges {
		degrees[edge.Start]++
		degrees[edge.End]++
	}
	vertices := make(map[int]bool)
	for i, v := range h.Vertices {
		if vertices[v] || s.vertices[v] != xyz[i] || i >= 4 && degrees[v] != 4 {
			t.Fatal("distinct exact source vertex identities", i, v)
		}
		vertices[v] = true
	}

	edges := make(map[int]bool)
	for i, chain := range h.Edges {
		source := proof.Edges[i]
		edgeID := chain[0]
		edge := s.edges[edgeID]
		start, end := h.Vertices[source.Start], h.Vertices[source.End]
		if chain[1] != -1 || edges[edgeID] || (edge.Start != start || edge.End != end) && (edge.Start != end || edge.End != start) {
			t.Fatal("native edge preserves the complete original restriction", i, chain, edge)
		}
		edges[edgeID] = true
		curve := edge.Curve.(surfaceTrimCurve).curve
		checkNativeThreePolynomialBranch(t, curve, axis, proof.Power[source.Branch])
		var owners, dirs [2]int
		for k, face := range h.RoofFaces {
			for _, use := range s.faces[face].Outer {
				if use.Edge != edgeID {
					continue
				}
				owners[h.RoofOwners[k]]++
				expected := curve
				if use.Reversed {
					expected = reversePCurve(curve)
					dirs[1]++
				} else {
					dirs[0]++
				}
				if !matchesTangencyTrim(use.Curve, expected) {
					t.Fatal("paired original owner coedge restriction", i, k, use)
				}
			}
		}
		if owners != [2]int{1, 1} || dirs != [2]int{1, 1} {
			t.Fatal("each original section edge uses both owners in opposite directions", i, edgeID, owners, dirs)
		}
	}
	if len(edges) != 4 {
		t.Fatal("complete original native section edge ledger", edges)
	}

	contact, node := r.Contact(), proof.Node
	source := proof.Vertices[node.Vertex]
	if contact.Order != 4 || contact.DerivativeGap.Cmp(proof.FourthDerivativeGap) != 0 ||
		contact.Vertex != h.Vertices[node.Vertex] || !reflect.DeepEqual(contact.Point, source.Point) ||
		!reflect.DeepEqual(contact.Parameters, source.Parameters) {
		t.Fatal("exact order-four native junction", contact)
	}
	for j, edge := range node.RayCycle {
		id := h.Edges[edge][0]
		if contact.Rays[j] != id || contact.Sectors[j] != h.RoofFaces[node.Regions[j]] {
			t.Fatal("one-sided original rays and signed sector incidence", j, contact)
		}
		matched := 0
		for _, use := range s.faces[contact.Sectors[j]].Outer {
			if use.Edge == id || use.Edge == contact.Rays[(j+1)%4] {
				matched++
			}
		}
		if matched != 2 {
			t.Fatal("both adjacent native rays bound the original roof sector", j, contact)
		}
	}
	contact.DerivativeGap.SetInt64(0)
	if r.Contact().DerivativeGap.Sign() == 0 {
		t.Fatal("returned rational contact aliases the original proof")
	}
}

func TestNativePolynomialQuarticTangencyEnvelopeRejections(t *testing.T) {
	ctx := context.Background()
	base, a, b := nativeQuarticTangencyFixture(t, 0, 0)
	r, e := CombineSplineGraphPolynomialQuarticTangency(ctx, base, a, b, 0, parametric.Intersection, 1e-9, PolynomialQuarticTangencyEnvelopeOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var raw bytes.Buffer
	if e = r.WriteJSON(ctx, &raw); e != nil {
		t.Fatal(e)
	}
	d := a.Data()
	d.Poles[(d.CountU-1)*d.CountV].Z += 1.0 / 1024
	changed, e := geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
	if e != nil {
		t.Fatal(e)
	}
	source, e := parametric.EncodeSurface(changed)
	if e != nil {
		t.Fatal(e)
	}
	for name, mutate := range map[string]func(*polynomialEnvelopeRecipe){
		"later source cell":     func(r *polynomialEnvelopeRecipe) { r.Sources[1] = source },
		"contact order":         func(r *polynomialEnvelopeRecipe) { r.Ledger.Contacts[0].Order = 1 },
		"fourth derivative gap": func(r *polynomialEnvelopeRecipe) { r.Ledger.Contacts[0].DerivativeGap = "0" },
		"contact point":         func(r *polynomialEnvelopeRecipe) { r.Ledger.Contacts[0].Point[0] = "0" },
		"sector":                func(r *polynomialEnvelopeRecipe) { r.Ledger.Contacts[0].Sectors[0] = r.Ledger.Contacts[0].Sectors[1] },
		"ray":                   func(r *polynomialEnvelopeRecipe) { r.Ledger.Contacts[0].Rays[0] = r.Ledger.Contacts[0].Rays[1] },
		"roof owner":            func(r *polynomialEnvelopeRecipe) { r.Ledger.RoofOwners[2] ^= 1 },
		"lost branch":           func(r *polynomialEnvelopeRecipe) { r.Ledger.Edges[1] = r.Ledger.Edges[1][:0] },
		"recipe kind":           func(r *polynomialEnvelopeRecipe) { r.Kind = nativePolynomialCubicCrossingEnvelope },
	} {
		t.Run(name, func(t *testing.T) {
			var recipe polynomialEnvelopeRecipe
			if e := json.Unmarshal(raw.Bytes(), &recipe); e != nil {
				t.Fatal(e)
			}
			mutate(&recipe)
			data, e := json.Marshal(recipe)
			if e != nil {
				t.Fatal(e)
			}
			if r, e := ReadPolynomialQuarticTangencyEnvelopeJSON(ctx, bytes.NewReader(data), JSONLimits{}, PolynomialQuarticTangencyEnvelopeOptions{}); r != nil || e == nil {
				t.Fatal("changed original support or order-four contact ledger accepted", e)
			}
		})
	}
	for _, limits := range []JSONLimits{{MaxBytes: 1}, {MaxControlPoints: 4}, {MaxEntities: 3}} {
		if r, e := ReadPolynomialQuarticTangencyEnvelopeJSON(ctx, bytes.NewReader(raw.Bytes()), limits, PolynomialQuarticTangencyEnvelopeOptions{}); r != nil || e == nil {
			t.Fatal("recipe reader ignored limits", limits, e)
		}
	}
	lowBudget := PolynomialQuarticTangencyEnvelopeOptions{Proof: geom.GraphPointContactOptions{MaxOperations: 1}}
	if r, e := ReadPolynomialQuarticTangencyEnvelopeJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, lowBudget); r != nil || !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("reproof ignored its work limit", e)
	}
	if r, e := CombineSplineGraphPolynomialQuarticTangency(ctx, base, a, b, 0, parametric.Difference, 1e-9, PolynomialQuarticTangencyEnvelopeOptions{}); r != nil || !errors.Is(e, geom.ErrInvalid) {
		t.Fatal("Common/Fuse recipe admitted point-contacting Cut", e)
	}
	base, a, b = nativeQuarticTangencyFixture(t, 0, 2)
	if r, e := CombineSplineGraphPolynomialQuarticTangency(ctx, base, a, b, 0, parametric.Union, 1e-9, PolynomialQuarticTangencyEnvelopeOptions{}); r != nil || !errors.Is(e, geom.ErrUnrepresentable) {
		t.Fatal("native proof rounded a nonbinary order-four tangency", e)
	}
}
