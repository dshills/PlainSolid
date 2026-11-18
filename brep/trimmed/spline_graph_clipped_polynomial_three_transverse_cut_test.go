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

func TestNativeClippedPolynomialThreeTransverseCut(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	o := PolynomialNodeCutOptions{}
	for axis := range 2 {
		_, roof, _ := nativeThreePolynomialNodeFixture(t, axis, 4, 128)
		for layout := range 3 {
			src := clippedTransverseFuseStock(t, axis, layout, roof)
			for order := range 2 {
				r, e := CutClippedPolynomialThreeTransverseComponents(ctx, src[0], src[1+order], src[2-order], axis, 1e-9, o)
				if order == 0 && layout != 1 {
					if r != nil || !errors.Is(e, ErrClippedPolynomialCutSelfContact) || !errors.Is(e, geom.ErrConvergence) {
						t.Fatal("proved intra-component links must refuse", axis, layout, e)
					}
					continue
				}
				if e != nil {
					t.Fatal(axis, layout, order, e)
				}
				checkNativeThreeTransverseClippedCut(t, r, layout, axis, order)
				var raw, echo bytes.Buffer
				if e := r.WriteJSON(ctx, &raw); e != nil {
					t.Fatal(e)
				}
				result, e := ReadClippedPolynomialThreeTransverseCutJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, o)
				if e != nil || !reflect.DeepEqual(r.cut.shells, result.cut.shells) || !reflect.DeepEqual(r.Contacts(), result.Contacts()) {
					t.Fatal("complete component and contact source replay", axis, layout, order, e)
				}
				for i, source := range r.sources {
					if !geom.SameSplineSurface(source, result.sources[i]) {
						t.Fatal("original physical stock or parameter clock changed in replay", i)
					}
				}
				if e := result.WriteJSON(ctx, &echo); e != nil || !bytes.Equal(raw.Bytes(), echo.Bytes()) {
					t.Fatal("canonical three-junction Cut source recipe", e)
				}
			}
		}
	}
}

func checkNativeThreeTransverseClippedCut(t *testing.T, r *ClippedPolynomialThreeTransverseCut, layout, axis, order int) {
	t.Helper()
	a, b := threeTransverseClippedComponentVolumes(layout, axis)
	want := [2][]string{a, b}[order]
	parts := r.Components()
	vols, total := r.ExactComponentVolumes()
	if len(parts) != len(want) || len(vols) != len(want) {
		t.Fatal("all positive original component volumes are returned", len(parts), vols, want)
	}
	sum := new(big.Rat)
	for i, part := range parts {
		volume := vols[i]
		if volume.Sign() <= 0 || volume.RatString() != want[i] {
			t.Fatal("independent exact physical per-shell volume", layout, axis, order, i, volume, want[i])
		}
		sum.Add(sum, volume)
		checkNativeThreeTransverseClippedCutOwner(t, r, part, i)
		if volume.Cmp(big.NewRat(1, 1)) > 0 {
			v, _ := volume.Float64()
			measured, e := part.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-9})
			if e != nil || math.Abs(measured.Volume-v) > 1e-8 {
				t.Fatal("native strip-bearing material matches the original exact integrals", i, measured.Volume, v, e)
			}
		}
	}
	if sum.Cmp(total) != 0 {
		t.Fatal("all independently integrated chambers contribute to the physical total", sum, total)
	}
	var stripComponent, chambers int
	switch {
	case order == 0:
		stripComponent, chambers = 0, 2
	case layout == 1:
		stripComponent, chambers = 4, 1
	case layout == 0:
		stripComponent, chambers = 3, 2
	default:
		stripComponent, chambers = 3, 3
	}
	group := r.Chambers(stripComponent)
	if len(group) != chambers || group[len(group)-1].Tile == 0 || group[len(group)-1].Region != -1 {
		t.Fatal("the proved original strip is attached or separate as required", layout, axis, order, group)
	}
	contacts := r.Contacts()
	if len(contacts) != 3 {
		t.Fatal("all three exact original contacts are returned", contacts)
	}
	station := [2][3]string{{"5/2", "3", "7/2"}, {"-1/4", "1/2", "5/4"}}[axis]
	derivative := [2][3]string{{"-15/256", "9/256", "-21/256"}, {"-5/192", "1/64", "-7/192"}}[axis]
	for i, c := range contacts {
		pair := [2]int{0, 1}
		if order == 1 {
			pair = [2]int{i, i + 1}
		}
		if c.Order != 1 || c.Components != pair || c.Point[axis].RatString() != station[i] ||
			c.DerivativeGap.RatString() != derivative[i] {
			t.Fatal("complete original transverse geometry and component contact graph", i, c)
		}
		for k, part := range c.Components {
			p := parts[part].vertices[c.Vertices[k]]
			if c.Point[0].Cmp(ratInset(p.X)) != 0 || c.Point[1].Cmp(ratInset(p.Y)) != 0 || c.Point[2].Cmp(ratInset(p.Z)) != 0 {
				t.Fatal("each incident shell has its own exact native contact vertex", i, part, p)
			}
		}
	}
	contacts[2].DerivativeGap.SetInt64(0)
	contacts[2].Point[axis].SetInt64(0)
	if r.Contacts()[2].Point[axis].RatString() != station[2] || r.Contacts()[2].DerivativeGap.RatString() != derivative[2] {
		t.Fatal("the complete returned contact slice has caller-owned fields")
	}
}

