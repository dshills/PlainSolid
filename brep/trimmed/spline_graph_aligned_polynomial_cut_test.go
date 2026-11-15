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

	"plainsolid/geom"
)

// These values come from integrating (105/128)(v³/3-(f+g)v²/2+fgv)
// at the physical branch ordinates of the original quintic and cubic.
func independentAlignedCutVolumes(profile, order int) []*big.Rat {
	if order == 0 {
		return [2][]*big.Rat{
			{big.NewRat(28550399214829, 2515682604351488), big.NewRat(590037023506439, 7547047813054464)},
			{big.NewRat(86673486561287, 7547047813054464), big.NewRat(195276624577773, 2515682604351488)},
		}[profile]
	}
	v := []*big.Rat{big.NewRat(4909741, 2515682604351488), big.NewRat(1, 38386270208), big.NewRat(160842989, 2515682604351488)}
	if profile == 1 {
		v[0], v[2] = v[2], v[0]
	}
	return v
}

func TestNativeAlignedPolynomialCut(t *testing.T) {
	ctx := context.Background()
	for profile := 0; profile < 2; profile++ {
		for axis := 0; axis < 2; axis++ {
			base, a, _ := nativeCubicCrossingTangencyFixture(t, axis, profile)
			b := independentAlignedEnvelopeRoof(t, axis, 1, 25.0/8)
			for order, roofs := range [][2]*geom.BSplineSurface{{a, b}, {b, a}} {
				cut, e := CutAlignedPolynomialComponents(ctx, base, roofs[0], roofs[1], axis, 1e-9, PolynomialNodeCutOptions{})
				if e != nil {
					t.Fatal("native owner-aware Cut", profile, axis, order, e)
				}
				checkNativeAlignedCut(t, cut, profile, axis, order)
				var first bytes.Buffer
				if e := cut.WriteJSON(ctx, &first); e != nil {
					t.Fatal(e)
				}
				reloaded, e := ReadAlignedPolynomialCutJSON(ctx, bytes.NewReader(first.Bytes()), JSONLimits{}, PolynomialNodeCutOptions{})
				if e != nil || !reflect.DeepEqual(reloaded.Contacts(), cut.Contacts()) {
					t.Fatal("reloaded exact indexed contacts", profile, axis, order, e)
				}
				checkNativeAlignedCut(t, reloaded, profile, axis, order)
				if profile == 0 && axis == 0 && order == 0 {
					var points [2]geom.Vec3
					for i, c := range reloaded.Contacts() {
						points[i], e = geom.RepresentedExactPoint(c.Point)
						if e != nil {
							t.Fatal(e)
						}
					}
					checkPolynomialNodeCutStrictCollision(t, [2]Solid(reloaded.Components()), points[:]...)
				}
				for i, source := range cut.sources {
					if !geom.SameSplineSurface(source, reloaded.sources[i]) {
						t.Fatal("source reproof altered an original chart", i)
					}
				}
				var second bytes.Buffer
				if e := reloaded.WriteJSON(ctx, &second); e != nil || !bytes.Equal(first.Bytes(), second.Bytes()) {
					t.Fatal("canonical aligned Cut source recipe", e)
				}
			}
		}
	}
}

func checkNativeAlignedCut(t *testing.T, cut *AlignedPolynomialCut, profile, axis, order int) {
	t.Helper()
	checkNativePolynomialCutMaterial(t, cut, profile, axis, order)
	parts := cut.Components()

	for i, part := range parts {
		topology := part.Topology()
		if len(topology.Shells) != 1 || topology.EulerCharacteristic != 2 || topology.CollapsedEdges != 0 {
			t.Fatal("separate embedded component topology", i, topology)
		}
		faces := part.faces
		wantFlip := [2]bool{axis == 0 && order == 1, axis == 1 || order == 1}
		if faces[0].Surface != cut.sources[1] || faces[1].Surface != cut.sources[2] ||
			faces[0].Reversed != wantFlip[0] || faces[1].Reversed != wantFlip[1] {
			t.Fatal("ordered original caps and chart orientations", i, wantFlip)
		}
		mapped := func(p geom.Vec3) geom.Vec2 {
			return geom.Vec2{X: 5 - 3*([2]float64{p.X, p.Y})[axis], Y: 2*([2]float64{p.X, p.Y})[1-axis] - 1}
		}
		for _, use := range faces[1-order].Outer {
			e := part.edges[use.Edge]
			if use.Reversed {
				e.Start, e.End = e.End, e.Start
			}
			if use.UVStart != mapped(part.vertices[e.Start]) || use.UVEnd != mapped(part.vertices[e.End]) {
				t.Fatal("original reflected chart cap pcurve range", i, use.Edge)
			}
		}
		for _, face := range faces[2:] {
			seen := [2]bool{}
			for _, use := range face.Outer {
				for roof := range seen {
					owner, ok := graphWallSourceTrim(use.Curve, roof == 0, roof == 1-order)
					if ok && owner.surface == cut.sources[1+roof] {
						seen[roof] = true
					}
				}
			}
			if seen != [2]bool{true, true} {
				t.Fatal("wall trims retain both original owners", i, seen)
			}
		}
	}
}

