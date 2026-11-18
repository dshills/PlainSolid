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

func clippedTransverseFuseStock(t *testing.T, axis, layout int, original *geom.BSplineSurface) [3]*geom.BSplineSurface {
	t.Helper()
	if layout < 2 {
		return clippedTransverseStock(t, axis, layout, original)
	}
	s := clippedTransverseStock(t, axis, 0, original)
	s[1] = restrictOriginalClipStock(t, s[1], geom.ParameterRectangle{UMin: -2, UMax: 1.5, VMin: 8.25, VMax: 10})
	s[2] = restrictOriginalClipStock(t, s[2], geom.ParameterRectangle{UMin: 2, UMax: 4.625, VMin: -.75, VMax: 1})
	return s
}

func TestNativeClippedPolynomialTransverseFuse(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	o := PolynomialCubicCrossingTangencyEnvelopeOptions{}
	for axis := range 2 {
		_, a, _ := nativeWideTwoPolynomialNodeFixture(t, axis)
		for layout := range 3 {
			src := clippedTransverseFuseStock(t, axis, layout, a)
			for order := range 2 {
				r, e := FuseClippedPolynomialEnvelope(ctx, src[0], src[1+order], src[2-order], axis, 1e-9, o)
				if e != nil {
					t.Fatal(axis, layout, order, e)
				}
				checkClippedTransverseFuseMeasures(t, r, layout, axis)
				checkClippedTransverseFuseOwners(t, r, layout, axis)
				if layout == 2 {
					checkClippedFuseTwoAxisBase(t, r, 0)
					checkClippedFuseCompoundCorners(t, r, 0, axis, order)
				}
				var raw bytes.Buffer
				if e := r.WriteJSON(ctx, &raw); e != nil {
					t.Fatal(e)
				}
				reloaded, e := ReadClippedPolynomialFuseJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, o)
				if e != nil || !reflect.DeepEqual(r.ledger, reloaded.ledger) || !reflect.DeepEqual(r.Walls(), reloaded.Walls()) ||
					r.ExactVolume().Cmp(reloaded.ExactVolume()) != 0 {
					t.Fatal("original crossing, exposed wall and source-volume reconstruction", axis, layout, order, e)
				}
				for i, source := range r.sources {
					if !geom.SameSplineSurface(source, reloaded.sources[i]) {
						t.Fatal("source replay preserves the original charts and limits", i)
					}
				}
				var echo bytes.Buffer
				if e := reloaded.WriteJSON(ctx, &echo); e != nil || !bytes.Equal(raw.Bytes(), echo.Bytes()) {
					t.Fatal("canonical clipped Fuse encoding", e)
				}
			}
		}
	}
}

func checkClippedTransverseFuseMeasures(t *testing.T, r *ClippedPolynomialFuse, layout, axis int) {
	t.Helper()
	// Integrate both original stocks, then subtract their common
	// volume. Common is B-base on the overlap less the integral of
	// (105/768)|f-g|³ over the three free slabs. The original XY
	// determinant is six. The gap is (4u-1)(4u-3)(1+u²)/32.
	volumes := [3]string{"661394355266388005/40250921669623808", "17471121279/1073741824", "633568152956250917/40250921669623808"}
	want, _ := new(big.Rat).SetString(volumes[layout])
	if r.ExactVolume().Cmp(want) != 0 {
		t.Fatal("independent exact physical Fuse volume", layout, axis, r.ExactVolume(), want)
	}
	// Independently integrate the original roof derivatives and the
	// exposed height jump on each original footprint segment with
	// 32-point Gauss-Legendre quadrature, then add the sloping base.
	areas := [3][2]float64{
		{40.064876134731698, 40.387662713865431},
		{39.675781600888001, 39.970264207330395},
		{39.226816948128871, 39.472614926121949},
	}
	v, _ := want.Float64()
	p, e := r.Boundary().MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-9})
	if e != nil || math.Abs(p.Volume-v) > 1e-8 || math.Abs(p.Area-areas[layout][axis]) > 1e-8 {
		t.Fatal("native shell agrees with the independent material and area", layout, axis, p, v, areas[layout][axis], e)
	}
}

