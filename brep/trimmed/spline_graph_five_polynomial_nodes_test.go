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

func nativeFivePolynomialNodeFixture(t *testing.T, axis int, firstDenominator int64) (*geom.BSplineSurface, *geom.BSplineSurface, *geom.BSplineSurface) {
	t.Helper()
	f := []*big.Rat{big.NewRat(1, 4), big.NewRat(3, 8), big.NewRat(-3, 8), big.NewRat(1, 8)}
	// For denominator eight the original gap is the negative product of
	// five monic linear factors. The roof product f*g has degree eight.
	factors := [5][2]int64{{1, firstDenominator}, {1, 4}, {1, 2}, {3, 4}, {7, 8}}
	g := []*big.Rat{big.NewRat(-1, 2048)}
	for _, root := range factors {
		next := make([]*big.Rat, len(g)+1)
		for i := range next {
			next[i] = new(big.Rat)
		}
		for i, c := range g {
			next[i].Sub(next[i], new(big.Rat).Mul(c, big.NewRat(root[0], 1)))
			next[i+1].Add(next[i+1], new(big.Rat).Mul(c, big.NewRat(root[1], 1)))
		}
		g = next
	}
	for i, c := range f {
		g[i].Add(g[i], c)
	}
	return twoPolynomialRoofFixture(t, axis, f, g)
}

func fivePolynomialNodeMeasures(t *testing.T, op parametric.BooleanOp) (float64, float64) {
	t.Helper()
	sum := func(order int) *big.Rat {
		total := new(big.Rat)
		for _, v := range fivePolynomialNodeCutVolumes(t, order) {
			total.Add(total, v)
		}
		return total
	}
	fuse := op == parametric.Union
	volume := big.NewRat(769, 256) // exact volume of B=3+u*v/64
	if fuse {
		volume.Add(volume, sum(0))
		if volume.Cmp(big.NewRat(2674732429150191067, 864691128455135232)) != 0 {
			t.Fatal("exact positive exterior material in Fuse", volume)
		}
	} else {
		volume.Sub(volume, sum(1))
		if volume.Cmp(big.NewRat(865817026814476471, 288230376151711744)) != 0 {
			t.Fatal("exact omitted interior material in Common", volume)
		}
	}
	area := polynomialPairSourceEnvelopeArea(fuse, []float64{0, .125, .25, .5, .75, .875, 1}, func(u float64) (f, g, df, dg float64) {
		f = ((u/8-3.0/8)*u+3.0/8)*u + .25
		df = (3*u/8-.75)*u + .375
		a, b, c, d, e := u-.125, u-.25, u-.5, u-.75, u-.875
		product := a * b * c * d
		productDerivative := (a+b)*c*d + a*b*(c+d)
		g = f - e*product
		dg = df - product - e*productDerivative
		return f, g, df, dg
	})
	v, _ := volume.Float64()
	return area, v
}

