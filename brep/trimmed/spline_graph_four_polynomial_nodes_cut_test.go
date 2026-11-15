package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"reflect"
	"slices"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func fourPolynomialNodeCutVolumes(order int) []*big.Rat {
	// Independently integrate the original quadratic height gap. Its
	// dependent primitive is (105/128)(v³/3-(f+g)v²/2+fgv).
	// On each slab evaluate the primitive at the original ordered branch
	// ordinates and domain boundaries, then integrate in the free coordinate.
	if order == 0 {
		return []*big.Rat{big.NewRat(88785960167, 14740327759872), big.NewRat(788858940363311, 7547047813054464)}
	}
	return []*big.Rat{
		big.NewRat(751610671, 15094095626108928), big.NewRat(25319, 29480655519744), big.NewRat(4711, 29480655519744),
		big.NewRat(25319, 29480655519744), big.NewRat(751610671, 15094095626108928),
	}
}

func TestNativeFourPolynomialNodeCut(t *testing.T) {
	ctx := context.Background()
	for axis := 0; axis < 2; axis++ {
		base, a, b := nativeFourPolynomialNodeFixture(t, axis, 8)
		for order, roofs := range [][2]*geom.BSplineSurface{{a, b}, {b, a}} {
			cut, e := CutSplineGraphFourPolynomialNodeComponents(ctx, base, roofs[0], roofs[1], axis, 1e-9, PolynomialNodeCutOptions{})
			if e != nil {
				t.Fatal(axis, order, e)
			}
			original := checkNativeFourPolynomialNodeCutVolumes(t, cut, order)
			var raw bytes.Buffer
			if e = cut.WriteJSON(ctx, &raw); e != nil {
				t.Fatal(e)
			}
			loaded, e := ReadFourPolynomialNodeCutJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, PolynomialNodeCutOptions{})
			if e != nil || !reflect.DeepEqual(cut.Contacts(), loaded.Contacts()) {
				t.Fatal("four exact contacts on original-source reload", axis, order, e)
			}
			for i, source := range cut.sources {
				if !geom.SameSplineSurface(source, loaded.sources[i]) {
					t.Fatal("native recipe replaced an original spline basis", i)
				}
			}
			restored := checkNativeFourPolynomialNodeCutVolumes(t, loaded, order)
			if !reflect.DeepEqual(original, restored) {
				t.Fatal("original and reloaded source volume ledgers differ", axis, order)
			}
			checkNativeFourPolynomialNodeCutGraph(t, loaded, restored.Arrangement, order)
			var rewritten bytes.Buffer
			if e = loaded.WriteJSON(ctx, &rewritten); e != nil || !bytes.Equal(raw.Bytes(), rewritten.Bytes()) {
				t.Fatal("canonical four-contact original recipe", e)
			}
			if axis == 0 && order == 0 {
				var pt [4]geom.Vec3
				for i, c := range loaded.Contacts() {
					pt[i], e = geom.RepresentedExactPoint(c.Point)
					if e != nil {
						t.Fatal(e)
					}
				}
				checkPolynomialNodeCutStrictCollision(t, [2]Solid(loaded.Components()), pt[:]...)
			}
		}
	}
}

func checkNativeFourPolynomialNodeCutVolumes(t *testing.T, cut *FourPolynomialNodeCut, order int) geom.SplineGraphFourPolynomialNodeVolumes {
	t.Helper()
	volumes, e := cut.ExactComponentVolumes(context.Background(), FourPolynomialNodeCutVolumeOptions{})
	want := fourPolynomialNodeCutVolumes(order)
	if e != nil || !volumes.Arrangement.Proven || !slices.Equal(volumes.Regions, cut.Regions()) || len(volumes.Volumes) != len(want) {
		t.Fatal("exact original Cut region and volume ledger", volumes.Regions, e)
	}
	total := new(big.Rat)
	for i, v := range volumes.Volumes {
		if v.Sign() <= 0 || v.Cmp(want[i]) != 0 {
			t.Fatal("independent rational positive component volume", i, v, want[i])
		}
		total.Add(total, want[i])
	}
	if volumes.Total.Cmp(total) != 0 {
		t.Fatal("exact total of the positive region volumes", volumes.Total, total)
	}
	return volumes
}

