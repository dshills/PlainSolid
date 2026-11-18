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

func quarticTangencyCutVolumes(t *testing.T, profile, order int) []*big.Rat {
	t.Helper()
	// Integrate the original gap via the dependent primitive
	// (105/128)(v³/3-(f+g)v²/2+fgv), evaluated at the actual branch and
	// perimeter polynomials and integrated over the corresponding slabs.
	fixtures := [2][2][]string{
		{
			{"135261/11534336", "92123897/1199570944"},
			{"35/218103808", "35/218103808"},
		},
		{
			{"72663757/5905580032", "12146112808083327/161003686678495232"},
			{"1506635235/117093590311632896", "1153564453125/117093590311632896"},
		},
	}
	want := make([]*big.Rat, len(fixtures[profile][order]))
	for i, raw := range fixtures[profile][order] {
		x, ok := new(big.Rat).SetString(raw)
		if !ok {
			t.Fatal("invalid independent source integral", raw)
		}
		want[i] = x
	}
	return want
}

func TestNativePolynomialQuarticTangencyCut(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	for profile := 0; profile < 2; profile++ {
		for axis := 0; axis < 2; axis++ {
			base, a, b := nativeQuarticTangencyFixture(t, axis, profile)
			for order, roofs := range [][2]*geom.BSplineSurface{{a, b}, {b, a}} {
				cut, e := CutSplineGraphPolynomialQuarticTangencyComponents(ctx, base, roofs[0], roofs[1], axis, 1e-9, PolynomialNodeCutOptions{})
				if e != nil {
					t.Fatal(profile, axis, order, e)
				}
				original := checkQuarticTangencyCutVolumes(t, cut, profile, order)
				var raw bytes.Buffer
				if e = cut.WriteJSON(ctx, &raw); e != nil {
					t.Fatal(e)
				}
				reloaded, e := ReadPolynomialQuarticTangencyCutJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, PolynomialNodeCutOptions{})
				if e != nil || !reflect.DeepEqual(cut.Contact(), reloaded.Contact()) {
					t.Fatal("typed original contact ledger changed on source reload", profile, axis, order, e)
				}
				for i, source := range cut.sources {
					if !geom.SameSplineSurface(source, reloaded.sources[i]) {
						t.Fatal("recipe changed an original spline support", i)
					}
				}
				restored := checkQuarticTangencyCutVolumes(t, reloaded, profile, order)
				if !reflect.DeepEqual(original, restored) {
					t.Fatal("exact original volume proof changed on reload", profile, axis, order)
				}
				checkQuarticTangencyCutGraph(t, reloaded, restored.Arrangement, profile, order)
				var replay bytes.Buffer
				if e = reloaded.WriteJSON(ctx, &replay); e != nil || !bytes.Equal(raw.Bytes(), replay.Bytes()) {
					t.Fatal("canonical original-source quartic-tangency Cut recipe", e)
				}
				if profile == 0 && axis == 0 && order == 0 {
					point, e := geom.RepresentedExactPoint(reloaded.Contact().Point)
					if e != nil {
						t.Fatal(e)
					}
					checkPolynomialNodeCutStrictCollision(t, [2]Solid(reloaded.Components()), point)
				}
			}
		}
	}
}

func checkQuarticTangencyCutVolumes(t *testing.T, cut *PolynomialQuarticTangencyCut, profile, order int) geom.SplineGraphPolynomialQuarticTangencyVolumes {
	t.Helper()
	report, e := cut.ExactComponentVolumes(context.Background(), PolynomialMixedCutVolumeOptions{})
	want := quarticTangencyCutVolumes(t, profile, order)
	if e != nil || !report.Arrangement.Proven || !slices.Equal(report.Regions, cut.Regions()) || len(report.Volumes) != len(want) {
		t.Fatal("exact source-bound component volume ledger", profile, cut.axis, order, report.Regions, e)
	}
	total := new(big.Rat)
	for i, got := range report.Volumes {
		if got.Sign() <= 0 || got.Cmp(want[i]) != 0 {
			t.Fatal("independently integrated rational component volume", profile, cut.axis, order, i, got, want[i])
		}
		total.Add(total, want[i])
	}
	if report.Total.Cmp(total) != 0 {
		t.Fatal("exact positive Cut material total", report.Total, total)
	}
	return report
}

