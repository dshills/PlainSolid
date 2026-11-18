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

func clippedTransverseStock(t *testing.T, axis, layout int, original *geom.BSplineSurface) [3]*geom.BSplineSurface {
	t.Helper()
	base, _, b := rectangularPolynomialStock(t, axis, 0)
	p := independentProjectedEnvelopeRoof(t, original)
	d := p.Data()
	for i, q := range d.Poles {
		d.Poles[i] = geom.Vec3{X: 2 + 2*q.X, Y: -1 + 3*q.Y, Z: q.Z}
	}
	a, e := geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
	if e != nil {
		t.Fatal(e)
	}
	var aClip [2]geom.ParameterRectangle
	var bClip geom.ParameterRectangle
	if layout == 0 {
		aClip = [2]geom.ParameterRectangle{
			{UMin: -2, UMax: 1.5, VMin: 8, VMax: 10},
			{UMin: -2, UMax: 2, VMin: 8.25, VMax: 10},
		}
		bClip = geom.ParameterRectangle{UMin: 2, UMax: 4.625, VMin: -1, VMax: 1}
	} else {
		aClip = [2]geom.ParameterRectangle{
			{UMin: -2, UMax: 2, VMin: 8.25, VMax: 10},
			{UMin: -2, UMax: 1.5, VMin: 8, VMax: 10},
		}
		bClip = geom.ParameterRectangle{UMin: 2, UMax: 5, VMin: -.75, VMax: 1}
	}
	return [3]*geom.BSplineSurface{base, restrictOriginalClipStock(t, a, aClip[axis]), restrictOriginalClipStock(t, b, bClip)}
}

func TestNativeClippedPolynomialTransverseCut(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	o := PolynomialNodeCutOptions{}
	for axis := range 2 {
		_, a, _ := nativeWideTwoPolynomialNodeFixture(t, axis)
		for layout := range 2 {
			s := clippedTransverseStock(t, axis, layout, a)
			for order := range 2 {
				r, e := CutClippedPolynomialComponents(ctx, s[0], s[1+order], s[2-order], axis, 1e-9, o)
				if layout == 0 && order == 0 {
					if r != nil || !errors.Is(e, ErrClippedPolynomialCutSelfContact) {
						t.Fatal("a global strip join leaves two local links at the simple crossing", axis, e)
					}
					continue
				}
				if e != nil {
					t.Fatal(axis, layout, order, e)
				}
				checkClippedTransverseCutVolumes(t, r, axis, layout, order)
				checkClippedTransverseCutBinding(t, r, axis, layout, order)
				var raw bytes.Buffer
				if e := r.WriteJSON(ctx, &raw); e != nil {
					t.Fatal(e)
				}
				reloaded, e := ReadClippedPolynomialCutJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, o)
				if e != nil || !reflect.DeepEqual(r.shells, reloaded.shells) || !reflect.DeepEqual(r.Contacts(), reloaded.Contacts()) {
					t.Fatal("transverse roots, component volume and original coedges re-prove on reload", axis, layout, order, e)
				}
				for i, source := range r.native.sources {
					if !geom.SameSplineSurface(source, reloaded.native.sources[i]) {
						t.Fatal("unmodified source charts survive reload", i)
					}
				}
				var echo bytes.Buffer
				if e := reloaded.WriteJSON(ctx, &echo); e != nil || !bytes.Equal(raw.Bytes(), echo.Bytes()) {
					t.Fatal("stable clipped Cut encoding", e)
				}
			}
		}
	}
}

func checkClippedTransverseCutVolumes(t *testing.T, r *ClippedPolynomialCut, axis, layout, order int) {
	t.Helper()
	// Independently integrate ±(105/128)(v-f)(v-g) on the original
	// overlap using v³/3-(f+g)v²/2+fgv. For the between-branch
	// chambers use (105/768)|g-f|³ and the exact original gap
	// (4u-1)(4u-3)(1+u²)/32. Add each positive exclusive strip
	// from its original roof minus the sloping base. Physical XY
	// contributes a factor of six throughout.
	partial := []string{"116671899511/80501843339247616", "3536369/307090161664",
		[2]string{"154213871378417875/80501843339247616", "155864788087523539/80501843339247616"}[axis]}
	dependentA := []string{[2]string{"59387915123/27917287424", "59960437619/27917287424"}[axis], "6619420531/27917287424"}
	dependentB := []string{"688490045/19653770346496", "3536369/307090161664", "4818269699/19653770346496",
		[2]string{"15861/8192", "15693/8192"}[axis]}
	want := partial
	if layout == 1 {
		want = [2][]string{dependentA, dependentB}[order]
	}
	volumes, total := r.ExactComponentVolumes()
	if len(volumes) != len(want) || len(volumes) != len(r.Components()) {
		t.Fatal("all positive components have their own exact volume", volumes, want)
	}
	sum := new(big.Rat)
	for i, q := range volumes {
		if q.Sign() <= 0 || q.RatString() != want[i] {
			t.Fatal("independent exact component volume", axis, layout, order, i, q, want[i])
		}
		sum.Add(sum, q)
		value, _ := q.Float64()
		p, e := r.Components()[i].MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-9})
		if e != nil || math.Abs(p.Volume-value) > 1e-8 {
			t.Fatal("native boundary matches the independently integrated volume", axis, layout, order, i, p.Volume, value, e)
		}
	}
	if sum.Cmp(total) != 0 {
		t.Fatal("exact total stock material", sum, total)
	}
}

