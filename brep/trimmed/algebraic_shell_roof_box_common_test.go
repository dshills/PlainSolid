package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"os"
	"os/exec"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestOrderedRoofBoxCommonNative(t *testing.T) {
	ctx := context.Background()
	cube, _ := makeOpenSplineBox(t, [3]float64{}, [3]float64{2, 2, 2}, 0)
	shared, _ := makeOpenSplineBox(t, [3]float64{1, 0, 0}, [3]float64{3, 2, 2}, 0)
	contact, _ := makeOpenSplineBox(t, [3]float64{2, 1, 1}, [3]float64{4, 3, 3}, 0)
	if s, h, err := IntersectOrderedRoofBoxes(ctx, cube, contact, MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{}); err != nil || !h.ProjectedContact || s.Topology().Faces != 0 {
		t.Fatal("face-only contact is regularized away", s.Topology(), h, err)
	}
	curved, _ := crossingFootprintRoofShell(t, false)
	plane, _ := makeOpenSplineBox(t, [3]float64{.25, 0, -.75}, [3]float64{.75, 1, 1}, 0)
	var ref struct {
		Volume, Area  float64
		Centroid      [3]float64
		Solids, Euler int
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_ORDERED_ROOF_COMMON"); oracle != "" {
		data, err := exec.CommandContext(ctx, oracle, "ordered-common").Output()
		if err != nil || json.Unmarshal(data, &ref) != nil || ref.Solids != 1 || ref.Euler != 2 {
			t.Fatal("independent OCCT native curved Common", ref, err, string(data))
		}
	}
	for i, tc := range []struct {
		a, b Solid
		v    *big.Rat
		roof int
	}{{cube, shared, big.NewRat(4, 1), 0}, {curved, plane, big.NewRat(193, 512), 0}, {plane, curved, big.NewRat(193, 512), 1}} {
		s, h, err := IntersectOrderedRoofBoxes(ctx, tc.a, tc.b, MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{})
		if err != nil || len(h.Faces) != 6 || h.Faces[0].Operand != tc.roof || !h.ProjectedContact {
			t.Fatal("original owner of native ordered common", i, h, err)
		}
		if top := s.Topology(); top.Faces != 6 || top.Edges != 12 || top.EulerCharacteristic != 2 {
			t.Fatal("closed common topology", top)
		}
		agreement, err := s.ValidateEdgeAgreement(ctx, edgecheck.Options{})
		if err != nil || !agreement.Complete || len(agreement.Uses) != 24 {
			t.Fatal("continuous retained edge agreement", agreement, err)
		}
		for _, u := range agreement.Uses {
			if u.Report.Status != edgecheck.Certified || u.Report.SquaredUpperBound == nil || u.Report.SquaredUpperBound.Sign() != 0 {
				t.Fatal("nonzero continuous model-edge/face error", u)
			}
		}
		mass, err := s.MassProperties(ctx, parametric.IntegrationOptions{})
		want, _ := tc.v.Float64()
		if err != nil || math.Abs(mass.Volume-want) > 1e-9 {
			t.Fatal("independent entire height integral", mass, want, err)
		}
		if i > 0 && ref.Solids != 0 && (math.Abs(mass.Volume-ref.Volume) > 1e-9 || math.Abs(mass.Area-ref.Area) > 1e-8 || mass.Centroid.Sub(geom.Vec3{X: ref.Centroid[0], Y: ref.Centroid[1], Z: ref.Centroid[2]}).Length() > 1e-8) {
			t.Fatal("native original curved Common disagrees with live OCCT", mass, ref)
		}
		bound, err := BoundRoofBoxVolume(ctx, tc.a, tc.b, parametric.Intersection, MixedRoofQueryOptions{}, MixedRoofVolumeOptions{})
		if err != nil || bound.Volume.Lower.Cmp(tc.v) != 0 || bound.Volume.Upper.Cmp(tc.v) != 0 {
			t.Fatal("direct original proof and assembled geometry differ", bound, err)
		}
	}
	dispatched, ledger, err := CombineSplineShells(ctx, curved, plane, parametric.Intersection, SplineShellCombineOptions{OrderedRoofCommon: &SplineShellRoofCommonOptions{}})
	if err != nil || ledger.Method != "ordered-roof-common" || dispatched.Topology().Faces != 6 || len(ledger.FacesA[1]) != 1 || ledger.FacesA[1][0] != 0 {
		t.Fatal("opt-in dispatcher preserves original curved face ancestry", ledger, err)
	}
	cross, _ := crossingFootprintRoofShell(t, true)
	if out, h, err := IntersectOrderedRoofBoxes(ctx, curved, cross, MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{}); !errors.Is(err, geom.ErrUnrepresentable) || out.Topology().Faces != 0 || len(h.Faces) != 0 {
		t.Fatal("crossing roofs must fail this sufficient proof atomically", out.Topology(), h, err)
	}
	if out, h, err := IntersectOrderedRoofBoxes(ctx, cube, shared, MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{MaxOperations: 1}); !errors.Is(err, geom.ErrConvergence) || out.Topology().Faces != 0 || len(h.Faces) != 0 {
		t.Fatal("budget exhaustion leaked topology", out.Topology(), h, err)
	}
}

