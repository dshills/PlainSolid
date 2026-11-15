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

func nativeFourPolynomialNodeFixture(t *testing.T, axis int, firstDenominator int64) (*geom.BSplineSurface, *geom.BSplineSurface, *geom.BSplineSurface) {
	t.Helper()
	return nativeQuarticRoofPairFixture(t, axis, big.NewRat(1, 4096), [4][2]int64{{1, firstDenominator}, {3, 8}, {5, 8}, {7, 8}})
}

func nativeQuarticRoofPairFixture(t *testing.T, axis int, scale *big.Rat, factors [4][2]int64) (*geom.BSplineSurface, *geom.BSplineSurface, *geom.BSplineSurface) {
	t.Helper()
	f := []*big.Rat{big.NewRat(1, 4), new(big.Rat), new(big.Rat), new(big.Rat), big.NewRat(1, 8)}
	g := []*big.Rat{new(big.Rat).Set(scale)}
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

func fourPolynomialNodeMeasures(op parametric.BooleanOp) (area, volume float64) {
	// The between-branch height integral is -105|g-f|³/768.
	// Adding its integral on the five slabs to the B stock gives
	// Common. Fuse adds the absolute negative contribution to the
	// integral of the original A roof over the full square.
	fuse := op == parametric.Union
	if fuse {
		return fourPolynomialNodeSourceArea(fuse), 54790073302219.0 / 17592186044416
	}
	return fourPolynomialNodeSourceArea(fuse), 52845275824949.0 / 17592186044416
}

func fourPolynomialNodeSourceArea(fuse bool) float64 {
	return polynomialPairSourceEnvelopeArea(fuse, []float64{0, .125, .375, .625, .875, 1}, func(u float64) (f, g, df, dg float64) {
		f = 1.0/4 + u*u*u*u/8
		g = f + (u-.125)*(u-.375)*(u-.625)*(u-.875)
		df = u * u * u / 2
		dg = df + ((4*u-6)*u+43.0/16)*u - 11.0/32
		return f, g, df, dg
	})
}

func polynomialPairSourceEnvelopeArea(fuse bool, stations []float64, branches func(float64) (f, g, df, dg float64)) float64 {
	return polynomialPairSourceEnvelopeAreaScaled(fuse, stations, branches, 1, 1, [3]float64{})
}

func polynomialPairSourceEnvelopeAreaScaled(fuse bool, stations []float64, branches func(float64) (f, g, df, dg float64),
	freeLength, dependentLength float64, base [3]float64) float64 {
	// Independent Gauss-Legendre integration directly on the original
	// stock equations. Each quadrature rectangle maps to one signed
	// strip over a slab and evaluates the original roof derivatives.
	x := [8]float64{.09501250983763744, .2816035507792589, .4580167776572274, .6178762444026438, .7554044083550030, .8656312023878318, .9445750230732326, .9894009349916499}
	w := [8]float64{.1894506104550685, .1826034150449236, .1691565193950025, .1495959888165767, .1246289712555339, .09515851168249278, .06225352393864789, .027152459411754095}
	integrate := func(a, b float64, value func(float64) float64) float64 {
		half, center, sum := (b-a)/2, (a+b)/2, 0.0
		for i, t := range x {
			delta := half * t
			sum += w[i] * (value(center-delta) + value(center+delta))
		}
		return sum * half
	}
	roofHeight := func(u, v float64, raised bool) float64 {
		base := 3 + u*v/64
		if raised {
			f, g, _, _ := branches(u)
			base += (105.0 / 128) * (v - f) * (v - g)
		}
		return base
	}
	roofJacobian := func(u, v float64, raised bool) float64 {
		du, dv := v/64, u/64
		if raised {
			f, g, df, dg := branches(u)
			du -= (105.0 / 128) * (df*(v-g) + dg*(v-f))
			dv += (105.0 / 128) * (2*v - f - g)
		}
		du, dv = du*dependentLength, dv*freeLength
		xy := freeLength * dependentLength
		return math.Sqrt(xy*xy + du*du + dv*dv)
	}
	fiber := func(u float64, value func(float64, float64, bool) float64) float64 {
		f, g, _, _ := branches(u)
		lo, hi := min(f, g), max(f, g)
		exterior := func(v float64) float64 { return value(u, v, fuse) }
		interior := func(v float64) float64 { return value(u, v, !fuse) }
		return integrate(0, lo, exterior) + integrate(lo, hi, interior) + integrate(hi, 1, exterior)
	}
	xy := freeLength * dependentLength
	baseU, baseV := base[1]*dependentLength, base[2]*freeLength
	area := math.Sqrt(xy*xy + baseU*baseU + baseV*baseV)
	for i := range stations[:len(stations)-1] {
		area += integrate(stations[i], stations[i+1], func(u float64) float64 { return fiber(u, roofJacobian) })
	}
	// Each wall is a vertical fiber over an original domain side.
	for _, edge := range []float64{0, 1} {
		thickness := func(u, v float64, raised bool) float64 {
			return roofHeight(u, v, raised) - (base[0] + base[1]*u + base[2]*v)
		}
		area += freeLength * integrate(0, 1, func(u float64) float64 { return thickness(u, edge, fuse) })
		area += dependentLength * fiber(edge, thickness)
	}
	return area
}

func TestNativeFourPolynomialNodeEnvelope(t *testing.T) {
	ctx := context.Background()
	for axis := 0; axis < 2; axis++ {
		base, a, b := nativeFourPolynomialNodeFixture(t, axis, 8)
		for order, roofs := range [][2]*geom.BSplineSurface{{a, b}, {b, a}} {
			var patches [2]*geom.SurfacePatch
			for i, roof := range roofs {
				var e error
				patches[i], e = geom.NewSurfacePatch(roof, geom.SurfacePatchOptions{})
				if e != nil {
					t.Fatal(e)
				}
			}
			proof, e := geom.DiscoverSplineGraphFourPolynomialNodes(ctx, patches[0], patches[1], axis, geom.GraphPointContactOptions{})
			if e != nil {
				t.Fatal(e)
			}
			for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union} {
				r, e := CombineSplineGraphFourPolynomialNodes(ctx, base, roofs[0], roofs[1], axis, op, 1e-9, FourPolynomialNodeEnvelopeOptions{})
				if e != nil {
					t.Fatal(axis, order, op, e)
				}
				var raw bytes.Buffer
				if e = r.WriteJSON(ctx, &raw); e != nil {
					t.Fatal(e)
				}
				loaded, e := ReadFourPolynomialNodeEnvelopeJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, FourPolynomialNodeEnvelopeOptions{})
				if e != nil || loaded.History() != r.History() || !reflect.DeepEqual(loaded.Contacts(), r.Contacts()) {
					t.Fatal("reproved native ledger changed on reload", axis, order, op, e)
				}
				for i, source := range r.sources {
					if !geom.SameSplineSurface(source, loaded.sources[i]) {
						t.Fatal("native changed an original support", i)
					}
				}
				checkNativeFourPolynomialNodeEnvelope(t, loaded, proof, order, op)
				m, e := loaded.Boundary().MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
				area, volume := fourPolynomialNodeMeasures(op)
				if e != nil || math.Abs(m.Volume-volume) > 1e-9 || math.Abs(m.Area-area) > 1e-8 {
					t.Fatal("independent four-crossing area and volume", axis, order, op, m.Area, area, m.Volume, volume, e)
				}
				var rewritten bytes.Buffer
				if e = loaded.WriteJSON(ctx, &rewritten); e != nil || !bytes.Equal(raw.Bytes(), rewritten.Bytes()) {
					t.Fatal("canonical original four-node recipe", e)
				}
			}
		}
	}
}