func checkNativeThreeTransverseClippedCutOwner(t *testing.T, r *ClippedPolynomialThreeTransverseCut, part Solid, component int) {
	t.Helper()
	top := part.Topology()
	if len(top.Shells) != 1 || top.EulerCharacteristic != 2 || top.CollapsedEdges != 0 {
		t.Fatal("every native component is an embedded outward shell", component, top)
	}
	for j, ch := range r.Chambers(component) {
		floor := r.sources[2]
		if ch.Tile > 0 {
			floor = r.sources[0]
		}
		if part.faces[2*j].Surface != r.sources[1] || part.faces[2*j+1].Surface != floor {
			t.Fatal("original roof and floor of every positive chamber", component, j, ch)
		}
	}
	for _, wall := range r.Walls(component) {
		for i, owner := range [2]geom.ClippedPolynomialHeightOwner{wall.Low, wall.High} {
			k := 0
			if i == 1 {
				k = 1 + len(wall.Stations[1])
			}
			u := part.faces[wall.Face].Outer[k]
			p, ok := graphWallSourceTrim(u.Curve, i == 1, true)
			if !ok || p.surface != r.sources[owner-1] || p.a != wall.OwnerUV[i][0] || p.b != wall.OwnerUV[i][1] || u.Edge != wall.TraceEdges[i] {
				t.Fatal("strict original floor and roof trim at every exposed wall", component, i, wall)
			}
		}
	}
}

func TestNativeClippedPolynomialThreeTransverseCutRefusals(t *testing.T) {
	ctx := context.Background()
	o := PolynomialNodeCutOptions{}
	_, a, _ := nativeThreePolynomialNodeFixture(t, 0, 4, 128)
	src := clippedTransverseFuseStock(t, 0, 0, a)
	r, e := CutClippedPolynomialThreeTransverseComponents(ctx, src[0], src[2], src[1], 0, 1e-9, o)
	if e != nil {
		t.Fatal(e)
	}
	var raw bytes.Buffer
	if e := r.WriteJSON(ctx, &raw); e != nil {
		t.Fatal(e)
	}
	for _, edit := range []func(*clippedPolynomialCutRecipe){
		func(r *clippedPolynomialCutRecipe) { r.Contacts[2].Vertices[1]++ },
		func(r *clippedPolynomialCutRecipe) { r.Contacts[2].Components[0] = 0 },
		func(r *clippedPolynomialCutRecipe) { r.DerivativeGaps[2] = "0" },
		func(r *clippedPolynomialCutRecipe) { r.Shells[len(r.Shells)-1].Volume = "1" },
	} {
		var stale clippedPolynomialCutRecipe
		if e := json.Unmarshal(raw.Bytes(), &stale); e != nil {
			t.Fatal(e)
		}
		edit(&stale)
		var changed bytes.Buffer
		if e := writePolynomialGraphRecipe(ctx, &changed, stale); e != nil {
			t.Fatal(e)
		}
		if out, e := ReadClippedPolynomialThreeTransverseCutJSON(ctx, &changed, JSONLimits{}, o); out != nil || !errors.Is(e, geom.ErrConvergence) {
			t.Fatal("source replay requires all exact component and contact fields", e)
		}
	}
	if out, e := ReadClippedPolynomialCutJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, o); out != nil || !errors.Is(e, geom.ErrInvalid) {
		t.Fatal("the legacy two-contact reader retains its own format", e)
	}

	_, a, _ = nativeThreePolynomialNodeFixture(t, 0, 3, 128)
	src = clippedTransverseFuseStock(t, 0, 0, a)
	if out, e := CutClippedPolynomialThreeTransverseComponents(ctx, src[0], src[2], src[1], 0, 1e-9, o); out != nil || !errors.Is(e, geom.ErrUnrepresentable) {
		t.Fatal("an inexact native vertex cannot enter a three-contact Cut shell", e)
	}
}