func checkClippedTransverseFuseOwners(t *testing.T, r *ClippedPolynomialFuse, layout, axis int) {
	t.Helper()
	s := r.Boundary()
	top := s.Topology()
	if len(top.Shells) != 1 || top.EulerCharacteristic != 2 || top.CollapsedEdges != 0 || s.faces[r.ledger.Base].Surface != r.sources[0] {
		t.Fatal("one closed source-backed union shell", top)
	}
	for i, id := range r.ledger.RoofFaces {
		if s.faces[id].Surface != r.sources[1+r.ledger.RoofOwners[i]] {
			t.Fatal("all upper caps use their selected original owner", i)
		}
	}
	derivative := [2][2]string{{"-51/128", "75/128"}, {"-17/96", "25/96"}}[axis]
	station := [2][2]string{{"5/2", "7/2"}, {"-1/4", "5/4"}}[axis]
	for i, c := range r.ledger.Contacts {
		if c.Order != 1 || c.DerivativeGap != derivative[i] || c.Point[axis] != station[i] ||
			len(c.Rays) != 4 || len(c.Sectors) != 4 {
			t.Fatal("exact transverse derivatives and four incident roof sectors", axis, i, c)
		}
	}
	riserAtoms := make(map[int]int)
	var risers, tapers int
	for _, wall := range r.Walls() {
		for j, owner := range [2]geom.ClippedPolynomialHeightOwner{wall.Low, wall.High} {
			face := s.faces[wall.Face]
			i := 0
			if j == 1 {
				i = 1 + len(wall.Stations[1])
			}
			use := face.Outer[i]
			trim, ok := graphWallSourceTrim(use.Curve, j == 1, true)
			if !ok || use.Edge != wall.TraceEdges[j] || trim.surface != r.sources[owner-1] ||
				trim.a != wall.OwnerUV[j][0] || trim.b != wall.OwnerUV[j][1] {
				t.Fatal("both wall traces retain their original surface clock", j, wall)
			}
		}
		if wall.Low == geom.ClippedPolynomialBaseHeight {
			continue
		}
		if wall.EndingRoofs != 1<<uint8(wall.High-geom.ClippedPolynomialRoofAHeight) {
			t.Fatal("the upper original stock side owns the exposed riser", wall)
		}
		risers++
		riserAtoms[wall.Atom]++
		for _, chain := range wall.Stations {
			if len(chain) == 0 {
				tapers++
			}
		}
		if layout == 1 {
			fixed := [2]float64{1.625, 3.75}[axis]
			for _, p := range wall.Points {
				if [2]float64{p.X, p.Y}[1-axis] != fixed {
					t.Fatal("the full riser follows the ending dependent footprint side", wall)
				}
			}
		}
	}
	if risers != [3]int{4, 1, 5}[layout] || len(riserAtoms) != [3]int{3, 1, 4}[layout] || tapers != [3]int{4, 0, 4}[layout] {
		t.Fatal("exact original riser and tapered endpoint incidence", layout, risers, riserAtoms, tapers)
	}
}

func TestNativeClippedPolynomialTransverseFuseRefusals(t *testing.T) {
	ctx := context.Background()
	o := PolynomialCubicCrossingTangencyEnvelopeOptions{}
	_, a, _ := nativeWideTwoPolynomialNodeFixture(t, 0)
	s := clippedTransverseFuseStock(t, 0, 0, a)
	r, e := FuseClippedPolynomialEnvelope(ctx, s[0], s[1], s[2], 0, 1e-9, o)
	if e != nil {
		t.Fatal(e)
	}
	var raw bytes.Buffer
	if e := r.WriteJSON(ctx, &raw); e != nil {
		t.Fatal(e)
	}
	for _, edit := range []func(*clippedPolynomialFuseRecipe){
		func(r *clippedPolynomialFuseRecipe) { r.Ledger.Contacts[0].DerivativeGap = "0" },
		func(r *clippedPolynomialFuseRecipe) { r.Walls[0].OwnerUV[0][0].Y += .125 },
	} {
		var stale clippedPolynomialFuseRecipe
		if e := json.Unmarshal(raw.Bytes(), &stale); e != nil {
			t.Fatal(e)
		}
		edit(&stale)
		var forged bytes.Buffer
		if e := writePolynomialGraphRecipe(ctx, &forged, stale); e != nil {
			t.Fatal(e)
		}
		if out, e := ReadClippedPolynomialFuseJSON(ctx, &forged, JSONLimits{}, o); out != nil || !errors.Is(e, geom.ErrConvergence) {
			t.Fatal("stale crossing or original wall traces must fail source replay", e)
		}
	}
	_, a, _ = nativeTwoTangenciesFixture(t, 0, 0)
	s = clippedTransverseFuseStock(t, 0, 0, a)
	if out, e := FuseClippedPolynomialEnvelope(ctx, s[0], s[1], s[2], 0, 1e-9, o); out != nil || !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("two tangent nodes remain outside this bounded native union", e)
	}
	_, a, _ = nativeTwoPolynomialNodeFixture(t, 0, 3)
	s = clippedTransverseFuseStock(t, 0, 0, a)
	if out, e := FuseClippedPolynomialEnvelope(ctx, s[0], s[1], s[2], 0, 1e-9, o); out != nil || !errors.Is(e, geom.ErrUnrepresentable) {
		t.Fatal("an unrepresentable original transverse node cannot round into the native union", e)
	}
}
