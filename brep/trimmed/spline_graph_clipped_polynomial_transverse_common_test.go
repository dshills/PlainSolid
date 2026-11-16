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

func TestNativeClippedPolynomialTransverseCommon(t *testing.T) {
	ctx := context.Background()
	o := PolynomialCubicCrossingTangencyEnvelopeOptions{}
	for axis := range 2 {
		_, a, _ := nativeWideTwoPolynomialNodeFixture(t, axis)
		for layout := range 3 {
			s := clippedTransverseFuseStock(t, axis, layout, a)
			for order := range 2 {
				r, e := CommonClippedPolynomialEnvelope(ctx, s[0], s[1+order], s[2-order], axis, 1e-9, o)
				if e != nil {
					t.Fatal(axis, layout, order, e)
				}
				checkClippedTransverseCommonMeasures(t, r, layout, axis)
				checkClippedTransverseCommonOwners(t, r, layout, axis, order)
				var raw bytes.Buffer
				if e := r.WriteJSON(ctx, &raw); e != nil {
					t.Fatal(e)
				}
				reloaded, e := ReadClippedPolynomialCommonJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, o)
				if e != nil || r.History() != reloaded.History() || !reflect.DeepEqual(r.Contacts(), reloaded.Contacts()) ||
					*r.clippedFootprint != *reloaded.clippedFootprint {
					t.Fatal("complete original chart, wall and junction replay", layout, axis, order, e)
				}
				for i, source := range r.sources {
					if !geom.SameSplineSurface(source, reloaded.sources[i]) {
						t.Fatal("replay preserves the original unmodified support and domain", i)
					}
				}
				var echo bytes.Buffer
				if e := reloaded.WriteJSON(ctx, &echo); e != nil || !bytes.Equal(raw.Bytes(), echo.Bytes()) {
					t.Fatal("canonical clipped Common source recipe", e)
				}
			}
		}
	}
}