func checkNativeFourPolynomialNodeCutGraph(t *testing.T, r *FourPolynomialNodeCut, proof geom.SplineGraphFourPolynomialNodes, order int) {
	t.Helper()
	regions := []int{0, 6}
	if order == 1 {
		regions = []int{1, 2, 3, 4, 5}
	}
	parts := r.Components()
	if !slices.Equal(regions, r.Regions()) || len(parts) != len(regions) {
		t.Fatal("exact positive signed region order", r.Regions())
	}
	axis := proof.FreeAxis
	for i, part := range parts {
		topology := part.Topology()
		if len(topology.Shells) != 1 || topology.EulerCharacteristic != 2 || topology.CollapsedEdges != 0 ||
			part.faces[0].Surface != r.sources[1] || part.faces[1].Surface != r.sources[2] || part.faces[0].Reversed || !part.faces[1].Reversed {
			t.Fatal("positive native layer on the two original roof supports", i, topology)
		}
		if regions[i] >= 2 && regions[i] <= 4 &&
			(len(part.faces) != 2 || len(part.faces[0].Outer) != 4 || len(part.faces[1].Outer) != 4 || len(part.edges) != 4) {
			t.Fatal("each bounded lens has two roof caps and four exact edges", i, topology)
		}
		for j, top := range part.faces[0].Outer {
			curve, ok := top.Curve.(ExactBezierUV)
			if !ok {
				continue
			}
			bottom := part.faces[1].Outer[j]
			if top.Edge != bottom.Edge || !matchesTangencyTrim(bottom.Curve, curve) {
				t.Fatal("both original owner restrictions on a shared Cut edge", i, j)
			}
			uv, _, e := curve.EvaluateRational(big.NewRat(1, 2))
			if e != nil {
				t.Fatal(e)
			}
			branch := 0
			if uv[1-axis].Cmp(fourPolynomialCutBranchValue(proof.Power[0], uv[axis])) != 0 {
				branch = 1
			}
			checkNativeThreePolynomialBranch(t, curve, axis, proof.Power[branch])
		}
	}
	_, points, e := proof.RepresentedVertices()
	if e != nil {
		t.Fatal(e)
	}
	contacts := r.Contacts()
	used := make(map[[2]int]bool)
	for i, c := range contacts {
		want := [2]int{0, 1}
		if order == 1 {
			want = [2]int{i, i + 1}
		}
		original := proof.Vertices[4+i]
		if c.Components != want || c.Parameters[axis].Cmp(big.NewRat(int64(2*i+1), 8)) != 0 ||
			!reflect.DeepEqual(c.Parameters, original.Parameters) || !reflect.DeepEqual(c.Point, original.Point) {
			t.Fatal("four exact indexed component incidences", i, c)
		}
		for j, k := range c.Components {
			part, v := parts[k], c.Vertices[j]
			identity := [2]int{k, v}
			if used[identity] || part.vertices[v] != points[4+i] {
				t.Fatal("local vertex identity on a separate component", i, k, v)
			}
			used[identity] = true
			degree := 0
			for _, edge := range part.edges {
				if edge.Start == v || edge.End == v {
					degree++
					m, ok := edge.Curve.(surfaceTrimCurve)
					if !ok {
						t.Fatal("shared point must be incident to a source edge", i, k)
					}
					if _, ok = m.curve.(ExactBezierUV); !ok {
						t.Fatal("shared point must be incident to an exact original restriction", i, k)
					}
				}
			}
			if degree != 2 {
				t.Fatal("each component contact has the two original branch incidences", i, k, degree)
			}
		}
	}
	contacts[3].Point[0].SetInt64(999)
	if r.Contacts()[3].Point[0].Cmp(contacts[3].Point[0]) == 0 {
		t.Fatal("returned exact vertex aliases the internal proof")
	}
}