func TestNativeFivePolynomialNodeEnvelope(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	for axis := 0; axis < 2; axis++ {
		base, a, b := nativeFivePolynomialNodeFixture(t, axis, 8)
		for order, roofs := range [][2]*geom.BSplineSurface{{a, b}, {b, a}} {
			var patches [2]*geom.SurfacePatch
			for i, roof := range roofs {
				var e error
				patches[i], e = geom.NewSurfacePatch(roof, geom.SurfacePatchOptions{})
				if e != nil {
					t.Fatal(e)
				}
			}
			original, e := geom.DiscoverSplineGraphPolynomialArrangement(ctx, patches[0], patches[1], axis, geom.GraphPointContactOptions{})
			if e != nil {
				t.Fatal(e)
			}
			for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union} {
				r, e := CombineSplineGraphFivePolynomialNodes(ctx, base, roofs[0], roofs[1], axis, op, 1e-9, FivePolynomialNodeEnvelopeOptions{})
				if e != nil {
					t.Fatal(axis, order, op, e)
				}
				var raw bytes.Buffer
				if e = r.WriteJSON(ctx, &raw); e != nil {
					t.Fatal(e)
				}
				loaded, e := ReadFivePolynomialNodeEnvelopeJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, FivePolynomialNodeEnvelopeOptions{})
				if e != nil || loaded.History() != r.History() || !reflect.DeepEqual(loaded.Contacts(), r.Contacts()) {
					t.Fatal("source reload changed native owner and junction ledgers", axis, order, op, e)
				}
				for i, source := range r.sources {
					if !geom.SameSplineSurface(source, loaded.sources[i]) {
						t.Fatal("reloaded native support changed its original coefficients", i)
					}
				}
				checkFivePolynomialNodeEnvelope(t, loaded, original, order, op)
				wantArea, wantVolume := fivePolynomialNodeMeasures(t, op)
				got, e := loaded.Boundary().MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
				if e != nil || math.Abs(got.Volume-wantVolume) > 1e-9 || math.Abs(got.Area-wantArea) > 1e-8 {
					t.Fatal("independent original envelope volume and area", axis, order, op, got.Volume, wantVolume, got.Area, wantArea, e)
				}
				var replay bytes.Buffer
				if e = loaded.WriteJSON(ctx, &replay); e != nil || !bytes.Equal(raw.Bytes(), replay.Bytes()) {
					t.Fatal("canonical five-contact source recipe", e)
				}
				if axis == 0 && order == 0 && op == parametric.Intersection {
					var ordinary bytes.Buffer
					if e = WriteJSON(&ordinary, r.Boundary()); !errors.Is(e, geom.ErrInvalid) || ordinary.Len() != 0 {
						t.Fatal("ordinary native codec admitted source-only quintic pcurves", e)
					}
				}
			}
		}
	}
}

