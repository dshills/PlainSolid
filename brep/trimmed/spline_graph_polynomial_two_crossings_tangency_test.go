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

func nativeTwoCrossingsTangencyFixture(t *testing.T, axis int, tangent, firstDenominator int64) (*geom.BSplineSurface, *geom.BSplineSurface, *geom.BSplineSurface) {
	t.Helper()
	return nativeQuarticRoofPairFixture(t, axis, big.NewRat(1, 256),
		[4][2]int64{{1, firstDenominator}, {2, 4}, {3, 4}, {tangent, 4}})
}

func polynomialTwoCrossingsTangencyMeasures(tangent int64, op parametric.BooleanOp) (area, volume float64) {
	// The original height difference integrates between the branches to
	// -105|g-f|³/768. The exact fractions integrate its four slab terms
	// and the signed full-square contribution to the selected envelope.
	var volumeByOperation = [3][2]float64{
		{4025321609071.0 / 1340029796352, 4172423538833.0 / 1340029796352},
		{806354699.0 / 268435456, 835776757.0 / 268435456},
		{4025321609071.0 / 1340029796352, 4171973699729.0 / 1340029796352},
	}
	fuse := op == parametric.Union
	index := 0
	if fuse {
		index = 1
	}
	area = polynomialPairSourceEnvelopeArea(fuse, []float64{0, .25, .5, .75, 1}, func(u float64) (f, g, df, dg float64) {
		f = 1.0/4 + u*u*u*u/8
		df = u * u * u / 2
		x, y, z, t := u-.25, u-.5, u-.75, u-float64(tangent)/4
		g = f + x*y*z*t
		dg = df + y*z*t + x*z*t + x*y*t + x*y*z
		return f, g, df, dg
	})
	return area, volumeByOperation[tangent-1][index]
}

func TestNativePolynomialTwoCrossingsTangencyEnvelope(t *testing.T) {
	ctx := context.Background()
	for _, tangent := range []int64{1, 2, 3} {
		for axis := 0; axis < 2; axis++ {
			base, a, b := nativeTwoCrossingsTangencyFixture(t, axis, tangent, 4)
			for order, roofs := range [][2]*geom.BSplineSurface{{a, b}, {b, a}} {
				var patches [2]*geom.SurfacePatch
				for i, roof := range roofs {
					var e error
					patches[i], e = geom.NewSurfacePatch(roof, geom.SurfacePatchOptions{})
					if e != nil {
						t.Fatal(e)
					}
				}
				original, e := geom.DiscoverSplineGraphPolynomialTwoCrossingsTangency(ctx, patches[0], patches[1], axis, geom.GraphPointContactOptions{})
				if e != nil {
					t.Fatal(e)
				}
				for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union} {
					r, e := CombineSplineGraphPolynomialTwoCrossingsTangency(ctx, base, roofs[0], roofs[1], axis, op, 1e-9, PolynomialTwoCrossingsTangencyEnvelopeOptions{})
					if e != nil {
						t.Fatal(tangent, axis, order, op, e)
					}
					var raw bytes.Buffer
					if e = r.WriteJSON(ctx, &raw); e != nil {
						t.Fatal(e)
					}
					loaded, e := ReadPolynomialTwoCrossingsTangencyEnvelopeJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, PolynomialTwoCrossingsTangencyEnvelopeOptions{})
					if e != nil || loaded.History() != r.History() || !reflect.DeepEqual(loaded.Contacts(), r.Contacts()) {
						t.Fatal("source reload changed native owner and junction ledgers", tangent, axis, order, op, e)
					}
					for i, source := range r.sources {
						if !geom.SameSplineSurface(source, loaded.sources[i]) {
							t.Fatal("reloaded native support changed its original coefficients", i)
						}
					}
					checkTwoCrossingsTangencyEnvelope(t, loaded, original, tangent, order, op)
					wantArea, wantVolume := polynomialTwoCrossingsTangencyMeasures(tangent, op)
					got, e := loaded.Boundary().MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
					if e != nil || math.Abs(got.Volume-wantVolume) > 1e-9 || math.Abs(got.Area-wantArea) > 1e-8 {
						t.Fatal("independent original envelope volume and area", tangent, axis, order, op, got.Volume, wantVolume, got.Area, wantArea, e)
					}
					var replay bytes.Buffer
					if e = loaded.WriteJSON(ctx, &replay); e != nil || !bytes.Equal(raw.Bytes(), replay.Bytes()) {
						t.Fatal("canonical three-contact source recipe", e)
					}
					if tangent == 2 && axis == 0 && order == 0 && op == parametric.Intersection {
						checkTwoCrossingsTangencyOrdinaryWriter(t, r)
					}
				}
			}
		}
	}
}