func checkClippedTransverseCommonMeasures(t *testing.T, r *AlignedPolynomialEnvelope, layout, axis int) {
	t.Helper()
	// Independently integrate B-base over the physical intersection, then
	// subtract (105/768)|f-g|³ integrated on the three free slabs. The XY
	// determinant is six and g-f=(4u-1)(4u-3)(1+u²)/32.
	volumes := [3]string{"481831030782470107/40250921669623808", "12853131393/1073741824", "361373072328796123/40250921669623808"}
	exact, _ := new(big.Rat).SetString(volumes[layout])
	want, _ := exact.Float64()
	// Independent 32-point Gauss-Legendre integration of the selected
	// source roof derivatives, four side heights and the sloping base.
	areas := [3][2]float64{
		{32.985044942786089, 31.655019507695926},
		{31.652153672005817, 32.983255727388524},
		{26.733857637423235, 26.733913108776832},
	}
	ctx := context.Background()
	p, e := r.Boundary().MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
	if e != nil || math.Abs(p.Volume-want) > 1e-8 || math.Abs(p.Area-areas[layout][axis]) > 1e-8 {
		t.Fatal("native boundary matches independently integrated overlap material and area", layout, axis, p, want, areas[layout][axis], e)
	}
	var patches [3]*geom.SurfacePatch
	for i, source := range r.sources {
		patches[i], e = geom.NewSurfacePatch(source, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
	}
	rat, e := geom.IntegrateClippedSplineGraphPolynomialMaterial(ctx, patches[0], patches[1], patches[2], axis, geom.GraphPointContactOptions{})
	if e != nil || rat.Common.Cmp(exact) != 0 {
		t.Fatal("exact source integral matches independent result", rat.Common, exact, e)
	}
}

func checkClippedTransverseCommonOwners(t *testing.T, r *AlignedPolynomialEnvelope, layout, axis, order int) {
	t.Helper()
	s := r.Boundary()
	h := r.History()
	top := s.Topology()
	if len(top.Shells) != 1 || top.EulerCharacteristic != 2 || top.CollapsedEdges != 0 ||
		s.faces[h.Base].Surface != r.sources[0] {
		t.Fatal("one closed positive common shell on its original base", top)
	}
	for i, id := range h.RoofFaces {
		if s.faces[id].Surface != r.sources[1+h.RoofOwners[i]] {
			t.Fatal("selected original roof for each signed overlap region", i)
		}
	}
	derivative := [2][2]string{{"-51/128", "75/128"}, {"-17/96", "25/96"}}[axis]
	station := [2][2]string{{"5/2", "7/2"}, {"-1/4", "5/4"}}[axis]
	for i, c := range r.Contacts() {
		if c.Order != 1 || c.DerivativeGap.RatString() != derivative[i] || c.Point[axis].RatString() != station[i] {
			t.Fatal("exact original transverse station and first derivative", i, c)
		}
	}

	fp, ok := r.ClippedFootprint()
	if !ok {
		t.Fatal("missing original clipped footprint")
	}
	checkClippedCommonWallOwners(t, s, h.Base, r.sources, fp, layout, axis, order)
}

func checkClippedCommonWallOwners(t *testing.T, s Solid, base int, sources [3]*geom.BSplineSurface,
	fp ClippedPolynomialFootprint, layout, axis, order int) {
	t.Helper()
	ends := [3][2][4]uint8{
		{{3, 1, 3, 2}, {2, 3, 1, 3}},
		{{2, 3, 1, 3}, {3, 1, 3, 2}},
		{{2, 1, 1, 2}, {2, 1, 1, 2}},
	}[layout][axis]
	if order == 1 {
		for i, mask := range ends {
			ends[i] = (mask&1)<<1 | (mask&2)>>1
		}
	}
	if fp.BoundaryRoofs != ends {
		t.Fatal("exact ownership of all original clipped footprint sides", fp, ends)
	}
	var otherStockWall bool
	for i, f := range s.faces[base+1:] {
		if len(f.Outer) != 4 {
			t.Fatal("one upper trace, lower trace and two uprights per common wall", i)
		}
		low, loOK := graphWallSourceTrim(f.Outer[0].Curve, false, true)
		high, hiOK := graphWallSourceTrim(f.Outer[2].Curve, true, true)
		if !loOK || !hiOK || low.surface != sources[0] {
			t.Fatal("external wall keeps the original floor and roof curves", i)
		}
		owner := 0
		if high.surface != sources[1] {
			owner = 1
			if high.surface != sources[2] {
				t.Fatal("upper wall trace lost its original roof", i)
			}
		}
		bottom, ok := s.edges[f.Outer[0].Edge].Curve.(surfaceTrimCurve)
		if !ok || bottom.surface != low.surface || bottom.a != low.a || bottom.b != low.b || bottom.curve != nil {
			t.Fatal("wall lower edge keeps the original base UV", i)
		}
		edge := s.edges[f.Outer[2].Edge]
		model, ok := edge.Curve.(surfaceTrimCurve)
		if !ok || model.surface != high.surface || model.a != high.a || model.b != high.b || model.curve != nil {
			t.Fatal("wall uses the selected native roof edge with its original UV", i)
		}
		p, q := s.vertices[edge.Start], s.vertices[edge.End]
		d := fp.Physical
		side := -1
		switch {
		case p.Y == d.VMin && q.Y == d.VMin:
			side = 0
		case p.X == d.UMax && q.X == d.UMax:
			side = 1
		case p.Y == d.VMax && q.Y == d.VMax:
			side = 2
		case p.X == d.UMin && q.X == d.UMin:
			side = 3
		}
		if side < 0 {
			t.Fatal("wall lies on the exact original clipped footprint", i, p, q)
		}
		otherStockWall = otherStockWall || ends[side]&(1<<owner) == 0
	}
	if !otherStockWall {
		t.Fatal("a shared wall must retain its roof trace even when the other stock ends there")
	}
}

func TestNativeClippedPolynomialTransverseCommonRefusals(t *testing.T) {
	ctx := context.Background()
	o := PolynomialCubicCrossingTangencyEnvelopeOptions{}
	_, a, _ := nativeWideTwoPolynomialNodeFixture(t, 0)
	s := clippedTransverseFuseStock(t, 0, 0, a)
	r, e := CommonClippedPolynomialEnvelope(ctx, s[0], s[1], s[2], 0, 1e-9, o)
	if e != nil {
		t.Fatal(e)
	}
	var raw bytes.Buffer
	if e := r.WriteJSON(ctx, &raw); e != nil {
		t.Fatal(e)
	}
	for _, edit := range []func(*clippedPolynomialCommonRecipe){
		func(r *clippedPolynomialCommonRecipe) { r.Ledger.Contacts[0].Order = 2 },
		func(r *clippedPolynomialCommonRecipe) { r.Footprint.BoundaryRoofs[1] = 3 },
	} {
		var stale clippedPolynomialCommonRecipe
		if e := json.Unmarshal(raw.Bytes(), &stale); e != nil {
			t.Fatal(e)
		}
		edit(&stale)
		var changed bytes.Buffer
		if e := writePolynomialGraphRecipe(ctx, &changed, stale); e != nil {
			t.Fatal(e)
		}
		if out, e := ReadClippedPolynomialCommonJSON(ctx, &changed, JSONLimits{}, o); out != nil || !errors.Is(e, geom.ErrConvergence) {
			t.Fatal("original crossing and exact stock-side ownership must re-prove", e)
		}
	}
	_, a, _ = nativeTwoTangenciesFixture(t, 0, 0)
	s = clippedTransverseFuseStock(t, 0, 0, a)
	if out, e := CommonClippedPolynomialEnvelope(ctx, s[0], s[1], s[2], 0, 1e-9, o); out != nil || !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("the clipped Common path is still bounded by exact profile", e)
	}
	_, a, _ = nativeTwoPolynomialNodeFixture(t, 0, 3)
	s = clippedTransverseFuseStock(t, 0, 0, a)
	if out, e := CommonClippedPolynomialEnvelope(ctx, s[0], s[1], s[2], 0, 1e-9, o); out != nil || !errors.Is(e, geom.ErrUnrepresentable) {
		t.Fatal("an unrepresentable node cannot enter native Common", e)
	}
}