func checkNativeFourPolynomialNodeEnvelope(t *testing.T, r *FourPolynomialNodeEnvelope, proof geom.SplineGraphFourPolynomialNodes, order int, op parametric.BooleanOp) {
	t.Helper()
	s, h := r.Boundary(), r.History()
	topology := s.Topology()
	if len(topology.Shells) != 1 || topology.EulerCharacteristic != 2 || topology.CollapsedEdges != 0 || len(s.faces) != 16 ||
		s.faces[h.Base].Surface != r.sources[0] {
		t.Fatal("one native envelope shell containing all seven roof regions", topology)
	}
	_, xyz, e := proof.RepresentedVertices()
	if e != nil {
		t.Fatal(e)
	}
	vertices := make(map[int]bool)
	degree := make([]int, len(s.vertices))
	for _, edge := range s.edges {
		degree[edge.Start]++
		degree[edge.End]++
	}
	for i, id := range h.Vertices {
		if vertices[id] || s.vertices[id] != xyz[i] || i >= 4 && degree[id] != 4 {
			t.Fatal("original exact vertex identity", i, id)
		}
		vertices[id] = true
	}
	for i, face := range h.RoofFaces {
		sign := [7]int{1, -1, -1, -1, -1, -1, 1}[i] * (1 - 2*order)
		want := 1
		if (sign > 0) == (op == parametric.Union) {
			want = 0
		}
		if h.RoofOwners[i] != want || s.faces[face].Surface != r.sources[1+want] || s.faces[face].Reversed {
			t.Fatal("signed original selected owner", i, h)
		}
	}
	axis := proof.FreeAxis
	for lens, pair := range h.Midpoints {
		if len(s.faces[h.RoofFaces[lens+2]].Outer) != 4 {
			t.Fatal("an interior lens must retain four native sides", lens)
		}
		for _, id := range pair {
			p := s.vertices[id]
			if vertices[id] || degree[id] != 2 || ([2]float64{p.X, p.Y})[axis] != float64(lens+1)/4 {
				t.Fatal("original lens branch midpoint and station", lens, id, p)
			}
			vertices[id] = true
		}
	}

	seen := make(map[int]bool)
	for i, ids := range h.Edges {
		source := proof.Edges[i]
		local := i % 5
		stations := []int{h.Vertices[source.Start], h.Vertices[source.End]}
		if local >= 1 && local <= 3 {
			stations = []int{stations[0], h.Midpoints[local-1][source.Branch], stations[1]}
		}
		if (ids[1] >= 0) != (len(stations) == 3) {
			t.Fatal("original branch subdivisions", i, ids)
		}
		for j, id := range ids[:len(stations)-1] {
			edge := s.edges[id]
			ends := [2]int{edge.Start, edge.End}
			if seen[id] || ends != [2]int{stations[j], stations[j+1]} && ends != [2]int{stations[j+1], stations[j]} {
				t.Fatal("source branch and distinct exact edge identity", i, j, id, ends)
			}
			seen[id] = true
			curve := edge.Curve.(surfaceTrimCurve).curve
			checkNativeThreePolynomialBranch(t, curve, axis, proof.Power[source.Branch])
			var owners, directions [2]int
			for k, fi := range h.RoofFaces {
				for _, use := range s.faces[fi].Outer {
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
						t.Fatal("both exact owner restrictions must survive reload", i, j, k)
					}
				}
			}
			if owners != [2]int{1, 1} || directions != [2]int{1, 1} {
				t.Fatal("paired original coedge ownership", i, j, owners, directions)
			}
		}
	}
	if len(seen) != 16 {
		t.Fatal("all original source subdivisions", len(seen))
	}

	contacts := r.Contacts()
	for i, c := range contacts {
		node := proof.Nodes[i]
		point := proof.Vertices[node.Vertex]
		gap := new(big.Rat).Sub(node.Tangents[1][1-axis], node.Tangents[0][1-axis])
		if c.Order != 1 || c.DerivativeGap.Cmp(gap) != 0 || c.Vertex != h.Vertices[node.Vertex] ||
			!reflect.DeepEqual(c.Parameters, point.Parameters) || !reflect.DeepEqual(c.Point, point.Point) {
			t.Fatal("exact original transverse junction certificate", i, c)
		}
		for j, ei := range node.RayCycle {
			id := h.Edges[ei][0]
			if proof.Edges[ei].End == node.Vertex && h.Edges[ei][1] >= 0 {
				id = h.Edges[ei][1]
			}
			if c.Rays[j] != id || c.Sectors[j] != h.RoofFaces[node.Regions[j]] {
				t.Fatal("original model ordered rays and their native sectors", i, j, c)
			}
			found := 0
			for _, use := range s.faces[c.Sectors[j]].Outer {
				if use.Edge == id || use.Edge == c.Rays[(j+1)%4] {
					found++
				}
			}
			if found != 2 {
				t.Fatal("native sector must meet both original incident rays", i, j, c)
			}
		}
	}
}

