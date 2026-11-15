package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"slices"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func threePolynomialNodeCutVolumes(order int) []float64 {
	// Integrate each original fiber of (105/128)(v-f)(v-g). For the
	// exterior, l²h/2-l³/6 is its boundary-to-branch integral before
	// scaling, where l and h are the distances to the near and far
	// branch. For the between-branch cuts the integral is |g-f|³/6.
	// Integrate those explicit polynomials on [0,1/4], [1/4,1/2],
	// [1/2,3/4], and [3/4,1], switching near and far at every node.
	if order == 0 {
		return []float64{3548086484039.0 / 314460325543936, 24570048473159.0 / 314460325543936}
	}
	const denominator = 1886761953263616.0
	return []float64{147258197 / denominator, 243157 / denominator, 423509 / denominator, 1004254165 / denominator}
}

func TestNativeThreePolynomialNodeCutComponents(t *testing.T) {
	ctx := context.Background()
	for axis := 0; axis < 2; axis++ {
		base, a, b := nativeThreePolynomialNodeFixture(t, axis, 4, 128)
		for order, roofs := range [][2]*geom.BSplineSurface{{a, b}, {b, a}} {
			cut, e := CutSplineGraphThreePolynomialNodeComponents(ctx, base, roofs[0], roofs[1], axis, 1e-9, PolynomialNodeCutOptions{})
			if e != nil {
				t.Fatal(axis, order, e)
			}
			var encoded bytes.Buffer
			if e = cut.WriteJSON(ctx, &encoded); e != nil {
				t.Fatal(e)
			}
			loaded, e := ReadThreePolynomialNodeCutJSON(ctx, bytes.NewReader(encoded.Bytes()), JSONLimits{}, PolynomialNodeCutOptions{})
			if e != nil {
				t.Fatal(e)
			}
			checkThreePolynomialNodeCutGraph(t, loaded, axis, order)
			want := threePolynomialNodeCutVolumes(order)
			for i, part := range loaded.Components() {
				original := cut.Components()[i]
				for j := range roofs {
					restored, ok := part.faces[j].Surface.(*geom.BSplineSurface)
					if original.faces[j].Surface != roofs[j] || !ok || !geom.SameSplineSurface(restored, roofs[j]) {
						t.Fatal("original Cut roof ownership", axis, order, i, j)
					}
				}
				mass, e := part.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
				if e != nil || mass.Volume <= 0 || math.Abs(mass.Volume-want[i]) > math.Max(2e-14, 1e-8*want[i]) {
					t.Fatal("independent exact slab volume", axis, order, i, mass.Volume, want[i], e)
				}
			}
			var rewritten bytes.Buffer
			if e = loaded.WriteJSON(ctx, &rewritten); e != nil || !bytes.Equal(rewritten.Bytes(), encoded.Bytes()) {
				t.Fatal("canonical original three-contact recipe", e)
			}
			if axis == 0 && order == 0 {
				var points [3]geom.Vec3
				for i, contact := range loaded.Contacts() {
					points[i], e = geom.RepresentedExactPoint(contact.Point)
					if e != nil {
						t.Fatal(e)
					}
				}
				checkPolynomialNodeCutStrictCollision(t, [2]Solid(loaded.Components()), points[:]...)
			}
		}
	}
}

func checkThreePolynomialNodeCutGraph(t *testing.T, cut *ThreePolynomialNodeCut, axis, order int) {
	t.Helper()
	regions := []int{0, 5}
	if order == 1 {
		regions = []int{1, 2, 3, 4}
	}
	parts := cut.Components()
	if !slices.Equal(cut.Regions(), regions) || len(parts) != len(regions) {
		t.Fatal("retained original positive regions", cut.Regions(), len(parts))
	}
	vertices := make([]map[int]bool, len(parts))
	for i, part := range parts {
		vertices[i] = make(map[int]bool)
		topology := part.Topology()
		if len(topology.Shells) != 1 || topology.EulerCharacteristic != 2 || topology.CollapsedEdges != 0 || part.faces[0].Reversed || !part.faces[1].Reversed {
			t.Fatal("embedded and outward Cut layer", i, topology)
		}
		if regions[i] == 2 || regions[i] == 3 {
			checkThreePolynomialNodeCutLens(t, part, axis, regions[i])
		}
	}
	contacts := cut.Contacts()
	for i, c := range contacts {
		wantComponents := [2]int{0, 1}
		if order == 1 {
			wantComponents = [2]int{i, i + 1}
		}
		wantPoint := threePolynomialNodeCutPoint(axis, i)
		if c.Components != wantComponents || c.Parameters[0].Cmp(wantPoint[0]) != 0 || c.Parameters[1].Cmp(wantPoint[1]) != 0 {
			t.Fatal("exact component incidence and original UV junction", i, c)
		}
		for j, want := range wantPoint {
			if c.Point[j].Cmp(want) != 0 {
				t.Fatal("exact original XYZ junction", i, c, wantPoint)
			}
		}
		p, e := geom.RepresentedExactPoint(wantPoint)
		if e != nil {
			t.Fatal(e)
		}
		for j, component := range c.Components {
			part, v := parts[component], c.Vertices[j]
			if vertices[component][v] || part.vertices[v] != p {
				t.Fatal("distinct exact local vertex identity", i, component, v)
			}
			vertices[component][v] = true
			degree := 0
			for _, edge := range part.edges {
				if edge.Start != v && edge.End != v {
					continue
				}
				degree++
				curve, ok := edge.Curve.(surfaceTrimCurve)
				if !ok {
					t.Fatal("junction incident to a non-source edge", i, component, v)
				}
				if _, ok := curve.curve.(ExactBezierUV); !ok {
					t.Fatal("junction incident to an altered source branch", i, component, v)
				}
			}
			if degree != 2 {
				t.Fatal("two original branch incidences per component vertex", i, component, v, degree)
			}
		}
	}
	contacts[2].Point[0].SetInt64(999)
	if cut.Contacts()[2].Point[0].Cmp(contacts[2].Point[0]) == 0 {
		t.Fatal("caller changed the stored exact proof")
	}
}

