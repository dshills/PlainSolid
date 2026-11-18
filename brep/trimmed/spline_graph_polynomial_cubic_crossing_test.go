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
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

func nativeCubicCrossingFixture(t *testing.T, axis, profile int) (*geom.BSplineSurface, *geom.BSplineSurface, *geom.BSplineSurface) {
	t.Helper()
	f := []*big.Rat{big.NewRat(1, 4), big.NewRat(3, 8), big.NewRat(-3, 8), big.NewRat(1, 8)}
	// The original separations are -(u-1/2)^3/4, -(1+u)(u-3/8)^3/4,
	// and -(3u-1)^3/128 for the nonbinary coordinate control.
	g := [3][]*big.Rat{
		{big.NewRat(9, 32), big.NewRat(3, 16), new(big.Rat), big.NewRat(-1, 8)},
		{big.NewRat(539, 2048), big.NewRat(579, 2048), big.NewRat(-51, 256), big.NewRat(5, 32), big.NewRat(-1, 4)},
		{big.NewRat(33, 128), big.NewRat(39, 128), big.NewRat(-21, 128), big.NewRat(-11, 128)},
	}
	return twoPolynomialRoofFixture(t, axis, f, g[profile])
}

func polynomialCubicCrossingEnvelopeMeasures(profile int, op parametric.BooleanOp) (area, volume float64) {
	// Integrate the original quadratic height gap on each signed slab,
	// using the dependent primitive (105/128)(v³/3-(f+g)v²/2+fgv).
	volumes := [2][2]float64{
		{50397177.0 / 16777216, 51892359.0 / 16777216},
		{703472986951436809.0 / 234187180623265792, 724794756423300599.0 / 234187180623265792},
	}
	r := [2]float64{.5, .375}[profile]
	fuse := op == parametric.Union
	index := 0
	if fuse {
		index = 1
	}
	area = polynomialPairSourceEnvelopeArea(fuse, []float64{0, r, 1}, func(u float64) (f, g, df, dg float64) {
		f = ((u/8-3.0/8)*u+3.0/8)*u + .25
		df = (3*u/8-.75)*u + .375
		x := u - r
		scale, scaleDerivative := -.25, 0.0
		if profile == 1 {
			scale, scaleDerivative = -(1+u)/4, -.25
		}
		g = f + scale*x*x*x
		dg = df + x*x*(3*scale+x*scaleDerivative)
		return f, g, df, dg
	})
	return area, volumes[profile][index]
}

func TestNativePolynomialCubicCrossingEnvelope(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	for profile := 0; profile < 2; profile++ {
		for axis := 0; axis < 2; axis++ {
			base, a, b := nativeCubicCrossingFixture(t, axis, profile)
			for order, roofs := range [][2]*geom.BSplineSurface{{a, b}, {b, a}} {
				var patches [2]*geom.SurfacePatch
				for i, roof := range roofs {
					var e error
					patches[i], e = geom.NewSurfacePatch(roof, geom.SurfacePatchOptions{})
					if e != nil {
						t.Fatal(e)
					}
				}
				proof, e := geom.DiscoverSplineGraphPolynomialCubicCrossing(ctx, patches[0], patches[1], axis, geom.GraphPointContactOptions{})
				if e != nil {
					t.Fatal(e)
				}
				for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union} {
					r, e := CombineSplineGraphPolynomialCubicCrossing(ctx, base, roofs[0], roofs[1], axis, op, 1e-9, PolynomialCubicCrossingEnvelopeOptions{})
					if e != nil {
						t.Fatal(profile, axis, order, op, e)
					}
					var raw bytes.Buffer
					if e = r.WriteJSON(ctx, &raw); e != nil {
						t.Fatal(e)
					}
					loaded, e := ReadPolynomialCubicCrossingEnvelopeJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, PolynomialCubicCrossingEnvelopeOptions{})
					if e != nil || loaded.History() != r.History() || !reflect.DeepEqual(loaded.Contact(), r.Contact()) {
						t.Fatal("source reload changed native owner and order-three contact ledgers", profile, axis, order, op, e)
					}
					for i, source := range r.sources {
						if !geom.SameSplineSurface(source, loaded.sources[i]) {
							t.Fatal("reloaded native support changed original coefficients", i)
						}
					}
					checkNativePolynomialCubicCrossingEnvelope(t, loaded, proof, order, op)
					wantArea, wantVolume := polynomialCubicCrossingEnvelopeMeasures(profile, op)
					got, e := loaded.Boundary().MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
					if e != nil || math.Abs(got.Volume-wantVolume) > 1e-9 || math.Abs(got.Area-wantArea) > 1e-8 {
						t.Fatal("independent original roof volume and area", profile, axis, order, op, got.Volume, wantVolume, got.Area, wantArea, e)
					}
					var replay bytes.Buffer
					if e = loaded.WriteJSON(ctx, &replay); e != nil || !bytes.Equal(raw.Bytes(), replay.Bytes()) {
						t.Fatal("canonical cubic-crossing source recipe", e)
					}
					if profile == 0 && axis == 0 && order == 0 && op == parametric.Intersection {
						between := loaded.solid.faces[loaded.history.RoofFaces[1]]
						regular, e := ValidateFaceTrimLoops(ctx, between, trimvalidate.Options{})
						if e != nil {
							t.Fatal(e)
						}
						if regular.Status != trimvalidate.Certified {
							var ordinary bytes.Buffer
							if e := WriteJSON(&ordinary, loaded.Boundary()); e == nil || ordinary.Len() != 0 {
								t.Fatal("ordinary writer ignored its trim gate", e)
							}
						}
					}
				}
			}
		}
	}
}