func checkQuarticTangencyCutGraph(t *testing.T, cut *PolynomialQuarticTangencyCut, proof geom.SplineGraphPolynomialQuarticTangency, profile, order int) {
	t.Helper()
	regions := []int{0, 3}
	if order == 1 {
		regions = []int{1, 2}
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
	contact, source := cut.Contact(), proof.Vertices[4]
	station := [2]*big.Rat{big.NewRat(1, 2), big.NewRat(3, 8)}[profile]
	if contact.Components != [2]int{0, 1} || contact.Order != 4 || contact.DerivativeGap.Cmp(proof.FourthDerivativeGap) != 0 ||
		contact.Parameters[axis].Cmp(station) != 0 ||
		!reflect.DeepEqual(contact.Parameters, source.Parameters) || !reflect.DeepEqual(contact.Point, source.Point) {
		t.Fatal("exact order-four contact and both component indices", contact)
	}
	for j, component := range contact.Components {
		part, vertex := parts[component], contact.Vertices[j]
		if part.vertices[vertex] != xyz[4] {
			t.Fatal("separate local component identity at the exact source point", component, vertex)
		}
		degree := 0
		for _, edge := range part.edges {
			if edge.Start == vertex || edge.End == vertex {
				degree++
				trim, ok := edge.Curve.(surfaceTrimCurve)
				if !ok {
					t.Fatal("junction must meet an original roof edge", component)
				}
				if _, ok := trim.curve.(ExactBezierUV); !ok {
					t.Fatal("junction lost its exact original branch restriction", component)
				}
			}
		}
		if degree != 2 {
			t.Fatal("two original edges at the local branch vertex", component, degree)
		}
	}
	contact.DerivativeGap.SetInt64(0)
	if cut.Contact().DerivativeGap.Sign() == 0 {
		t.Fatal("returned rational difference aliases the internal proof")
	}
}

func TestNativePolynomialQuarticTangencyCutRejections(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	base, a, b := nativeQuarticTangencyFixture(t, 0, 0)
	cut, e := CutSplineGraphPolynomialQuarticTangencyComponents(ctx, base, b, a, 0, 1e-9, PolynomialNodeCutOptions{})
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
		"contact type": func(r *polynomialMixedCutRecipe) { r.Orders[0] = 1 },
		"gap":          func(r *polynomialMixedCutRecipe) { r.DerivativeGaps[0] = "0" },
		"last point":   func(r *polynomialMixedCutRecipe) { r.Contacts[0].Point[2] = "0" },
		"wrong pair":   func(r *polynomialMixedCutRecipe) { r.Contacts[0].Components[1] = 0 },
		"vertex":       func(r *polynomialMixedCutRecipe) { r.Contacts[0].Vertices[1]++ },
		"lost slab":    func(r *polynomialMixedCutRecipe) { r.Regions[1] = 3 },
		"recipe kind":  func(r *polynomialMixedCutRecipe) { r.Kind = nativePolynomialCubicCrossingCut },
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
			if q, e := ReadPolynomialQuarticTangencyCutJSON(ctx, bytes.NewReader(out), JSONLimits{}, PolynomialNodeCutOptions{}); q != nil || e == nil {
				t.Fatal("accepted a changed source or typed contact ledger", e)
			}
		})
	}
	for _, limits := range []JSONLimits{{MaxBytes: 1}, {MaxControlPoints: 4}, {MaxEntities: 3}} {
		if q, e := ReadPolynomialQuarticTangencyCutJSON(ctx, bytes.NewReader(raw.Bytes()), limits, PolynomialNodeCutOptions{}); q != nil || e == nil {
			t.Fatal("recipe ignored source limits", limits, e)
		}
	}
	report := checkQuarticTangencyCutVolumes(t, cut, 0, 1)
	for _, options := range []PolynomialMixedCutVolumeOptions{
		{Patch: geom.SurfacePatchOptions{MaxPoles: 4}},
		{Proof: geom.GraphPointContactOptions{MaxCells: 1}},
		{Proof: geom.GraphPointContactOptions{MaxOperations: report.ExactOperations - 1}},
	} {
		got, e := cut.ExactComponentVolumes(ctx, options)
		if !errors.Is(e, geom.ErrConvergence) || !reflect.DeepEqual(got, geom.SplineGraphPolynomialQuarticTangencyVolumes{}) {
			t.Fatal("volume work limit must fail atomically", options, e)
		}
	}
	base, a, b = nativeQuarticTangencyFixture(t, 0, 2)
	if q, e := CutSplineGraphPolynomialQuarticTangencyComponents(ctx, base, a, b, 0, 1e-9, PolynomialNodeCutOptions{}); q != nil || !errors.Is(e, geom.ErrUnrepresentable) {
		t.Fatal("rounded original nonbinary order-four point accepted", e)
	}
}