func threePolynomialNodeCutPoint(axis, node int) [3]*big.Rat {
	u := big.NewRat(int64(node+1), 4)
	// At all three intersections the original cubic determines v.
	v := new(big.Rat)
	for _, coefficient := range []int64{1, -3, 3, 2} {
		v.Mul(v, u).Add(v, big.NewRat(coefficient, 8))
	}
	z := new(big.Rat).Mul(u, v)
	z.Quo(z, big.NewRat(64, 1)).Add(z, big.NewRat(3, 1))
	if axis == 1 {
		u, v = v, u
	}
	return [3]*big.Rat{u, v, z}
}

func checkThreePolynomialNodeCutLens(t *testing.T, part Solid, axis, region int) {
	t.Helper()
	if len(part.faces) != 2 || len(part.faces[0].Outer) != 4 || len(part.faces[1].Outer) != 4 || len(part.edges) != 4 {
		t.Fatal("each interior lens has two original caps and four distinct edges", part.Topology())
	}
	station := float64(2*region-1) / 8
	midpoints := 0
	for _, p := range part.vertices {
		if ([2]float64{p.X, p.Y})[axis] == station {
			midpoints++
		}
	}
	if midpoints != 2 {
		t.Fatal("both exact midpoint identities must survive on the lens", region, midpoints)
	}
	var degrees [5]int
	for i, top := range part.faces[0].Outer {
		bottom := part.faces[1].Outer[i]
		upperTrim, ok := top.Curve.(ExactBezierUV)
		if !ok || top.Edge != bottom.Edge || !matchesTangencyTrim(bottom.Curve, upperTrim) {
			t.Fatal("both original roof coedges must share the exact branch restriction", region, i)
		}
		degrees[len(upperTrim.control)-1]++
	}
	if degrees[3] != 2 || degrees[4] != 2 {
		t.Fatal("lens must retain both cubic and quartic original branches", region, degrees)
	}
}

func TestNativeThreePolynomialNodeCutRecipeRejections(t *testing.T) {
	ctx := context.Background()
	base, a, b := nativeThreePolynomialNodeFixture(t, 0, 4, 128)
	cut, e := CutSplineGraphThreePolynomialNodeComponents(ctx, base, b, a, 0, 1e-9, PolynomialNodeCutOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var encoded bytes.Buffer
	if e = cut.WriteJSON(ctx, &encoded); e != nil {
		t.Fatal(e)
	}
	for name, mutate := range map[string]func(*polynomialNodeComponentsRecipe){
		"third point":     func(r *polynomialNodeComponentsRecipe) { r.Contacts[2].Point[0] = "1/2" },
		"omitted contact": func(r *polynomialNodeComponentsRecipe) { r.Contacts = r.Contacts[:2] },
		"wrong pair":      func(r *polynomialNodeComponentsRecipe) { r.Contacts[2].Components[0] = 0 },
		"wrong vertex":    func(r *polynomialNodeComponentsRecipe) { r.Contacts[1].Vertices[1]++ },
		"wrong sector":    func(r *polynomialNodeComponentsRecipe) { r.Regions[1] = 3 },
		"exchanged roofs": func(r *polynomialNodeComponentsRecipe) { r.Sources[1], r.Sources[2] = r.Sources[2], r.Sources[1] },
		"two-node kind":   func(r *polynomialNodeComponentsRecipe) { r.Kind = nativeTwoPolynomialNodeCut },
	} {
		t.Run(name, func(t *testing.T) {
			var recipe polynomialNodeComponentsRecipe
			if e := json.Unmarshal(encoded.Bytes(), &recipe); e != nil {
				t.Fatal(e)
			}
			mutate(&recipe)
			changed, e := json.Marshal(recipe)
			if e != nil {
				t.Fatal(e)
			}
			if out, e := ReadThreePolynomialNodeCutJSON(ctx, bytes.NewReader(changed), JSONLimits{}, PolynomialNodeCutOptions{}); out != nil || e == nil {
				t.Fatal("changed original three-contact recipe was accepted", e)
			}
		})
	}
	for _, limits := range []JSONLimits{{MaxBytes: 1}, {MaxControlPoints: 4}, {MaxEntities: 3}} {
		if out, e := ReadThreePolynomialNodeCutJSON(ctx, bytes.NewReader(encoded.Bytes()), limits, PolynomialNodeCutOptions{}); out != nil || e == nil {
			t.Fatal("reader ignored bounds", limits, e)
		}
	}
	proof := PolynomialNodeCutOptions{Proof: geom.GraphPointContactOptions{MaxOperations: 1}}
	if out, e := ReadThreePolynomialNodeCutJSON(ctx, bytes.NewReader(encoded.Bytes()), JSONLimits{}, proof); out != nil || !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("reader ignored the original proof work limit", e)
	}
	base, a, b = nativeThreePolynomialNodeFixture(t, 0, 3, 128)
	if out, e := CutSplineGraphThreePolynomialNodeComponents(ctx, base, a, b, 0, 1e-9, PolynomialNodeCutOptions{}); out != nil || !errors.Is(e, geom.ErrUnrepresentable) {
		t.Fatal("rounded native junction accepted", e)
	}
}
