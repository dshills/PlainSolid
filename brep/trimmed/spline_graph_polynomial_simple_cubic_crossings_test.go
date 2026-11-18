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

func nativeSimpleCubicCrossingsFixture(t *testing.T, axis, profile int) (*geom.BSplineSurface, *geom.BSplineSurface, *geom.BSplineSurface) {
	t.Helper()
	f := []*big.Rat{big.NewRat(1, 4), big.NewRat(3, 8), big.NewRat(-3, 8), big.NewRat(1, 8)}
	// The original separations are (u-1/4)^3(u-3/4)/2,
	// 3(u-1/4)(u-3/4)^3/4, and (5u-1)^3(4u-3)/1024.
	g := [3][]*big.Rat{
		{big.NewRat(131, 512), big.NewRat(19, 64), new(big.Rat), big.NewRat(-5, 8), big.NewRat(1, 2)},
		{big.NewRat(337, 1024), big.NewRat(-33, 128), big.NewRat(21, 16), big.NewRat(-7, 4), big.NewRat(3, 4)},
		{big.NewRat(259, 1024), big.NewRat(335, 1024), big.NewRat(-99, 1024), big.NewRat(-547, 1024), big.NewRat(500, 1024)},
	}
	return twoPolynomialRoofFixture(t, axis, f, g[profile])
}

func polynomialSimpleCubicCrossingsEnvelopeMeasures(profile int, op parametric.BooleanOp) (area, volume float64) {
	// Integrate the original quadratic height gap over the three signed
	// slabs using (105/128)(v³/3-(f+g)v²/2+fgv) as its dependent primitive.
	volumes := [2][2]float64{
		{103213407107.0 / 34359738368, 106269811837.0 / 34359738368},
		{825706768081.0 / 274877906944, 849948809519.0 / 274877906944},
	}
	params := [2][3]float64{{.25, .75, .5}, {.75, .25, .75}}
	cubic, simple, scale := params[profile][0], params[profile][1], params[profile][2]
	fuse := op == parametric.Union
	index := 0
	if fuse {
		index = 1
	}
	area = polynomialPairSourceEnvelopeArea(fuse, []float64{0, .25, .75, 1}, func(u float64) (f, g, df, dg float64) {
		f = ((u/8-3.0/8)*u+3.0/8)*u + .25
		df = (3*u/8-.75)*u + .375
		x, y := u-cubic, u-simple
		g = f + scale*x*x*x*y
		dg = df + scale*x*x*(x+3*y)
		return f, g, df, dg
	})
	return area, volumes[profile][index]
}

func TestNativePolynomialSimpleCubicCrossingsEnvelope(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	for profile := 0; profile < 2; profile++ {
		for axis := 0; axis < 2; axis++ {
			base, a, b := nativeSimpleCubicCrossingsFixture(t, axis, profile)
			for order, roofs := range [][2]*geom.BSplineSurface{{a, b}, {b, a}} {
				var patches [2]*geom.SurfacePatch
				for i, roof := range roofs {
					var e error
					patches[i], e = geom.NewSurfacePatch(roof, geom.SurfacePatchOptions{})
					if e != nil {
						t.Fatal(e)
					}
				}
				proof, e := geom.DiscoverSplineGraphPolynomialSimpleCubicCrossings(ctx, patches[0], patches[1], axis, geom.GraphPointContactOptions{})
				if e != nil {
					t.Fatal(e)
				}
				for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union} {
					r, e := CombineSplineGraphPolynomialSimpleCubicCrossings(ctx, base, roofs[0], roofs[1], axis, op, 1e-9, PolynomialSimpleCubicCrossingsEnvelopeOptions{})
					if e != nil {
						t.Fatal(profile, axis, order, op, e)
					}
					var raw bytes.Buffer
					if e = r.WriteJSON(ctx, &raw); e != nil {
						t.Fatal(e)
					}
					loaded, e := ReadPolynomialSimpleCubicCrossingsEnvelopeJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, PolynomialSimpleCubicCrossingsEnvelopeOptions{})
					if e != nil || loaded.History() != r.History() || !reflect.DeepEqual(loaded.Contacts(), r.Contacts()) {
						t.Fatal("source reload changed native owner and crossing ledgers", profile, axis, order, op, e)
					}
					for i, source := range r.sources {
						if !geom.SameSplineSurface(source, loaded.sources[i]) {
							t.Fatal("reloaded native support changed original coefficients", i)
						}
					}
					checkNativePolynomialSimpleCubicCrossingsEnvelope(t, loaded, proof, profile, order, op)
					wantArea, wantVolume := polynomialSimpleCubicCrossingsEnvelopeMeasures(profile, op)
					got, e := loaded.Boundary().MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
					if e != nil || math.Abs(got.Volume-wantVolume) > 1e-9 || math.Abs(got.Area-wantArea) > 1e-8 {
						t.Fatal("independent original roof volume and area", profile, axis, order, op, got.Volume, wantVolume, got.Area, wantArea, e)
					}
					var replay bytes.Buffer
					if e = loaded.WriteJSON(ctx, &replay); e != nil || !bytes.Equal(raw.Bytes(), replay.Bytes()) {
						t.Fatal("canonical simple-cubic-crossing source recipe", e)
					}

				}
			}
		}
	}
}