func checkNativePolynomialCutMaterial(t *testing.T, cut *AlignedPolynomialCut, profile, axis, order int) {
	t.Helper()
	parts, contacts := cut.Components(), cut.Contacts()
	regions := [2][]int{{0, 4}, {1, 2, 3}}[order]
	if !slices.Equal(cut.Regions(), regions) || len(parts) != len(regions) {
		t.Fatal("two exterior shells or three between-branch shells", cut.Regions(), len(parts))
	}
	volumes, total := cut.ExactComponentVolumes()
	wantTotal := new(big.Rat)
	for i, want := range independentAlignedCutVolumes(profile, order) {
		if len(volumes) != len(parts) || volumes[i].Sign() <= 0 || volumes[i].Cmp(want) != 0 {
			t.Fatal("independent exact component integral", profile, axis, order, i, volumes, want)
		}
		wantTotal.Add(wantTotal, want)
	}
	if total.Cmp(wantTotal) != 0 {
		t.Fatal("independent exact Cut total", total, wantTotal)
	}
	volumes[0].SetInt64(0)
	total.SetInt64(0)
	actualVolumes, actualTotal := cut.ExactComponentVolumes()
	if actualVolumes[0].Sign() <= 0 || actualTotal.Sign() <= 0 {
		t.Fatal("exact integrals are immutable at the native boundary")
	}
	orders := [2][2]int{{3, 2}, {2, 3}}[profile]
	for i, c := range contacts {
		pair := [2]int{0, 1}
		if order == 1 {
			pair = [2]int{i, i + 1}
		}
		p, e := geom.RepresentedExactPoint(c.Point)
		if e != nil || c.Order != orders[i] || c.Components != pair || ([2]float64{p.X, p.Y})[axis] != float64(1+2*i)/4 {
			t.Fatal("exact shared contact identity and indexed incidences", i, c, e)
		}
		for j, part := range pair {
			if parts[part].vertices[c.Vertices[j]] != p {
				t.Fatal("original contact was not bound to a local vertex", i, j, c)
			}
		}
	}
}

func TestNativeAlignedPolynomialCutRejections(t *testing.T) {
	ctx := context.Background()
	base, a, _ := nativeCubicCrossingTangencyFixture(t, 0, 0)
	b := independentAlignedEnvelopeRoof(t, 0, 1, 25.0/8)
	cut, e := CutAlignedPolynomialComponents(ctx, base, b, a, 0, 1e-9, PolynomialNodeCutOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var raw bytes.Buffer
	if e := cut.WriteJSON(ctx, &raw); e != nil {
		t.Fatal(e)
	}
	var recipe polynomialMixedCutRecipe
	if e := json.Unmarshal(raw.Bytes(), &recipe); e != nil {
		t.Fatal(e)
	}
	recipe.Orders[1] = 3
	tampered, e := json.Marshal(recipe)
	if e != nil {
		t.Fatal(e)
	}
	if r, e := ReadAlignedPolynomialCutJSON(ctx, bytes.NewReader(tampered), JSONLimits{}, PolynomialNodeCutOptions{}); r != nil || !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("source reproof must match contact type", e)
	}
	if r, e := ReadAlignedPolynomialCutJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{MaxEntities: 1}, PolynomialNodeCutOptions{}); r != nil || !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("aggregate entity budget", e)
	}
	low := PolynomialNodeCutOptions{Proof: geom.GraphPointContactOptions{MaxOperations: 1}}
	if r, e := ReadAlignedPolynomialCutJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, low); r != nil || !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("native recipe requires alignment and exact measure proof", e)
	}
}