func checkNativePolynomialCubicCrossingEnvelope(t *testing.T, r *PolynomialCubicCrossingEnvelope, proof geom.SplineGraphPolynomialCubicCrossing, order int, op parametric.BooleanOp) {
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
	if contact.Order != 3 || contact.DerivativeGap.Cmp(proof.ThirdDerivativeGap) != 0 ||
		contact.Vertex != h.Vertices[node.Vertex] || !reflect.DeepEqual(contact.Point, source.Point) ||
		!reflect.DeepEqual(contact.Parameters, source.Parameters) {
		t.Fatal("exact order-three native junction", contact)
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

func TestNativePolynomialCubicCrossingEnvelopeRejections(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	base, a, b := nativeCubicCrossingFixture(t, 0, 0)
	r, e := CombineSplineGraphPolynomialCubicCrossing(ctx, base, a, b, 0, parametric.Intersection, 1e-9, PolynomialCubicCrossingEnvelopeOptions{})
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
		"later source cell":    func(r *polynomialEnvelopeRecipe) { r.Sources[1] = source },
		"contact order":        func(r *polynomialEnvelopeRecipe) { r.Ledger.Contacts[0].Order = 1 },
		"third derivative gap": func(r *polynomialEnvelopeRecipe) { r.Ledger.Contacts[0].DerivativeGap = "0" },
		"contact point":        func(r *polynomialEnvelopeRecipe) { r.Ledger.Contacts[0].Point[0] = "0" },
		"sector":               func(r *polynomialEnvelopeRecipe) { r.Ledger.Contacts[0].Sectors[0] = r.Ledger.Contacts[0].Sectors[1] },
		"ray":                  func(r *polynomialEnvelopeRecipe) { r.Ledger.Contacts[0].Rays[0] = r.Ledger.Contacts[0].Rays[1] },
		"roof owner":           func(r *polynomialEnvelopeRecipe) { r.Ledger.RoofOwners[2] ^= 1 },
		"lost branch":          func(r *polynomialEnvelopeRecipe) { r.Ledger.Edges[1] = r.Ledger.Edges[1][:0] },
		"recipe kind":          func(r *polynomialEnvelopeRecipe) { r.Kind = nativePolynomialTwoTangenciesEnvelope },
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
			if r, e := ReadPolynomialCubicCrossingEnvelopeJSON(ctx, bytes.NewReader(data), JSONLimits{}, PolynomialCubicCrossingEnvelopeOptions{}); r != nil || e == nil {
				t.Fatal("changed original support or order-three contact ledger accepted", e)
			}
		})
	}
	for _, limits := range []JSONLimits{{MaxBytes: 1}, {MaxControlPoints: 4}, {MaxEntities: 3}} {
		if r, e := ReadPolynomialCubicCrossingEnvelopeJSON(ctx, bytes.NewReader(raw.Bytes()), limits, PolynomialCubicCrossingEnvelopeOptions{}); r != nil || e == nil {
			t.Fatal("recipe reader ignored limits", limits, e)
		}
	}
	lowBudget := PolynomialCubicCrossingEnvelopeOptions{Proof: geom.GraphPointContactOptions{MaxOperations: 1}}
	if r, e := ReadPolynomialCubicCrossingEnvelopeJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, lowBudget); r != nil || !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("reproof ignored its work limit", e)
	}
	if r, e := CombineSplineGraphPolynomialCubicCrossing(ctx, base, a, b, 0, parametric.Difference, 1e-9, PolynomialCubicCrossingEnvelopeOptions{}); r != nil || !errors.Is(e, geom.ErrInvalid) {
		t.Fatal("Common/Fuse recipe admitted point-contacting Cut", e)
	}
	base, a, b = nativeCubicCrossingFixture(t, 0, 2)
	if r, e := CombineSplineGraphPolynomialCubicCrossing(ctx, base, a, b, 0, parametric.Union, 1e-9, PolynomialCubicCrossingEnvelopeOptions{}); r != nil || !errors.Is(e, geom.ErrUnrepresentable) {
		t.Fatal("native proof rounded a nonbinary order-three crossing", e)
	}
}
