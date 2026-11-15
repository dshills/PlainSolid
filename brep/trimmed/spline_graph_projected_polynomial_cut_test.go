package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"plainsolid/geom"
)

func TestNativeProjectedPolynomialCut(t *testing.T) {
	ctx := context.Background()
	options := PolynomialNodeCutOptions{}
	for profile := 0; profile < 2; profile++ {
		for axis := 0; axis < 2; axis++ {
			base, physical, _ := nativeCubicCrossingTangencyFixture(t, axis, profile)
			a := independentProjectedEnvelopeRoof(t, physical)
			b := independentAlignedEnvelopeRoof(t, axis, 1, 25.0/8)
			for order, roofs := range [][2]*geom.BSplineSurface{{a, b}, {b, a}} {
				cut, e := CutProjectedPolynomialComponents(ctx, base, roofs[0], roofs[1], axis, 1e-9, options)
				if e != nil {
					t.Fatal("two independently projected original caps", profile, axis, order, e)
				}
				checkNativeProjectedCut(t, cut, profile, axis, order)
				var raw bytes.Buffer
				if e := cut.WriteJSON(ctx, &raw); e != nil {
					t.Fatal(e)
				}
				loaded, e := ReadProjectedPolynomialCutJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, options)
				if e != nil || !reflect.DeepEqual(loaded.Contacts(), cut.Contacts()) {
					t.Fatal("canonical indexed contacts survive source reload", profile, axis, order, e)
				}
				for i, source := range cut.sources {
					if !geom.SameSplineSurface(source, loaded.sources[i]) {
						t.Fatal("source reproof changes original roof", i)
					}
				}
				checkNativeProjectedCut(t, loaded, profile, axis, order)
				var replay bytes.Buffer
				if e := loaded.WriteJSON(ctx, &replay); e != nil || !bytes.Equal(raw.Bytes(), replay.Bytes()) {
					t.Fatal("canonical projected Cut recipe", e)
				}
				if profile == 0 && axis == 0 && order == 0 {
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

func checkNativeProjectedCut(t *testing.T, cut *AlignedPolynomialCut, profile, axis, order int) {
	t.Helper()
	checkNativePolynomialCutMaterial(t, cut, profile, axis, order)
	checkProjectedCutOwnerTrims(t, cut, axis, order, func(p geom.Vec3) geom.Vec3 { return p })
}

func checkProjectedCutOwnerTrims(t *testing.T, cut *AlignedPolynomialCut, axis, order int, toUnit func(geom.Vec3) geom.Vec3) {
	t.Helper()
	parts := cut.Components()
	for region, part := range parts {
		top := part.Topology()
		wantFaceCount := [2][]int{{5, 5}, {4, 2, 4}}[order][region]
		if len(top.Shells) != 1 || top.EulerCharacteristic != 2 || top.CollapsedEdges != 0 || len(part.faces) != wantFaceCount {
			t.Fatal("closed positive component topology", region, top, len(part.faces))
		}
		for i, face := range part.faces[:2] {
			physicalOwner := i ^ order
			negativeChart := physicalOwner == 0 || axis == 0
			if face.Surface != cut.sources[i+1] || face.Reversed != ((i == 1) != negativeChart) {
				t.Fatal("original cap support and physical orientation", region, i)
			}
			for _, use := range face.Outer {
				e := part.edges[use.Edge]
				if use.Reversed {
					e.Start, e.End = e.End, e.Start
				}
				a, b := part.vertices[e.Start], part.vertices[e.End]
				if use.UVStart != projectedFixtureUV(physicalOwner, axis, toUnit(a)) || use.UVEnd != projectedFixtureUV(physicalOwner, axis, toUnit(b)) {
					t.Fatal("original cap pcurve interval", region, i, use.Edge)
				}
			}
		}
		for _, face := range part.faces[2:] {
			var seen [2]bool
			for _, use := range face.Outer {
				for i := range seen {
					trim, ok := graphWallSourceTrim(use.Curve, i == 0, true)
					if !ok || trim.surface != cut.sources[i+1] {
						continue
					}
					e := part.edges[use.Edge]
					a, b := part.vertices[e.Start], part.vertices[e.End]
					if seen[i] || use.Reversed != (i == 0) || trim.a != projectedFixtureUV(i^order, axis, toUnit(a)) || trim.b != projectedFixtureUV(i^order, axis, toUnit(b)) {
						t.Fatal("Bound wall pcurve station and owner", region, i, use.Edge)
					}
					seen[i] = true
				}
			}
			if seen != [2]bool{true, true} {
				t.Fatal("wall bounded by both original supports", region, seen)
			}
		}
	}
}

func TestNativeProjectedPolynomialCutRejections(t *testing.T) {
	ctx := context.Background()
	o := PolynomialNodeCutOptions{}
	base, physical, _ := nativeCubicCrossingTangencyFixture(t, 0, 0)
	a := independentProjectedEnvelopeRoof(t, physical)
	b := independentAlignedEnvelopeRoof(t, 0, 1, 25.0/8)
	cut, e := CutProjectedPolynomialComponents(ctx, base, b, a, 0, 1e-9, o)
	if e != nil {
		t.Fatal(e)
	}
	var raw bytes.Buffer
	if e := cut.WriteJSON(ctx, &raw); e != nil {
		t.Fatal(e)
	}
	if c, e := ReadAlignedPolynomialCutJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, o); c != nil || !errors.Is(e, geom.ErrInvalid) {
		t.Fatal("existing aligned recipe remains separate", e)
	}
	var recipe polynomialMixedCutRecipe
	if e := json.Unmarshal(raw.Bytes(), &recipe); e != nil {
		t.Fatal(e)
	}
	recipe.Contacts[0].Vertices[0]++
	bad, e := json.Marshal(recipe)
	if e != nil {
		t.Fatal(e)
	}
	if r, e := ReadProjectedPolynomialCutJSON(ctx, bytes.NewReader(bad), JSONLimits{}, o); r != nil || !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("native local vertex identity must survive reproof", e)
	}
	if e := json.Unmarshal(raw.Bytes(), &recipe); e != nil {
		t.Fatal(e)
	}
	narrow := independentAlignedEnvelopeRoof(t, 0, .75, 25.0/8)
	recipe.Sources, e = encodePolynomialGraphSupports(ctx, [3]*geom.BSplineSurface{base, narrow, a})
	if e != nil {
		t.Fatal(e)
	}
	bad, e = json.Marshal(recipe)
	if e != nil {
		t.Fatal(e)
	}
	if r, e := ReadProjectedPolynomialCutJSON(ctx, bytes.NewReader(bad), JSONLimits{}, o); r != nil || !errors.Is(e, geom.ErrDomain) {
		t.Fatal("source ledger cannot conceal changed roof footprint", e)
	}
	if r, e := ReadProjectedPolynomialCutJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{MaxEntities: 1}, o); r != nil || !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("projected components respect the aggregate topology limit", e)
	}
	o.Proof.MaxOperations = 1
	if r, e := ReadProjectedPolynomialCutJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, o); r != nil || !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("reload must spend work on the original proof", e)
	}
}
