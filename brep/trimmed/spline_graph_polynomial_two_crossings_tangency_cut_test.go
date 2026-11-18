package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"plainsolid/internal/slowtest"
	"reflect"
	"slices"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func twoCrossingsTangencyCutVolumes(tangent int64, order int) []*big.Rat {
	// Integrate the original gap via its dependent primitive
	// (105/128)(v³/3-(f+g)v²/2+fgv). Evaluate at the actual branch and
	// perimeter polynomials on each rational slab and integrate in u.
	if order == 0 {
		values := [3][2]*big.Rat{
			{big.NewRat(185111353019, 29480655519744), big.NewRat(1017027043817, 9826885173248)},
			{big.NewRat(130414639, 20937965568), big.NewRat(2164486775, 20937965568)},
			{big.NewRat(182136280763, 29480655519744), big.NewRat(1014719914473, 9826885173248)},
		}
		return values[tangent-1][:]
	}
	const leftRight = 2680059592704
	const middle = 460635242496
	values := [3][4]*big.Rat{
		{big.NewRat(126143, leftRight), big.NewRat(17, leftRight), big.NewRat(529, leftRight), big.NewRat(4416065, leftRight)},
		{big.NewRat(210205, middle), big.NewRat(5, middle), big.NewRat(5, middle), big.NewRat(210205, middle)},
		{big.NewRat(4416065, leftRight), big.NewRat(529, leftRight), big.NewRat(17, leftRight), big.NewRat(126143, leftRight)},
	}
	return values[tangent-1][:]
}

func TestNativePolynomialTwoCrossingsTangencyCut(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	for _, tangent := range []int64{1, 2, 3} {
		for axis := 0; axis < 2; axis++ {
			base, a, b := nativeTwoCrossingsTangencyFixture(t, axis, tangent, 4)
			for order, roofs := range [][2]*geom.BSplineSurface{{a, b}, {b, a}} {
				cut, e := CutSplineGraphPolynomialTwoCrossingsTangencyComponents(ctx, base, roofs[0], roofs[1], axis, 1e-9, PolynomialNodeCutOptions{})
				if e != nil {
					t.Fatal(tangent, axis, order, e)
				}
				original := checkTwoCrossingsTangencyCutVolumes(t, cut, tangent, order)
				var raw bytes.Buffer
				if e = cut.WriteJSON(ctx, &raw); e != nil {
					t.Fatal(e)
				}
				reloaded, e := ReadPolynomialTwoCrossingsTangencyCutJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, PolynomialNodeCutOptions{})
				if e != nil || !reflect.DeepEqual(cut.Contacts(), reloaded.Contacts()) {
					t.Fatal("typed original contact ledger changed on source reload", tangent, axis, order, e)
				}
				for i, source := range cut.sources {
					if !geom.SameSplineSurface(source, reloaded.sources[i]) {
						t.Fatal("recipe changed an original spline support", i)
					}
				}
				restored := checkTwoCrossingsTangencyCutVolumes(t, reloaded, tangent, order)
				if !reflect.DeepEqual(original, restored) {
					t.Fatal("exact original volume proof changed on reload", tangent, axis, order)
				}
				checkTwoCrossingsTangencyCutGraph(t, reloaded, restored.Arrangement, tangent, order)
				var replay bytes.Buffer
				if e = reloaded.WriteJSON(ctx, &replay); e != nil || !bytes.Equal(raw.Bytes(), replay.Bytes()) {
					t.Fatal("canonical original-source three-contact Cut recipe", e)
				}
				if tangent == 2 && axis == 0 && order == 0 {
					var points [3]geom.Vec3
					for i, c := range reloaded.Contacts() {
						points[i], e = geom.RepresentedExactPoint(c.Point)
						if e != nil {
							t.Fatal(e)
						}
					}
					checkPolynomialNodeCutStrictCollision(t, [2]Solid(reloaded.Components()), points[:]...)
				}
			}
		}
	}
}