func TestOrderedRoofBoxCommonNativeWeighted(t *testing.T) {
	ctx := context.Background()
	stock := weightedIdentityRoofStock(t, false, 2)
	out, h, err := IntersectOrderedRoofBoxes(ctx, stock, stock, MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{})
	if err != nil || len(h.Faces) != 6 || out.Topology().Edges != 12 {
		t.Fatal("qualified exact weighted roof", h, err)
	}
	for _, owner := range h.Faces {
		if owner.Operand != 0 {
			t.Fatal("coincident owner policy", owner)
		}
	}
	// (2+x²)/(1+x) = x−1+3/(1+x). The height at X=1/2 remains exactly 3/2.
	face, ok := out.Faces()[0].Surface.(*geom.ExactBezierSurface)
	if !ok {
		t.Fatal("the selected original roof lost its exact chart")
	}
	jet, err := face.ExactPatch().EvaluateRational(ctx, big.NewRat(1, 2), big.NewRat(1, 4), geom.SurfacePatchEvaluationOptions{})
	if err != nil || jet.Point[2].Cmp(big.NewRat(3, 2)) != 0 {
		t.Fatal("restricted rational geometry", jet, err)
	}
	mass, err := out.MassProperties(ctx, parametric.IntegrationOptions{})
	if err != nil || math.Abs(mass.Volume-(3*math.Log(2)-.5)) > 1e-8 {
		t.Fatal("independent rational roof antiderivative", mass, err)
	}
	var native bytes.Buffer
	if err := WriteJSON(&native, out); err != nil {
		t.Fatal("native weighted Common persistence", err)
	}
	back, err := ReadJSONContext(ctx, &native, JSONLimits{})
	if err != nil || back.Topology().EulerCharacteristic != 2 {
		t.Fatal("native weighted Common reload", back.Topology(), err)
	}
	reloaded, err := back.MassProperties(ctx, parametric.IntegrationOptions{})
	if err != nil || math.Abs(reloaded.Volume-mass.Volume) > 1e-11 {
		t.Fatal("native exact recipe changed weighted volume", reloaded, err)
	}
}