func fourPolynomialCutBranchValue(power []*big.Rat, x *big.Rat) *big.Rat {
	value := new(big.Rat)
	for i := len(power) - 1; i >= 0; i-- {
		value.Mul(value, x).Add(value, power[i])
	}
	return value
}

func TestNativeFourPolynomialNodeCutRejections(t *testing.T) {
	ctx := context.Background()
	base, a, b := nativeFourPolynomialNodeFixture(t, 0, 8)
	cut, e := CutSplineGraphFourPolynomialNodeComponents(ctx, base, b, a, 0, 1e-9, PolynomialNodeCutOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var raw bytes.Buffer
	if e = cut.WriteJSON(ctx, &raw); e != nil {
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
	for name, change := range map[string]func(*polynomialNodeComponentsRecipe){
		"later source knot": func(r *polynomialNodeComponentsRecipe) { r.Sources[2] = altered },
		"fourth point":      func(r *polynomialNodeComponentsRecipe) { r.Contacts[3].Point[2] = "0" },
		"missing contact":   func(r *polynomialNodeComponentsRecipe) { r.Contacts = r.Contacts[:3] },
		"wrong pair":        func(r *polynomialNodeComponentsRecipe) { r.Contacts[3].Components[0] = 1 },
		"wrong vertex":      func(r *polynomialNodeComponentsRecipe) { r.Contacts[0].Vertices[1]++ },
		"lost lens":         func(r *polynomialNodeComponentsRecipe) { r.Regions[2] = 5 },
		"envelope kind":     func(r *polynomialNodeComponentsRecipe) { r.Kind = nativeFourPolynomialNodeEnvelope },
	} {
		t.Run(name, func(t *testing.T) {
			var recipe polynomialNodeComponentsRecipe
			if e := json.Unmarshal(raw.Bytes(), &recipe); e != nil {
				t.Fatal(e)
			}
			change(&recipe)
			encoded, e := json.Marshal(recipe)
			if e != nil {
				t.Fatal(e)
			}
			if result, e := ReadFourPolynomialNodeCutJSON(ctx, bytes.NewReader(encoded), JSONLimits{}, PolynomialNodeCutOptions{}); result != nil || e == nil {
				t.Fatal("altered original source or contact ledger accepted", e)
			}
		})
	}
	for _, limits := range []JSONLimits{{MaxBytes: 1}, {MaxControlPoints: 4}, {MaxEntities: 3}} {
		if result, e := ReadFourPolynomialNodeCutJSON(ctx, bytes.NewReader(raw.Bytes()), limits, PolynomialNodeCutOptions{}); result != nil || e == nil {
			t.Fatal("source recipe ignored limits", limits, e)
		}
	}
	base, a, b = nativeFourPolynomialNodeFixture(t, 0, 6)
	if result, e := CutSplineGraphFourPolynomialNodeComponents(ctx, base, a, b, 0, 1e-9, PolynomialNodeCutOptions{}); result != nil || !errors.Is(e, geom.ErrUnrepresentable) {
		t.Fatal("unrepresentable original rational crossing accepted", e)
	}
	report := checkNativeFourPolynomialNodeCutVolumes(t, cut, 1)
	for _, options := range []FourPolynomialNodeCutVolumeOptions{
		{Patch: geom.SurfacePatchOptions{MaxPoles: 4}},
		{Proof: geom.GraphPointContactOptions{MaxCells: 1}},
		{Proof: geom.GraphPointContactOptions{MaxOperations: report.ExactOperations - 1}},
	} {
		got, e := cut.ExactComponentVolumes(ctx, options)
		if !errors.Is(e, geom.ErrConvergence) || !reflect.DeepEqual(got, geom.SplineGraphFourPolynomialNodeVolumes{}) {
			t.Fatal("bounded volume proof must fail atomically", options, e)
		}
	}
	if got, e := cut.ExactComponentVolumes(ctx, FourPolynomialNodeCutVolumeOptions{
		Proof: geom.GraphPointContactOptions{MaxOperations: report.ExactOperations},
	}); e != nil || !reflect.DeepEqual(got, report) {
		t.Fatal("exact operation boundary", e)
	}
}