func checkFivePolynomialNodeEnvelope(t *testing.T, r *FivePolynomialNodeEnvelope, proof geom.SplineGraphPolynomialArrangement, order int, op parametric.BooleanOp) {
	t.Helper()
	s, h := r.Boundary(), r.History()
	top := s.Topology()
	if len(top.Shells) != 1 || top.EulerCharacteristic != 2 || top.CollapsedEdges != 0 || len(s.faces) != 17 || s.faces[h.Base].Surface != r.sources[0] {
		t.Fatal("closed single-shell envelope with all eight roof regions", top)
	}
	for i, face := range h.RoofFaces {
		owner := 1
		sign := [8]int{1, -1, -1, -1, -1, -1, -1, 1}[i] * (1 - 2*order)
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
			if usedVertices[v] || degrees[v] != 2 || ([2]float64{p.X, p.Y})[axis] != [4]float64{3.0 / 16, 3.0 / 8, 5.0 / 8, 13.0 / 16}[i] {
				t.Fatal("distinct exact lens-side subdivision vertex", i, v, p)
			}
			usedVertices[v] = true
		}
	}
	usedEdges := make(map[int]bool)
	for i, ids := range h.Edges {
		source := proof.Edges[i]
		stations := []int{h.Vertices[source.Start], h.Vertices[source.End]}
		slab := i % 6
		if slab >= 1 && slab <= 4 {
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
	if len(usedEdges) != 20 {
		t.Fatal("complete original native section edge ledger", len(usedEdges))
	}
	contacts := r.Contacts()
	for i, c := range contacts {
		node := proof.Nodes[i]
		if c.Order != 1 || c.Vertex != h.Vertices[node.Vertex] || c.DerivativeGap.Cmp(proof.DerivativeGap[i]) != 0 ||
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
	gap := new(big.Rat).Set(contacts[0].DerivativeGap)
	contacts[0].DerivativeGap.SetInt64(0)
	if r.Contacts()[0].DerivativeGap.Cmp(gap) != 0 {
		t.Fatal("returned derivative gap aliases the native certificate")
	}
}

func TestNativeFivePolynomialNodeEnvelopeRejections(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	base, a, b := nativeFivePolynomialNodeFixture(t, 0, 8)
	r, e := CombineSplineGraphFivePolynomialNodes(ctx, base, a, b, 0, parametric.Intersection, 1e-9, FivePolynomialNodeEnvelopeOptions{})
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
		"contact order":     func(r *polynomialEnvelopeRecipe) { r.Ledger.Contacts[1].Order = 2 },
		"contact gap":       func(r *polynomialEnvelopeRecipe) { r.Ledger.Contacts[1].DerivativeGap = "0" },
		"last contact":      func(r *polynomialEnvelopeRecipe) { r.Ledger.Contacts[4].Point[0] = "0" },
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
			if r, e := ReadFivePolynomialNodeEnvelopeJSON(ctx, bytes.NewReader(data), JSONLimits{}, FivePolynomialNodeEnvelopeOptions{}); r != nil || e == nil {
				t.Fatal("changed original support or native contact identity accepted", e)
			}
		})
	}
	for _, limits := range []JSONLimits{{MaxBytes: 1}, {MaxControlPoints: 4}, {MaxEntities: 3}} {
		if r, e := ReadFivePolynomialNodeEnvelopeJSON(ctx, bytes.NewReader(raw.Bytes()), limits, FivePolynomialNodeEnvelopeOptions{}); r != nil || e == nil {
			t.Fatal("recipe reader ignored limits", limits, e)
		}
	}
	lowBudget := FivePolynomialNodeEnvelopeOptions{Proof: geom.GraphPointContactOptions{MaxOperations: 1}}
	if r, e := ReadFivePolynomialNodeEnvelopeJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, lowBudget); r != nil || !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("reproof ignored its work limit", e)
	}
	if r, e := CombineSplineGraphFivePolynomialNodes(ctx, base, a, b, 0, parametric.Difference, 1e-9, FivePolynomialNodeEnvelopeOptions{}); r != nil || !errors.Is(e, geom.ErrInvalid) {
		t.Fatal("Common/Fuse recipe admitted point-contacting Cut", e)
	}
	base, a, b = nativeFivePolynomialNodeFixture(t, 0, 7)
	if r, e := CombineSplineGraphFivePolynomialNodes(ctx, base, a, b, 0, parametric.Union, 1e-9, FivePolynomialNodeEnvelopeOptions{}); r != nil || !errors.Is(e, geom.ErrUnrepresentable) {
		t.Fatal("native proof rounded a nonbinary crossing", e)
	}
}

func TestNativeFivePolynomialNodeEnvelopeRejectsOtherArrangements(t *testing.T) {
	ctx := context.Background()
	for _, fixture := range []func() (*geom.BSplineSurface, *geom.BSplineSurface, *geom.BSplineSurface){
		func() (*geom.BSplineSurface, *geom.BSplineSurface, *geom.BSplineSurface) {
			return nativeFourPolynomialNodeFixture(t, 0, 8)
		},
		func() (*geom.BSplineSurface, *geom.BSplineSurface, *geom.BSplineSurface) {
			return nativeThreeCrossingsTangencyFixture(t, 0, 1, 8)
		},
	} {
		base, a, b := fixture()
		pa, e := geom.NewSurfacePatch(a, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		pb, e := geom.NewSurfacePatch(b, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		proof, e := geom.DiscoverSplineGraphPolynomialArrangement(ctx, pa, pb, 0, geom.GraphPointContactOptions{})
		if e != nil || !proof.Proven {
			t.Fatal("other arrangement is independently proved in the generic geometry", e)
		}
		out, e := CombineSplineGraphFivePolynomialNodes(ctx, base, a, b, 0, parametric.Intersection, 1e-9, FivePolynomialNodeEnvelopeOptions{})
		if out != nil || !errors.Is(e, geom.ErrConvergence) {
			t.Fatal("bounded native scope cannot admit a different pattern", e)
		}
	}
}
