package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"slices"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func polynomialMixedCutVolumes(numerator int64, order int) []float64 {
	// For each original free-coordinate slab, integrate the quadratic gap
	// in the dependent coordinate. An exterior fiber has primitive
	// l²h/2-l³/6 for the near and far branch distances l and h. Each
	// between-branch fiber has integral |g-f|³/6. Multiply by 105/128,
	// then integrate the polynomial on the three exact slabs.
	if numerator == 1 {
		if order == 0 {
			return []float64{55923235383397.0 / 5031365208702976, 396501040506981.0 / 5031365208702976}
		}
		return []float64{1216335.0 / 457396837154816, 335.0 / 457396837154816, 653735.0 / 893353197568}
	}
	if order == 0 {
		return []float64{57845556889919.0 / 5031365208702976, 389508707518783.0 / 5031365208702976}
	}
	return []float64{1131255.0 / 9826885173248, 6975.0 / 5031365208702976, 92195135.0 / 5031365208702976}
}

func TestNativePolynomialMixedCut(t *testing.T) {
	ctx := context.Background()
	for _, numerator := range []int64{1, 3} {
		for axis := 0; axis < 2; axis++ {
			base, a, b := nativePolynomialMixedFixture(t, axis, numerator, 4)
			for order, roofs := range [][2]*geom.BSplineSurface{{a, b}, {b, a}} {
				r, e := CutSplineGraphPolynomialMixedComponents(ctx, base, roofs[0], roofs[1], axis, 1e-9, PolynomialNodeCutOptions{})
				if e != nil {
					t.Fatal(numerator, axis, order, e)
				}
				var raw bytes.Buffer
				if e = r.WriteJSON(ctx, &raw); e != nil {
					t.Fatal(e)
				}
				loaded, e := ReadPolynomialMixedCutJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, PolynomialNodeCutOptions{})
				if e != nil || !reflect.DeepEqual(loaded.Contacts(), r.Contacts()) {
					t.Fatal("original mixed Cut reload", e)
				}
				for i, source := range r.sources {
					if !geom.SameSplineSurface(source, loaded.sources[i]) {
						t.Fatal("native recipe altered an original support", i)
					}
				}
				var rewritten bytes.Buffer
				if e = loaded.WriteJSON(ctx, &rewritten); e != nil || !bytes.Equal(raw.Bytes(), rewritten.Bytes()) {
					t.Fatal("canonical mixed Cut recipe", e)
				}
				var patches [2]*geom.SurfacePatch
				for i, roof := range roofs {
					patches[i], e = geom.NewSurfacePatch(roof, geom.SurfacePatchOptions{})
					if e != nil {
						t.Fatal(e)
					}
				}
				proof, e := geom.DiscoverSplineGraphPolynomialMixedContact(ctx, patches[0], patches[1], axis, geom.GraphPointContactOptions{})
				if e != nil {
					t.Fatal(e)
				}
				checkNativePolynomialMixedCutGraph(t, loaded, proof, order)
				want := polynomialMixedCutVolumes(numerator, order)
				for i, part := range loaded.Components() {
					options := parametric.IntegrationOptions{RelativeTolerance: 1e-9}
					if order == 1 && i == 1 {
						// The generic full mass integrator checks tiny lens
						// moments after subtraction of the two cap fluxes.
						options.RelativeTolerance = 1e-5
					}
					m, e := part.MassProperties(ctx, options)
					if e != nil || m.Volume <= 0 || math.Abs(m.Volume-want[i]) > math.Max(3e-15, 1e-8*want[i]) {
						t.Fatal("independent exact positive slab integral", numerator, axis, order, i, m.Volume, want[i], e)
					}
				}
				if numerator == 1 && axis == 0 && order == 0 {
					var points [2]geom.Vec3
					for i, c := range loaded.Contacts() {
						points[i], e = geom.RepresentedExactPoint(c.Point)
						if e != nil {
							t.Fatal(e)
						}
					}
					checkPolynomialNodeCutStrictCollision(t, [2]Solid(loaded.Components()), points[:]...)
				}
			}
		}
	}
}