func TestOrderedRoofBoxRedundantExactKnots(t *testing.T) {
	ctx := context.Background()
	stock := weightedIdentityRoofStockWithKnots(t, false, 2, true)
	flat, _ := makeOpenSplineBox(t, [3]float64{0, 0, -1}, [3]float64{1, 1, 3}, 0)
	result, ledger, err := IntersectOrderedRoofBoxes(ctx, stock, flat, MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{})
	if err != nil || len(ledger.Faces) != 6 || ledger.Faces[0] != (MixedRoofFaceOwner{0, 1}) || result.Topology().EulerCharacteristic != 2 {
		t.Fatal("source exact knot refinement is geometrically redundant", result.Topology(), ledger, err)
	}
	source, err := exactFaceSupportPatch(stock.faces[1], geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	u, v := source.KnotBreaks()
	if len(u) == 2 && len(v) == 2 {
		t.Fatal("expected a genuinely multispan source")
	}
	roof := result.Faces()[0].Surface.(*geom.ExactBezierSurface).ExactPatch()
	for _, x := range []*big.Rat{big.NewRat(0, 1), big.NewRat(2, 5), big.NewRat(3, 4), big.NewRat(1, 1)} {
		y := big.NewRat(1, 3)
		a, err := source.EvaluateRational(ctx, x, y, geom.SurfacePatchEvaluationOptions{})
		b, err2 := roof.EvaluateRational(ctx, x, y, geom.SurfacePatchEvaluationOptions{})
		if err != nil || err2 != nil {
			t.Fatal(err, err2)
		}
		for i := range a.Point {
			if a.Point[i].Cmp(b.Point[i]) != 0 || a.Du[i].Cmp(b.Du[i]) != 0 || a.Dv[i].Cmp(b.Dv[i]) != 0 {
				t.Fatal("point or derivative changed after exact global recovery", x, i)
			}
		}
	}
}

func TestOrderedRoofBoxPiecewiseCommon(t *testing.T) {
	ctx := context.Background()
	creased := weightedIdentityRoofStockMode(t, false, 2, false, true)
	flat, _ := makeOpenSplineBox(t, [3]float64{0, 0, 0}, [3]float64{1, 1, 3}, 0)
	split, h, err := IntersectOrderedRoofBoxes(ctx, creased, flat, MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{})
	if err != nil || len(h.Faces) != 10 {
		t.Fatal("both original roof spans must survive", h, err)
	}
	top := split.Topology()
	if len(top.Shells) != 1 || top.EulerCharacteristic != 2 || top.Faces != 10 || top.Edges != 20 {
		t.Fatal("the knot grid must not add an interior partition wall", top)
	}
	owner := MixedRoofFaceOwner{Operand: 0, Face: 1}
	var slopes []*big.Rat
	for i, parent := range h.Faces {
		if parent != owner {
			continue
		}
		p := split.Faces()[i].Surface.(*geom.ExactBezierSurface).ExactPatch()
		j, e := p.EvaluateRational(ctx, big.NewRat(1, 2), big.NewRat(1, 3), geom.SurfacePatchEvaluationOptions{})
		if e != nil || j.Point[2].Cmp(big.NewRat(5, 2)) != 0 {
			t.Fatal("the seam is an exact original crease", j, e)
		}
		slopes = append(slopes, j.Du[2])
	}
	if len(slopes) != 2 || slopes[0].Cmp(big.NewRat(1, 1)) != 0 || slopes[1].Cmp(big.NewRat(-1, 1)) != 0 {
		t.Fatal("the seam must retain the sharp change of roof normal", slopes)
	}
	mass, err := split.MassProperties(ctx, parametric.IntegrationOptions{})
	if err != nil || math.Abs(mass.Volume-9./4) > 1e-12 || math.Abs(mass.Area-(9.5+math.Sqrt2)) > 1e-12 {
		t.Fatal("independent piecewise prism integral", mass, err)
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_ORDERED_ROOF_COMMON"); oracle != "" {
		data, err := exec.CommandContext(ctx, oracle, "piecewise-ordered-common").Output()
		var ref struct {
			Volume, Area  float64
			Centroid      [3]float64
			Solids, Euler int
		}
		if err != nil || json.Unmarshal(data, &ref) != nil || ref.Solids != 1 || ref.Euler != 2 || math.Abs(ref.Volume-mass.Volume) > 1e-9 || math.Abs(ref.Area-mass.Area) > 1e-9 {
			t.Fatal("pinned OCCT creased intersection", ref, err, string(data))
		}
	}
	var native bytes.Buffer
	if err := WriteJSON(&native, split); err != nil {
		t.Fatal(err)
	}
	if restored, err := ReadJSONContext(ctx, &native, JSONLimits{}); err != nil || restored.Topology().EulerCharacteristic != 2 {
		t.Fatal("native piecewise roof must revalidate on read", err)
	}
	flatCrossing, _ := makeOpenSplineBox(t, [3]float64{0, 0, 0}, [3]float64{1, 1, 2.25}, 0)
	if s, h, err := IntersectOrderedRoofBoxes(ctx, creased, flatCrossing, MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{}); !errors.Is(err, geom.ErrUnrepresentable) || s.Topology().Faces != 0 || len(h.Faces) != 0 {
		t.Fatal("roof crossings inside span interiors need an imprint", s.Topology(), h, err)
	}
	tilted := orderedRoofParabolaSourceControls(t, [3]float64{1.5, 2.5, 3.5})
	switched, h, err := IntersectOrderedRoofBoxes(ctx, creased, tilted, MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{})
	if err != nil || switched.Topology().EulerCharacteristic != 2 || len(h.Faces) != 10 || h.Faces[0] != (MixedRoofFaceOwner{Operand: 1, Face: 1}) || h.Faces[5] != owner {
		t.Fatal("the original active roof switches on a pre-existing exact knot", switched.Topology(), h, err)
	}
	mass, err = switched.MassProperties(ctx, parametric.IntegrationOptions{})
	// Integrate 1.5+2X on the left half (1) and 3-X on the right (9/8).
	if err != nil || math.Abs(mass.Volume-17./8) > 1e-12 {
		t.Fatal("distinct roof owners must meet at the original knot without an interior wall", mass, err)
	}
}
func orderedRoofTangentSource(t *testing.T) Solid {
	t.Helper()
	return orderedRoofParabolaSource(t, 1, .5)
}

func orderedRoofParabolaSource(t *testing.T, sign, center float64) Solid {
	t.Helper()
	return orderedRoofParabolaSourceControls(t, [3]float64{1 + sign*center*center, 1 + sign*center*(center-1), 1 + sign*(center-1)*(center-1)})
}

func orderedRoofParabolaSourceControls(t *testing.T, z [3]float64) Solid {
	t.Helper()
	ctx := context.Background()
	x := [3]float64{0, .5, 1}
	y := [2]float64{-.125, 1.125}
	floor := -.5
	var parts []SewFace
	add := func(du int, d geom.ParameterRectangle, reversed bool, point func(int, int) geom.Vec3) {
		t.Helper()
		uKnots, vKnots := make([]float64, du*2+2), []float64{d.VMin, d.VMin, d.VMax, d.VMax}
		var controls []geom.Vec3
		for i := 0; i <= du; i++ {
			uKnots[i], uKnots[i+du+1] = d.UMin, d.UMax
			controls = append(controls, point(i, 0), point(i, 1))
		}
		p, err := geom.NewBSplineSurface(du, 1, du+1, 2, controls, uKnots, vKnots, nil)
		if err != nil {
			t.Fatal(err)
		}
		face, err := SplineFaceRectangle(p, d, reversed)
		if err != nil {
			t.Fatal(err)
		}
		parts = append(parts, face)
	}
	xy := geom.ParameterRectangle{UMax: 1, VMin: y[0], VMax: y[1]}
	add(1, xy, true, func(i, j int) geom.Vec3 { return geom.Vec3{X: x[2*i], Y: y[j], Z: floor} })
	add(2, xy, false, func(i, j int) geom.Vec3 { return geom.Vec3{X: x[i], Y: y[j], Z: z[i]} })
	for i := range 2 {
		add(1, geom.ParameterRectangle{UMin: y[0], UMax: y[1], VMax: 1}, i == 0, func(j, k int) geom.Vec3 {
			return geom.Vec3{X: x[2*i], Y: y[j], Z: [2]float64{floor, z[2*i]}[k]}
		})
		add(2, geom.ParameterRectangle{UMax: 1, VMax: 1}, i == 1, func(j, k int) geom.Vec3 {
			return geom.Vec3{X: x[j], Y: y[i], Z: [2]float64{floor, z[j]}[k]}
		})
	}
	curved, _, err := Sew(ctx, parts, SewOptions{Tolerance: 1e-10})
	if err != nil {
		t.Fatal(err)
	}
	return curved
}

func TestOrderedRoofBoxInteriorFloorContact(t *testing.T) {
	ctx := context.Background()
	tool := orderedRoofParabolaSource(t, -1, .5)
	stock, _ := makeOpenSplineBox(t, [3]float64{0, -.125, 1}, [3]float64{1, 1.125, 2}, 0)
	if out, h, err := IntersectOrderedRoofBoxes(ctx, stock, tool, MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{}); err != nil || !h.ProjectedContact || out.Topology().Faces != 0 {
		t.Fatal("interior line contact has no regularized common", out.Topology(), h, err)
	}
	if out, h, err := SubtractOrderedRoofBoxes(ctx, stock, tool, MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{}); err != nil || len(h.Faces) != 6 || out.faces[0].Surface != stock.faces[0].Surface {
		t.Fatal("contact-only cut must retain the authenticated stock", out.Topology(), h, err)
	}
	tool = orderedRoofParabolaSource(t, 1, .5)
	if out, h, err := IntersectOrderedRoofBoxes(ctx, stock, tool, MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{}); err == nil || out.Topology().Faces != 0 || len(h.Faces) != 0 {
		t.Fatal("positive material pinched at the interior line is unresolved", out.Topology(), h, err)
	}
	if out, h, err := SubtractOrderedRoofBoxes(ctx, stock, tool, MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{}); err == nil || out.Topology().Faces != 0 || len(h.Faces) != 0 {
		t.Fatal("changing cut floor contact remains outside the sewn-band proof", out.Topology(), h, err)
	}
}

func TestOrderedRoofBoxAdaptiveHeightOrder(t *testing.T) {
	ctx := context.Background()
	// A weak order at X=3/8 requires more than the previous single
	// halving. A strictly lower tool just below the minimum proves a
	// positive remaining band despite cancelling monomial coefficients.
	stock := orderedRoofParabolaSource(t, 1, .375)
	for _, tb := range []struct {
		roof float64
		cut  bool
	}{{1, false}, {1 - 1./65536, true}} {
		tool, _ := makeOpenSplineBox(t, [3]float64{0, -.125, -1}, [3]float64{1, 1.125, tb.roof}, 0)
		common, h, err := IntersectOrderedRoofBoxes(ctx, stock, tool, MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{})
		if err != nil || len(h.Faces) != 6 || h.Faces[0].Operand != 1 || common.Topology().EulerCharacteristic != 2 {
			t.Fatal("adaptive chart cover certifies the lower roof", h, err)
		}
		cut, ancestry, err := SubtractOrderedRoofBoxes(ctx, stock, tool, MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{})
		if !tb.cut {
			if err == nil || cut.Topology().Faces != 0 || len(ancestry.Faces) != 0 {
				t.Fatal("strictness may not be inferred from weak height order", cut.Topology(), ancestry, err)
			}
			continue
		}
		if err != nil || len(ancestry.Faces) != 6 {
			t.Fatal("adaptive strict order qualifies the positive band", ancestry, err)
		}
		mass, err := cut.MassProperties(ctx, parametric.IntegrationOptions{})
		want := 1.25 * (1./3 - .375 + .375*.375 + 1./65536)
		if err != nil || math.Abs(mass.Volume-want) > 1e-12 {
			t.Fatal("independent thin band volume", mass, want, err)
		}
	}
}

func TestOrderedRoofBoxNonbinaryTangentOrder(t *testing.T) {
	ctx := context.Background()
	// Heights 1+(3X-1)²/8 and 1 differ by a dyadic polynomial;
	// the original shared line is at nonbinary X=1/3.
	stock := orderedRoofParabolaSourceControls(t, [3]float64{1.125, .75, 1.5})
	flat, _ := makeOpenSplineBox(t, [3]float64{0, -.125, 0}, [3]float64{1, 1.125, 1}, 0)
	out, h, err := IntersectOrderedRoofBoxes(ctx, stock, flat, MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{})
	if err != nil || len(h.Faces) != 6 || h.Faces[0].Operand != 1 {
		t.Fatal("algebraic tangency must not change ceiling ownership", h, err)
	}
	mass, err := out.MassProperties(ctx, parametric.IntegrationOptions{})
	if err != nil || math.Abs(mass.Volume-1.25) > 1e-12 {
		t.Fatal("expected exact flat common", mass, err)
	}
	if out, h, err := SubtractOrderedRoofBoxes(ctx, stock, flat, MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{}); err == nil || out.Topology().Faces != 0 || len(h.Faces) != 0 {
		t.Fatal("even algebraic root must not certify strict height separation", out.Topology(), h, err)
	}
}

func TestOrderedRoofBoxCommonImprintsIsoCrossings(t *testing.T) {
	ctx := context.Background()
	// h=1+(X-1/2)^2 crosses h=17/16 at the exact original stations
	// X=1/4 and X=3/4. Both separations are proved before sewing.
	curved := orderedRoofTangentSource(t)
	upper, _ := makeOpenSplineBox(t, [3]float64{0, -.125, 0}, [3]float64{1, 1.125, 17. / 16}, 0)
	var ref struct {
		Volume, Area  float64
		Centroid      [3]float64
		Solids, Euler int
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_ORDERED_ROOF_COMMON"); oracle != "" {
		data, err := exec.CommandContext(ctx, oracle, "iso-crossing-ordered-common").Output()
		if err != nil || json.Unmarshal(data, &ref) != nil || ref.Solids != 1 || ref.Euler != 2 {
			t.Fatal("independent OCCT crossing roofs", ref, err, string(data))
		}
	}
	for i, pair := range [][2]Solid{{curved, upper}, {upper, curved}} {
		s, h, err := IntersectOrderedRoofBoxes(ctx, pair[0], pair[1], MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{})
		if err != nil || len(h.Faces) != 14 || s.Topology().EulerCharacteristic != 2 || len(s.Topology().Shells) != 1 {
			t.Fatal("two crossings must form one closed original boundary", s.Topology(), h, err)
		}
		for j, resultFace := range []int{0, 5, 9} {
			owner := 1 - i
			if j == 1 {
				owner = i
			}
			if h.Faces[resultFace].Operand != owner {
				t.Fatal("wrong roof owner in a cell", j, h.Faces)
			}
		}
		mass, err := s.MassProperties(ctx, parametric.IntegrationOptions{})
		// Integrating the two outer flat strips and the middle parabola
		// gives 25/24 per unit width.
		if err != nil || math.Abs(mass.Volume-125./96) > 1e-12 {
			t.Fatal("independent three-strip volume", mass, err)
		}
		if ref.Solids != 0 && (math.Abs(mass.Volume-ref.Volume) > 1e-8 || math.Abs(mass.Area-ref.Area) > 1e-7 || mass.Centroid.Sub(geom.Vec3{X: ref.Centroid[0], Y: ref.Centroid[1], Z: ref.Centroid[2]}).Length() > 1e-8) {
			t.Fatal("imprinted common disagrees with pinned OCCT", mass, ref)
		}
	}
	// 1+(3X-1)/8 changes owner at X=1/3, outside the native binary
	// cut representation. Neither the input nor the junction is rounded.
	line := orderedRoofParabolaSourceControls(t, [3]float64{.875, 1.0625, 1.25})
	flat, _ := makeOpenSplineBox(t, [3]float64{0, -.125, 0}, [3]float64{1, 1.125, 1}, 0)
	if s, h, err := IntersectOrderedRoofBoxes(ctx, line, flat, MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{}); !errors.Is(err, geom.ErrUnrepresentable) || s.Topology().Faces != 0 || len(h.Faces) != 0 {
		t.Fatal("a nonrepresentable crossing must fail atomically", s.Topology(), h, err)
	}
}

func TestOrderedRoofBoxCommonNativeTangent(t *testing.T) {
	ctx := context.Background()
	curved := orderedRoofTangentSource(t)
	plane, _ := makeOpenSplineBox(t, [3]float64{}, [3]float64{1, 1, 1}, 0)
	s, h, err := IntersectOrderedRoofBoxes(ctx, curved, plane, MixedRoofQueryOptions{}, geom.SurfacePatchEvaluationOptions{})
	if err != nil || len(h.Faces) != 6 || h.Faces[0].Operand != 1 {
		t.Fatal("even-multiplicity roof contact creates no material seam", h, err)
	}
	mass, err := s.MassProperties(ctx, parametric.IntegrationOptions{})
	if err != nil || math.Abs(mass.Volume-1) > 1e-12 || math.Abs(mass.Area-6) > 1e-12 {
		t.Fatal("analytic tangent Common is the unit box", mass, err)
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_ORDERED_ROOF_COMMON"); oracle != "" {
		data, err := exec.CommandContext(ctx, oracle, "tangent-ordered-common").Output()
		var ref struct {
			Volume, Area  float64
			Solids, Euler int
		}
		if err != nil || json.Unmarshal(data, &ref) != nil || ref.Solids != 1 || ref.Euler != 2 || math.Abs(ref.Volume-1) > 1e-8 || math.Abs(ref.Area-6) > 1e-8 {
			t.Fatal("independent regularized tangency", ref, err, string(data))
		}
	}
}