func TestNativeFourPolynomialNodeEnvelopeRejections(t *testing.T) {
	ctx := context.Background()
	base, a, b := nativeFourPolynomialNodeFixture(t, 0, 8)
	r, e := CombineSplineGraphFourPolynomialNodes(ctx, base, a, b, 0, parametric.Union, 1e-9, FourPolynomialNodeEnvelopeOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var raw bytes.Buffer
	if e = r.WriteJSON(ctx, &raw); e != nil {
		t.Fatal(e)
	}
	data := a.Data()
	data.Poles[(data.CountU-1)*data.CountV].Z += 1.0 / 1024
	changed, e := geom.NewBSplineSurface(data.DegreeU, data.DegreeV, data.CountU, data.CountV, data.Poles, data.KnotsU, data.KnotsV, data.Weights)
	if e != nil {
		t.Fatal(e)
	}
	altered, e := parametric.EncodeSurface(changed)
	if e != nil {
		t.Fatal(e)
	}
	for name, mutation := range map[string]func(*polynomialEnvelopeRecipe){
		"later original cell": func(r *polynomialEnvelopeRecipe) { r.Sources[1] = altered },
		"fourth point":        func(r *polynomialEnvelopeRecipe) { r.Ledger.Contacts[3].Point[0] = "0" },
		"missing node":        func(r *polynomialEnvelopeRecipe) { r.Ledger.Contacts = r.Ledger.Contacts[:3] },
		"wrong ray":           func(r *polynomialEnvelopeRecipe) { r.Ledger.Contacts[2].Rays[0] = r.Ledger.Contacts[2].Rays[1] },
		"wrong roof":          func(r *polynomialEnvelopeRecipe) { r.Ledger.RoofOwners[4] ^= 1 },
		"lost subdivision":    func(r *polynomialEnvelopeRecipe) { r.Ledger.Edges[3] = r.Ledger.Edges[3][:1] },
		"wrong kind":          func(r *polynomialEnvelopeRecipe) { r.Kind = nativePolynomialMixedEnvelope },
	} {
		t.Run(name, func(t *testing.T) {
			var recipe polynomialEnvelopeRecipe
			if e := json.Unmarshal(raw.Bytes(), &recipe); e != nil {
				t.Fatal(e)
			}
			mutation(&recipe)
			encoded, e := json.Marshal(recipe)
			if e != nil {
				t.Fatal(e)
			}
			if out, e := ReadFourPolynomialNodeEnvelopeJSON(ctx, bytes.NewReader(encoded), JSONLimits{}, FourPolynomialNodeEnvelopeOptions{}); out != nil || e == nil {
				t.Fatal("changed four-crossing geometry or ledger accepted", e)
			}
		})
	}
	for _, limits := range []JSONLimits{{MaxBytes: 1}, {MaxControlPoints: 4}, {MaxEntities: 3}} {
		if out, e := ReadFourPolynomialNodeEnvelopeJSON(ctx, bytes.NewReader(raw.Bytes()), limits, FourPolynomialNodeEnvelopeOptions{}); out != nil || e == nil {
			t.Fatal("recipe limits ignored", limits, e)
		}
	}
	lowBudget := FourPolynomialNodeEnvelopeOptions{Proof: geom.GraphPointContactOptions{MaxOperations: 1}}
	if out, e := ReadFourPolynomialNodeEnvelopeJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, lowBudget); out != nil || !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("reproof ignored its limits", e)
	}
	if out, e := CombineSplineGraphFourPolynomialNodes(ctx, base, a, b, 0, parametric.Difference, 1e-9, FourPolynomialNodeEnvelopeOptions{}); out != nil || !errors.Is(e, geom.ErrInvalid) {
		t.Fatal("envelope API admitted point-contacting Cut", e)
	}
	base, a, b = nativeFourPolynomialNodeFixture(t, 0, 6)
	if out, e := CombineSplineGraphFourPolynomialNodes(ctx, base, a, b, 0, parametric.Intersection, 1e-9, FourPolynomialNodeEnvelopeOptions{}); out != nil || !errors.Is(e, geom.ErrUnrepresentable) {
		t.Fatal("rounded original rational crossing accepted", e)
	}
}