func checkNativePolynomialMixedCutGraph(t *testing.T, r *PolynomialMixedCut, proof geom.SplineGraphPolynomialMixedContact, order int) {
	t.Helper()
	regions := []int{0, 4}
	if order == 1 {
		regions = []int{1, 2, 3}
	}
	parts := r.Components()
	if len(parts) != len(regions) || !slices.Equal(regions, r.Regions()) {
		t.Fatal("only positive original regions are retained", r.Regions())
	}
	for i, p := range parts {
		topology := p.Topology()
		if len(topology.Shells) != 1 || topology.EulerCharacteristic != 2 || topology.CollapsedEdges != 0 ||
			p.faces[0].Surface != r.sources[1] || p.faces[1].Surface != r.sources[2] || p.faces[0].Reversed || !p.faces[1].Reversed {
			t.Fatal("positive component and its two original roof caps", i, topology)
		}
		count := 0
		for j, top := range p.faces[0].Outer {
			curve, ok := top.Curve.(ExactBezierUV)
			if !ok {
				continue
			}
			count++
			bottom := p.faces[1].Outer[j]
			if top.Edge != bottom.Edge || !matchesTangencyTrim(bottom.Curve, curve) {
				t.Fatal("both original roof coedges must retain their exact shared curve", i, j)
			}
			branch := len(curve.control) - 4
			if branch < 0 || branch > 1 {
				t.Fatal("lost original mixed curve degree", i, j, curve)
			}
			checkNativeThreePolynomialBranch(t, curve, proof.FreeAxis, proof.Power[branch])
		}
		if regions[i] == 2 && (len(p.faces) != 2 || len(p.edges) != 4 || count != 4) {
			t.Fatal("positive interior lens has two original caps and four exact contour edges", topology)
		}
	}
	contacts := r.Contacts()
	seen := make(map[[2]int]bool)
	for i, c := range contacts {
		want := [2]int{0, 1}
		if order == 1 {
			want = [2]int{i, i + 1}
		}
		p := proof.Vertices[proof.Nodes[i].Vertex]
		if c.Components != want || c.Order != proof.ContactOrder[i] || c.DerivativeGap.Cmp(proof.DerivativeGap[i]) != 0 ||
			!reflect.DeepEqual(c.Parameters, p.Parameters) || !reflect.DeepEqual(c.Point, p.Point) {
			t.Fatal("exact ordered crossing/tangency contact graph", i, c)
		}
		point, e := geom.RepresentedExactPoint(p.Point)
		if e != nil {
			t.Fatal(e)
		}
		for j, k := range c.Components {
			part, vertex := parts[k], c.Vertices[j]
			key := [2]int{k, vertex}
			if seen[key] || part.vertices[vertex] != point {
				t.Fatal("separate original local vertex identity", i, k, vertex)
			}
			seen[key] = true
			degree := 0
			for _, edge := range part.edges {
				if edge.Start != vertex && edge.End != vertex {
					continue
				}
				degree++
				m, ok := edge.Curve.(surfaceTrimCurve)
				if !ok {
					t.Fatal("local contact must be incident to the original roof branch", i, k)
				}
				if _, ok := m.curve.(ExactBezierUV); !ok {
					t.Fatal("local contact edge lost the original polynomial", i, k)
				}
			}
			if degree != 2 {
				t.Fatal("exact degree-two branch vertex on this component", i, k, degree)
			}
		}
	}
	for i, a := range parts {
		for j := i + 1; j < len(parts); j++ {
			for v, p := range a.vertices {
				for w, q := range parts[j].vertices {
					if p != q {
						continue
					}
					found := false
					for _, c := range contacts {
						found = found || c.Components == [2]int{i, j} && c.Vertices == [2]int{v, w}
					}
					if !found {
						t.Fatal("component contact outside the exact ledger", i, j, v, w)
					}
				}
			}
		}
	}
	contacts[0].DerivativeGap.SetInt64(0)
	if r.Contacts()[0].DerivativeGap.Sign() == 0 {
		t.Fatal("caller changed the retained contact certificate")
	}
}

func TestNativePolynomialMixedCutReloadRejections(t *testing.T) {
	ctx := context.Background()
	base, a, b := nativePolynomialMixedFixture(t, 0, 1, 4)
	r, e := CutSplineGraphPolynomialMixedComponents(ctx, base, b, a, 0, 1e-9, PolynomialNodeCutOptions{})
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
	changedRoof, e := parametric.EncodeSurface(changed)
	if e != nil {
		t.Fatal(e)
	}
	for name, change := range map[string]func(*polynomialMixedCutRecipe){
		"contact type": func(r *polynomialMixedCutRecipe) { r.Orders[1] = 1 },
		"derivative":   func(r *polynomialMixedCutRecipe) { r.DerivativeGaps[1] = "0" },
		"pair":         func(r *polynomialMixedCutRecipe) { r.Contacts[1].Components[0] = 0 },
		"point":        func(r *polynomialMixedCutRecipe) { r.Contacts[1].Point[2] = "0" },
		"vertex":       func(r *polynomialMixedCutRecipe) { r.Contacts[0].Vertices[1]++ },
		"omitted lens": func(r *polynomialMixedCutRecipe) { r.Regions = []int{1, 3} },
		"source order": func(r *polynomialMixedCutRecipe) { r.Sources[1], r.Sources[2] = r.Sources[2], r.Sources[1] },
		"later knot":   func(r *polynomialMixedCutRecipe) { r.Sources[2] = changedRoof },
		"recipe kind":  func(r *polynomialMixedCutRecipe) { r.Kind = nativeTwoPolynomialNodeCut },
	} {
		t.Run(name, func(t *testing.T) {
			var recipe polynomialMixedCutRecipe
			if e := json.Unmarshal(raw.Bytes(), &recipe); e != nil {
				t.Fatal(e)
			}
			change(&recipe)
			modified, e := json.Marshal(recipe)
			if e != nil {
				t.Fatal(e)
			}
			if out, e := ReadPolynomialMixedCutJSON(ctx, bytes.NewReader(modified), JSONLimits{}, PolynomialNodeCutOptions{}); out != nil || e == nil {
				t.Fatal("changed positive sector or contact certificate accepted", e)
			}
		})
	}
	for _, limits := range []JSONLimits{{MaxBytes: 4}, {MaxControlPoints: 4}, {MaxEntities: 3}} {
		if out, e := ReadPolynomialMixedCutJSON(ctx, bytes.NewReader(raw.Bytes()), limits, PolynomialNodeCutOptions{}); out != nil || e == nil {
			t.Fatal("native recipe bound ignored", limits, e)
		}
	}
	lowBudget := PolynomialNodeCutOptions{Proof: geom.GraphPointContactOptions{MaxOperations: 1}}
	if out, e := ReadPolynomialMixedCutJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, lowBudget); out != nil || !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("proof work bound ignored", e)
	}
	base, a, b = nativePolynomialMixedFixture(t, 0, 1, 3)
	if out, e := CutSplineGraphPolynomialMixedComponents(ctx, base, a, b, 0, 1e-9, PolynomialNodeCutOptions{}); out != nil || !errors.Is(e, geom.ErrUnrepresentable) {
		t.Fatal("nonbinary original crossing must fail atomically", e)
	}
}