func checkTwoCrossingsTangencyEnvelope(t *testing.T, r *PolynomialTwoCrossingsTangencyEnvelope, proof geom.SplineGraphPolynomialTwoCrossingsTangency, tangent int64, order int, op parametric.BooleanOp) {
	t.Helper()
	s, h := r.Boundary(), r.History()
	top := s.Topology()
	if len(top.Shells) != 1 || top.EulerCharacteristic != 2 || top.CollapsedEdges != 0 || len(s.faces) != 15 || s.faces[h.Base].Surface != r.sources[0] {
		t.Fatal("closed single-shell envelope with all six roof regions", top)
	}
	for i, face := range h.RoofFaces {
		owner := 1
		sign := [6]int{1, -1, -1, -1, -1, 1}[i] * (1 - 2*order)
		if (sign > 0) == (op == parametric.Union) {
			owner = 0
		}
		if h.RoofOwners[i] != owner || s.faces[face].Surface != r.sources[1+owner] || s.faces[face].Reversed {
			t.Fatal("original signed roof selection", i, h)
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
	usedVertices := make(map[int]bool)
	for i, v := range h.Vertices {
		if s.vertices[v] != xyz[i] || usedVertices[v] || i >= 4 && degrees[v] != 4 {
			t.Fatal("distinct exact original vertex identity", i, v)
		}
		usedVertices[v] = true
	}
	for i, pair := range h.Midpoints {
		if len(s.faces[h.RoofFaces[i+2]].Outer) != 4 {
			t.Fatal("separate four-sided native lens", i)
		}
		for _, v := range pair {
			p := s.vertices[v]
			if usedVertices[v] || degrees[v] != 2 || ([2]float64{p.X, p.Y})[axis] != float64(3+2*i)/8 {
				t.Fatal("distinct exact lens-side subdivision vertex", i, v, p)
			}
			usedVertices[v] = true
		}
	}
	usedEdges := make(map[int]bool)
	for i, ids := range h.Edges {
		source := proof.Edges[i]
		stations := []int{h.Vertices[source.Start], h.Vertices[source.End]}
		slab := i % 4
		if slab == 1 || slab == 2 {
			stations = []int{stations[0], h.Midpoints[slab-1][source.Branch], stations[1]}
		}
		if (ids[1] >= 0) != (len(stations) == 3) {
			t.Fatal("original restriction subdivision ledger", i, ids)
		}
		for j, id := range ids[:len(stations)-1] {
			edge := s.edges[id]
			ends := [2]int{edge.Start, edge.End}
			if usedEdges[id] || ends != [2]int{stations[j], stations[j+1]} && ends != [2]int{stations[j+1], stations[j]} {
				t.Fatal("native edge preserves original endpoints", i, id, ends, stations)
			}
			usedEdges[id] = true
			curve := edge.Curve.(surfaceTrimCurve).curve
			checkNativeThreePolynomialBranch(t, curve, axis, proof.Power[source.Branch])
			var owners, directions [2]int
			for k, face := range h.RoofFaces {
				for _, use := range s.faces[face].Outer {
					if use.Edge != id {
						continue
					}
					owners[h.RoofOwners[k]]++
					expected := curve
					if use.Reversed {
						expected = reversePCurve(expected)
						directions[1]++
					} else {
						directions[0]++
					}
					if !matchesTangencyTrim(use.Curve, expected) {
						t.Fatal("both original owner coedges must retain their exact restrictions", i, id, k)
					}
				}
			}
			if owners != [2]int{1, 1} || directions != [2]int{1, 1} {
				t.Fatal("paired exact original coedges", i, id, owners, directions)
			}
		}
	}
	if len(usedEdges) != 12 {
		t.Fatal("complete original native section edge ledger", len(usedEdges))
	}
	contacts := r.Contacts()
	for i, c := range contacts {
		node := proof.Nodes[i]
		expectedOrder := 1
		if int64(i+1) == tangent {
			expectedOrder = 2
		}
		if c.Order != expectedOrder || c.Vertex != h.Vertices[node.Vertex] || c.DerivativeGap.Cmp(proof.DerivativeGap[i]) != 0 ||
			!reflect.DeepEqual(c.Parameters, proof.Vertices[node.Vertex].Parameters) || !reflect.DeepEqual(c.Point, proof.Vertices[node.Vertex].Point) {
			t.Fatal("exact original contact type and vertex", i, c)
		}
		for j, source := range node.RayCycle {
			id := h.Edges[source][0]
			if h.Edges[source][1] >= 0 && proof.Edges[source].End == node.Vertex {
				id = h.Edges[source][1]
			}
			if c.Rays[j] != id || c.Sectors[j] != h.RoofFaces[node.Regions[j]] {
				t.Fatal("native ray and sector cycle matches the original", i, j, c)
			}
			found := 0
			for _, use := range s.faces[c.Sectors[j]].Outer {
				if use.Edge == id || use.Edge == c.Rays[(j+1)%4] {
					found++
				}
			}
			if found != 2 {
				t.Fatal("both native rays must bound their signed roof sector", i, j, c)
			}
		}
	}
}

func checkTwoCrossingsTangencyOrdinaryWriter(t *testing.T, r *PolynomialTwoCrossingsTangencyEnvelope) {
	t.Helper()
	lens := r.solid.faces[r.history.RoofFaces[2]]
	regular, e := ValidateFaceTrimLoops(context.Background(), lens, trimvalidate.Options{})
	if e != nil {
		t.Fatal(e)
	}
	if regular.Status != trimvalidate.Certified {
		var raw bytes.Buffer
		if e := WriteJSON(&raw, r.Boundary()); e == nil || raw.Len() != 0 {
			t.Fatal("ordinary writer ignored its trim gate", e)
		}
	}
}

func TestNativePolynomialTwoCrossingsTangencyEnvelopeRejections(t *testing.T) {
	ctx := context.Background()
	base, a, b := nativeTwoCrossingsTangencyFixture(t, 0, 2, 4)
	r, e := CombineSplineGraphPolynomialTwoCrossingsTangency(ctx, base, a, b, 0, parametric.Intersection, 1e-9, PolynomialTwoCrossingsTangencyEnvelopeOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var raw bytes.Buffer
	if e = r.WriteJSON(ctx, &raw); e != nil {
		t.Fatal(e)
	}
	d := a.Data()
	d.Poles[(d.CountU-1)*d.CountV].Z += .0009765625
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
		"contact order":     func(r *polynomialEnvelopeRecipe) { r.Ledger.Contacts[1].Order = 1 },
		"contact gap":       func(r *polynomialEnvelopeRecipe) { r.Ledger.Contacts[1].DerivativeGap = "0" },
		"last contact":      func(r *polynomialEnvelopeRecipe) { r.Ledger.Contacts[2].Point[0] = "0" },
		"ray":               func(r *polynomialEnvelopeRecipe) { r.Ledger.Contacts[1].Rays[0] = r.Ledger.Contacts[1].Rays[1] },
		"roof owner":        func(r *polynomialEnvelopeRecipe) { r.Ledger.RoofOwners[3] ^= 1 },
		"lost lens side":    func(r *polynomialEnvelopeRecipe) { r.Ledger.Edges[2] = r.Ledger.Edges[2][:1] },
		"recipe kind":       func(r *polynomialEnvelopeRecipe) { r.Kind = nativeFourPolynomialNodeEnvelope },
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
			if r, e := ReadPolynomialTwoCrossingsTangencyEnvelopeJSON(ctx, bytes.NewReader(data), JSONLimits{}, PolynomialTwoCrossingsTangencyEnvelopeOptions{}); r != nil || e == nil {
				t.Fatal("changed original support or native contact identity accepted", e)
			}
		})
	}
	for _, limits := range []JSONLimits{{MaxBytes: 1}, {MaxControlPoints: 4}, {MaxEntities: 3}} {
		if r, e := ReadPolynomialTwoCrossingsTangencyEnvelopeJSON(ctx, bytes.NewReader(raw.Bytes()), limits, PolynomialTwoCrossingsTangencyEnvelopeOptions{}); r != nil || e == nil {
			t.Fatal("recipe reader ignored limits", limits, e)
		}
	}
	lowBudget := PolynomialTwoCrossingsTangencyEnvelopeOptions{Proof: geom.GraphPointContactOptions{MaxOperations: 1}}
	if r, e := ReadPolynomialTwoCrossingsTangencyEnvelopeJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, lowBudget); r != nil || !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("reproof ignored its work limit", e)
	}
	if r, e := CombineSplineGraphPolynomialTwoCrossingsTangency(ctx, base, a, b, 0, parametric.Difference, 1e-9, PolynomialTwoCrossingsTangencyEnvelopeOptions{}); r != nil || !errors.Is(e, geom.ErrInvalid) {
		t.Fatal("Common/Fuse recipe admitted point-contacting Cut", e)
	}
	base, a, b = nativeTwoCrossingsTangencyFixture(t, 0, 2, 5)
	if r, e := CombineSplineGraphPolynomialTwoCrossingsTangency(ctx, base, a, b, 0, parametric.Union, 1e-9, PolynomialTwoCrossingsTangencyEnvelopeOptions{}); r != nil || !errors.Is(e, geom.ErrUnrepresentable) {
		t.Fatal("native proof rounded a nonbinary crossing", e)
	}
}