func checkTwoCrossingsTangencyCutVolumes(t *testing.T, cut *PolynomialTwoCrossingsTangencyCut, tangent int64, order int) geom.SplineGraphPolynomialTwoCrossingsTangencyVolumes {
	t.Helper()
	report, e := cut.ExactComponentVolumes(context.Background(), PolynomialMixedCutVolumeOptions{})
	want := twoCrossingsTangencyCutVolumes(tangent, order)
	if e != nil || !report.Arrangement.Proven || !slices.Equal(report.Regions, cut.Regions()) || len(report.Volumes) != len(want) {
		t.Fatal("exact source-bound component volume ledger", tangent, cut.axis, order, report.Regions, e)
	}
	total := new(big.Rat)
	for i, got := range report.Volumes {
		if got.Sign() <= 0 || got.Cmp(want[i]) != 0 {
			t.Fatal("independently integrated rational component volume", tangent, cut.axis, order, i, got, want[i])
		}
		total.Add(total, want[i])
	}
	if report.Total.Cmp(total) != 0 {
		t.Fatal("exact positive Cut material total", report.Total, total)
	}
	return report
}

func checkTwoCrossingsTangencyCutGraph(t *testing.T, cut *PolynomialTwoCrossingsTangencyCut, proof geom.SplineGraphPolynomialTwoCrossingsTangency, tangent int64, order int) {
	t.Helper()
	regions := []int{0, 5}
	if order == 1 {
		regions = []int{1, 2, 3, 4}
	}
	parts := cut.Components()
	if len(parts) != len(regions) || !slices.Equal(cut.Regions(), regions) {
		t.Fatal("original positive region order", cut.Regions())
	}
	axis := proof.FreeAxis
	for i, part := range parts {
		topology := part.Topology()
		if len(topology.Shells) != 1 || topology.EulerCharacteristic != 2 || topology.CollapsedEdges != 0 ||
			part.faces[0].Surface != cut.sources[1] || part.faces[1].Surface != cut.sources[2] || part.faces[0].Reversed || !part.faces[1].Reversed {
			t.Fatal("outward embedded component on both original supports", i, topology)
		}
		if (regions[i] == 2 || regions[i] == 3) &&
			(len(part.faces) != 2 || len(part.edges) != 4 || len(part.faces[0].Outer) != 4 || len(part.faces[1].Outer) != 4) {
			t.Fatal("both bounded lenses retain two caps and four exact edges", i, topology)
		}
		for j, upper := range part.faces[0].Outer {
			curve, ok := upper.Curve.(ExactBezierUV)
			if !ok {
				continue
			}
			lower := part.faces[1].Outer[j]
			if lower.Edge != upper.Edge || !matchesTangencyTrim(lower.Curve, curve) {
				t.Fatal("both original cap coedges retain their precise source restriction", i, j)
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

	_, xyz, e := proof.RepresentedVertices()
	if e != nil {
		t.Fatal(e)
	}
	contacts := cut.Contacts()
	localVertices := make(map[[2]int]bool)
	for i, c := range contacts {
		wantPair := [2]int{0, 1}
		if order == 1 {
			wantPair = [2]int{i, i + 1}
		}
		wantOrder := 1
		if int64(i+1) == tangent {
			wantOrder = 2
		}
		source := proof.Vertices[4+i]
		if c.Components != wantPair || c.Order != wantOrder || c.DerivativeGap.Cmp(proof.DerivativeGap[i]) != 0 ||
			c.Parameters[axis].Cmp(big.NewRat(int64(i+1), 4)) != 0 ||
			!reflect.DeepEqual(c.Parameters, source.Parameters) || !reflect.DeepEqual(c.Point, source.Point) {
			t.Fatal("exact typed contact graph", i, c)
		}
		for j, component := range c.Components {
			part, vertex := parts[component], c.Vertices[j]
			key := [2]int{component, vertex}
			if localVertices[key] || part.vertices[vertex] != xyz[4+i] {
				t.Fatal("distinct exact local component vertex", i, component, vertex)
			}
			localVertices[key] = true
			degree := 0
			for _, edge := range part.edges {
				if edge.Start == vertex || edge.End == vertex {
					degree++
					trim, ok := edge.Curve.(surfaceTrimCurve)
					if !ok {
						t.Fatal("junction must meet an original roof edge", i, component)
					}
					if _, ok := trim.curve.(ExactBezierUV); !ok {
						t.Fatal("junction lost its original exact branch restriction", i, component)
					}
				}
			}
			if degree != 2 {
				t.Fatal("two original edges at each local contact vertex", i, component, degree)
			}
		}
	}
	contacts[0].DerivativeGap.SetInt64(0)
	if cut.Contacts()[0].DerivativeGap.Sign() == 0 {
		t.Fatal("returned rational type data aliases the internal proof")
	}
}

func TestNativePolynomialTwoCrossingsTangencyCutRejections(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	base, a, b := nativeTwoCrossingsTangencyFixture(t, 0, 2, 4)
	cut, e := CutSplineGraphPolynomialTwoCrossingsTangencyComponents(ctx, base, b, a, 0, 1e-9, PolynomialNodeCutOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var raw bytes.Buffer
	if e = cut.WriteJSON(ctx, &raw); e != nil {
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
	for name, change := range map[string]func(*polynomialMixedCutRecipe){
		"later knot":   func(r *polynomialMixedCutRecipe) { r.Sources[2] = source },
		"contact type": func(r *polynomialMixedCutRecipe) { r.Orders[1] = 1 },
		"gap":          func(r *polynomialMixedCutRecipe) { r.DerivativeGaps[1] = "0" },
		"last point":   func(r *polynomialMixedCutRecipe) { r.Contacts[2].Point[2] = "0" },
		"wrong pair":   func(r *polynomialMixedCutRecipe) { r.Contacts[2].Components[0] = 0 },
		"vertex":       func(r *polynomialMixedCutRecipe) { r.Contacts[0].Vertices[1]++ },
		"lost lens":    func(r *polynomialMixedCutRecipe) { r.Regions[2] = 4 },
		"recipe kind":  func(r *polynomialMixedCutRecipe) { r.Kind = nativeThreePolynomialNodeCut },
	} {
		t.Run(name, func(t *testing.T) {
			var recipe polynomialMixedCutRecipe
			if e := json.Unmarshal(raw.Bytes(), &recipe); e != nil {
				t.Fatal(e)
			}
			change(&recipe)
			out, e := json.Marshal(recipe)
			if e != nil {
				t.Fatal(e)
			}
			if q, e := ReadPolynomialTwoCrossingsTangencyCutJSON(ctx, bytes.NewReader(out), JSONLimits{}, PolynomialNodeCutOptions{}); q != nil || e == nil {
				t.Fatal("accepted a changed source or typed contact ledger", e)
			}
		})
	}
	for _, limits := range []JSONLimits{{MaxBytes: 1}, {MaxControlPoints: 4}, {MaxEntities: 3}} {
		if q, e := ReadPolynomialTwoCrossingsTangencyCutJSON(ctx, bytes.NewReader(raw.Bytes()), limits, PolynomialNodeCutOptions{}); q != nil || e == nil {
			t.Fatal("recipe ignored source limits", limits, e)
		}
	}
	report := checkTwoCrossingsTangencyCutVolumes(t, cut, 2, 1)
	for _, options := range []PolynomialMixedCutVolumeOptions{
		{Patch: geom.SurfacePatchOptions{MaxPoles: 4}},
		{Proof: geom.GraphPointContactOptions{MaxCells: 1}},
		{Proof: geom.GraphPointContactOptions{MaxOperations: report.ExactOperations - 1}},
	} {
		got, e := cut.ExactComponentVolumes(ctx, options)
		if !errors.Is(e, geom.ErrConvergence) || !reflect.DeepEqual(got, geom.SplineGraphPolynomialTwoCrossingsTangencyVolumes{}) {
			t.Fatal("volume work limit must fail atomically", options, e)
		}
	}
	base, a, b = nativeTwoCrossingsTangencyFixture(t, 0, 2, 5)
	if q, e := CutSplineGraphPolynomialTwoCrossingsTangencyComponents(ctx, base, a, b, 0, 1e-9, PolynomialNodeCutOptions{}); q != nil || !errors.Is(e, geom.ErrUnrepresentable) {
		t.Fatal("rounded original nonbinary crossing accepted", e)
	}
}