func checkClippedTransverseCutBinding(t *testing.T, r *ClippedPolynomialCut, axis, layout, order int) {
	t.Helper()
	station := [2][2]*big.Rat{{big.NewRat(5, 2), big.NewRat(7, 2)}, {big.NewRat(-1, 4), big.NewRat(5, 4)}}[axis]
	derivative := [2][2]*big.Rat{{big.NewRat(-51, 128), big.NewRat(75, 128)}, {big.NewRat(-17, 96), big.NewRat(25, 96)}}[axis]
	for i, c := range r.Contacts() {
		pair := [2]int{0, 1}
		if order == 1 {
			pair = [2]int{i, i + 1}
		}
		if c.Order != 1 || c.DerivativeGap.Cmp(derivative[i]) != 0 || c.Point[axis].Cmp(station[i]) != 0 || c.Components != pair {
			t.Fatal("physical transverse roots, jets and separate local links", axis, layout, order, i, c)
		}
	}
	var steps []ClippedPolynomialWallBinding
	for i, solid := range r.Components() {
		top := solid.Topology()
		if len(top.Shells) != 1 || top.EulerCharacteristic != 2 || top.CollapsedEdges != 0 {
			t.Fatal("one closed positive native shell per component", i, top)
		}
		for j, ch := range r.Chambers(i) {
			floor := r.native.sources[2]
			if ch.Tile != 0 {
				floor = r.native.sources[0]
			}
			if solid.faces[2*j].Surface != r.native.sources[1] || solid.faces[2*j+1].Surface != floor {
				t.Fatal("original upper and lower cap supports", i, ch)
			}
		}
		for _, wall := range r.Walls(i) {
			for j, owner := range [2]geom.ClippedPolynomialHeightOwner{wall.Low, wall.High} {
				fi := 0
				if j == 1 {
					fi += 1 + len(wall.Stations[1])
				}
				use := solid.faces[wall.Face].Outer[fi]
				trim, ok := graphWallSourceTrim(use.Curve, j == 1, true)
				if !ok || use.Edge != wall.TraceEdges[j] || trim.surface != r.native.sources[owner-1] ||
					trim.a != wall.OwnerUV[j][0] || trim.b != wall.OwnerUV[j][1] {
					t.Fatal("exact original stock-side coedges", i, j, wall)
				}
			}
			if wall.Low == geom.ClippedPolynomialBaseHeight && wall.High == geom.ClippedPolynomialRoofBHeight {
				if wall.EndingRoofs != 2 {
					t.Fatal("the cutter's original stock side owns the floor step", wall)
				}
				fixedAxis, fixed := axis, [2]float64{3.75, 1.625}[axis]
				if layout == 1 {
					fixedAxis, fixed = 1-axis, [2]float64{-.625, 2.25}[axis]
				}
				for _, p := range wall.Points {
					if [2]float64{p.X, p.Y}[fixedAxis] != fixed {
						t.Fatal("physical floor step lies on the exact cutter boundary", fixed, wall)
					}
				}
				steps = append(steps, wall)
			}
		}
	}
	wantSteps := 2
	if layout == 1 {
		wantSteps = 1 - order
	}
	if len(steps) != wantSteps {
		t.Fatal("exposed floor steps over the original strip adjacency", layout, order, steps)
	}
	if len(steps) == 2 {
		shared := false
		for i, p := range steps[0].Points {
			for j, q := range steps[1].Points {
				a, b := steps[0].Stations[i], steps[1].Stations[j]
				shared = shared || (p == q && len(a) == 1 && len(b) == 1 && a[0] == b[0])
			}
		}
		if steps[0].Atom != steps[1].Atom || !shared {
			t.Fatal("the two floor walls share the exact midpoint station", steps)
		}
	}
}

func TestNativeClippedPolynomialTransverseCutRefusals(t *testing.T) {
	ctx := context.Background()
	o := PolynomialNodeCutOptions{}
	_, a, _ := nativeWideTwoPolynomialNodeFixture(t, 0)
	s := clippedTransverseStock(t, 0, 0, a)
	r, e := CutClippedPolynomialComponents(ctx, s[0], s[2], s[1], 0, 1e-9, o)
	if e != nil {
		t.Fatal(e)
	}
	var canonical bytes.Buffer
	if e := r.WriteJSON(ctx, &canonical); e != nil {
		t.Fatal(e)
	}
	var stale clippedPolynomialCutRecipe
	if e := json.Unmarshal(canonical.Bytes(), &stale); e != nil {
		t.Fatal(e)
	}
	stale.Orders[0] = 3
	var changed bytes.Buffer
	if e := writePolynomialGraphRecipe(ctx, &changed, stale); e != nil {
		t.Fatal(e)
	}
	if out, e := ReadClippedPolynomialCutJSON(ctx, &changed, JSONLimits{}, o); out != nil || !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("contact multiplicity must be re-proved on original sources", e)
	}

	_, a, _ = nativeTwoTangenciesFixture(t, 0, 0)
	s = clippedTransverseStock(t, 0, 0, a)
	if out, e := CutClippedPolynomialComponents(ctx, s[0], s[2], s[1], 0, 1e-9, o); out != nil || !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("the bounded native profile rejects two tangencies", e)
	}
	_, a, _ = nativeTwoPolynomialNodeFixture(t, 0, 3)
	s = clippedTransverseStock(t, 0, 0, a)
	if out, e := CutClippedPolynomialComponents(ctx, s[0], s[2], s[1], 0, 1e-9, o); out != nil || !errors.Is(e, geom.ErrUnrepresentable) {
		t.Fatal("nonbinary original node cannot round into a native shell", e)
	}
}