func checkNativePolynomialSimpleCubicCrossingsEnvelope(t *testing.T, r *PolynomialSimpleCubicCrossingsEnvelope, proof geom.SplineGraphPolynomialSimpleCubicCrossings, profile, order int, op parametric.BooleanOp) {
	t.Helper()
	s, h := r.Boundary(), r.History()
	topology := s.Topology()
	if len(topology.Shells) != 1 || topology.EulerCharacteristic != 2 || topology.CollapsedEdges != 0 || len(s.faces) != 14 ||
		s.faces[h.Base].Surface != r.sources[0] || len(s.faces[h.RoofFaces[2]].Outer) != 4 {
		t.Fatal("closed single shell with all five roof regions and both subdivided lens sides", topology, h)
	}
	for i, face := range h.RoofFaces {
		owner := 1
		sign := [5]int{1, -1, -1, -1, 1}[i] * (1 - 2*order)
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
	for _, v := range h.Midpoints {
		p := s.vertices[v]
		if vertices[v] || degrees[v] != 2 || ([2]float64{p.X, p.Y})[axis] != .5 {
			t.Fatal("distinct exact lens-side subdivision vertex", v, p)
		}
		vertices[v] = true
	}

	edges := make(map[int]bool)
	for i, chain := range h.Edges {
		source := proof.Edges[i]
		stations := []int{h.Vertices[source.Start], h.Vertices[source.End]}
		if i%3 == 1 {
			stations = []int{stations[0], h.Midpoints[source.Branch], stations[1]}
		}
		if (chain[1] >= 0) != (len(stations) == 3) {
			t.Fatal("one chain for each complete original restriction", i, chain)
		}
		for j, edgeID := range chain[:len(stations)-1] {
			edge := s.edges[edgeID]
			if edges[edgeID] || (edge.Start != stations[j] || edge.End != stations[j+1]) &&
				(edge.Start != stations[j+1] || edge.End != stations[j]) {
				t.Fatal("native branch restriction endpoints", i, edge)
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
	}
	if len(edges) != 8 {
		t.Fatal("complete original native section edge ledger", edges)
	}

	for i, contact := range r.Contacts() {
		node := proof.Nodes[i]
		source := proof.Vertices[node.Vertex]
		wantOrder := [2][2]int{{3, 1}, {1, 3}}[profile][i]
		if contact.Order != wantOrder || contact.Order != proof.ContactOrder[i] || contact.DerivativeGap.Cmp(proof.DerivativeGap[i]) != 0 ||
			contact.Vertex != h.Vertices[node.Vertex] || !reflect.DeepEqual(contact.Point, source.Point) ||
			!reflect.DeepEqual(contact.Parameters, source.Parameters) {
			t.Fatal("original native crossing type and derivative gap", i, contact)
		}
		for j, edge := range node.RayCycle {
			id := h.Edges[edge][0]
			if h.Edges[edge][1] >= 0 && proof.Edges[edge].End == node.Vertex {
				id = h.Edges[edge][1]
			}
			if contact.Rays[j] != id || contact.Sectors[j] != h.RoofFaces[node.Regions[j]] {
				t.Fatal("one-sided original ray and sector incidence", i, j, contact)
			}
			matched := 0
			for _, use := range s.faces[contact.Sectors[j]].Outer {
				if use.Edge == id || use.Edge == contact.Rays[(j+1)%4] {
					matched++
				}
			}
			if matched != 2 {
				t.Fatal("both adjacent native rays bound the original roof sector", i, j, contact)
			}
		}
	}
}

func TestNativePolynomialSimpleCubicCrossingsEnvelopeRejections(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	base, a, b := nativeSimpleCubicCrossingsFixture(t, 0, 0)
	r, e := CombineSplineGraphPolynomialSimpleCubicCrossings(ctx, base, a, b, 0, parametric.Intersection, 1e-9, PolynomialSimpleCubicCrossingsEnvelopeOptions{})
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
		"later source cell": func(r *polynomialEnvelopeRecipe) { r.Sources[1] = source },
		"contact order":     func(r *polynomialEnvelopeRecipe) { r.Ledger.Contacts[0].Order = 2 },
		"derivative gap":    func(r *polynomialEnvelopeRecipe) { r.Ledger.Contacts[1].DerivativeGap = "0" },
		"second point":      func(r *polynomialEnvelopeRecipe) { r.Ledger.Contacts[1].Point[0] = "0" },
		"sector":            func(r *polynomialEnvelopeRecipe) { r.Ledger.Contacts[0].Sectors[0] = r.Ledger.Contacts[0].Sectors[1] },
		"ray":               func(r *polynomialEnvelopeRecipe) { r.Ledger.Contacts[0].Rays[0] = r.Ledger.Contacts[0].Rays[1] },
		"roof owner":        func(r *polynomialEnvelopeRecipe) { r.Ledger.RoofOwners[2] ^= 1 },
		"lost lens side":    func(r *polynomialEnvelopeRecipe) { r.Ledger.Edges[1] = r.Ledger.Edges[1][:1] },
		"recipe kind":       func(r *polynomialEnvelopeRecipe) { r.Kind = nativePolynomialMixedEnvelope },
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
			if r, e := ReadPolynomialSimpleCubicCrossingsEnvelopeJSON(ctx, bytes.NewReader(data), JSONLimits{}, PolynomialSimpleCubicCrossingsEnvelopeOptions{}); r != nil || e == nil {
				t.Fatal("changed original support or crossing ledger accepted", e)
			}
		})
	}
	for _, limits := range []JSONLimits{{MaxBytes: 1}, {MaxControlPoints: 4}, {MaxEntities: 3}} {
		if r, e := ReadPolynomialSimpleCubicCrossingsEnvelopeJSON(ctx, bytes.NewReader(raw.Bytes()), limits, PolynomialSimpleCubicCrossingsEnvelopeOptions{}); r != nil || e == nil {
			t.Fatal("recipe reader ignored limits", limits, e)
		}
	}
	lowBudget := PolynomialSimpleCubicCrossingsEnvelopeOptions{Proof: geom.GraphPointContactOptions{MaxOperations: 1}}
	if r, e := ReadPolynomialSimpleCubicCrossingsEnvelopeJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, lowBudget); r != nil || !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("reproof ignored its work limit", e)
	}
	if r, e := CombineSplineGraphPolynomialSimpleCubicCrossings(ctx, base, a, b, 0, parametric.Difference, 1e-9, PolynomialSimpleCubicCrossingsEnvelopeOptions{}); r != nil || !errors.Is(e, geom.ErrInvalid) {
		t.Fatal("Common/Fuse recipe admitted point-contacting Cut", e)
	}
	base, a, b = nativeSimpleCubicCrossingsFixture(t, 0, 2)
	if r, e := CombineSplineGraphPolynomialSimpleCubicCrossings(ctx, base, a, b, 0, parametric.Union, 1e-9, PolynomialSimpleCubicCrossingsEnvelopeOptions{}); r != nil || !errors.Is(e, geom.ErrUnrepresentable) {
		t.Fatal("native proof rounded a nonbinary cubic crossing", e)
	}
}
