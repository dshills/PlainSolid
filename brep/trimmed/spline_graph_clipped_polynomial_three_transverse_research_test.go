//go:build research

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

func TestNativeClippedPolynomialThreeTransverseFuse(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	o := PolynomialCubicCrossingTangencyEnvelopeOptions{}
	for axis := range 2 {
		_, a, _ := nativeThreePolynomialNodeFixture(t, axis, 4, 128)
		for layout := range 3 {
			s := clippedTransverseFuseStock(t, axis, layout, a)
			for order := range 2 {
				r, e := FuseClippedPolynomialEnvelope(ctx, s[0], s[1+order], s[2-order], axis, 1e-9, o)
				if e != nil {
					t.Fatal(axis, layout, order, e)
				}
				checkThreeTransverseClippedMaterial(t, r, layout, axis, order)
				checkThreeTransverseClippedFuseLedger(t, r, layout, axis)
				if layout == 2 {
					checkClippedFuseCompoundCorners(t, r, 0, axis, order)
				}
				var raw, canonical bytes.Buffer
				if e := r.WriteJSON(ctx, &raw); e != nil {
					t.Fatal(e)
				}
				loaded, e := ReadClippedPolynomialFuseJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, o)
				if e != nil || !reflect.DeepEqual(r.ledger, loaded.ledger) || !reflect.DeepEqual(r.Walls(), loaded.Walls()) ||
					r.ExactVolume().Cmp(loaded.ExactVolume()) != 0 {
					t.Fatal("rebuild must prove every original branch edge, node, wall and volume", axis, layout, order, e)
				}
				for i, source := range r.sources {
					if !geom.SameSplineSurface(source, loaded.sources[i]) {
						t.Fatal("three-node reload changed an original stock chart", i)
					}
				}
				if e := loaded.WriteJSON(ctx, &canonical); e != nil || !bytes.Equal(raw.Bytes(), canonical.Bytes()) {
					t.Fatal("canonical source recipe with three independent contacts", e)
				}
			}
		}
	}
}

func checkThreeTransverseClippedMaterial(t *testing.T, r *ClippedPolynomialFuse, layout, axis, order int) {
	t.Helper()
	// In the independent source equations g-f=(1-4u)(1-2u)(3-4u)(1+u)/128.
	// Integrate the height gap with dependent antiderivative
	// v³/3-(f+g)v²/2+fgv, scaled by 105/128 and the physical determinant
	// six. Integrate the between-branch terms with |f-g|³/6 over each
	// free slab. Include the positive exclusive original roof strips.
	fuse := [3]string{"37006903622579691/2251799813685248", "8949190016091/549755813888", "35449753287909867/2251799813685248"}[layout]
	common := [3]string{"26955626929285653/2251799813685248", "6580961624997/549755813888", "20216720162646549/2251799813685248"}[layout]
	if r.ExactVolume().RatString() != fuse {
		t.Fatal("exact native union volume from independent original stock integrals", r.ExactVolume(), fuse)
	}
	// Independent source-derivative and wall-height integration with
	// 32-point Gauss-Legendre quadrature on signed slabs.
	area := [3][2]float64{
		{40.136619825182073, 40.436898631268178},
		{39.737180716879287, 40.017778809963829},
		{39.268937973734481, 39.502336527274203},
	}[layout][axis]
	expected, _ := new(big.Rat).SetString(fuse)
	fv, _ := expected.Float64()
	ctx := context.Background()
	mass, e := r.Boundary().MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
	if e != nil || math.Abs(mass.Volume-fv) > 1e-8 || math.Abs(mass.Area-area) > 1e-8 {
		t.Fatal("native shell matches independent physical area and volume", layout, axis, mass, area, fv, e)
	}

	var patches [3]*geom.SurfacePatch
	for i, source := range r.sources {
		patches[i], e = geom.NewSurfacePatch(source, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
	}
	proof, e := geom.DiscoverClippedSplineGraphPolynomialCut(ctx, patches[0], patches[1], patches[2], axis, geom.GraphPointContactOptions{})
	if e != nil || proof.Material.Common.RatString() != common || proof.Material.Fuse.RatString() != fuse {
		t.Fatal("exact clipped material proof covers all three nodes", layout, axis, order, e)
	}
	overlap := proof.Material.Overlap
	for i, owner := range overlap.Projection.Owners {
		if owner.Domain != overlap.RestrictedDomains[i+1] || len(owner.Edges) != 8 || len(owner.Vertices) != 7 {
			t.Fatal("both original charts retain the entire three-node arrangement", i, owner)
		}
		for j, uv := range owner.Vertices {
			p, e := patches[i+1].EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{})
			want := overlap.Projection.Arrangement.Vertices[j].Point
			if e != nil || p.Point[0].Cmp(want[0]) != 0 || p.Point[1].Cmp(want[1]) != 0 || p.Point[2].Cmp(want[2]) != 0 {
				t.Fatal("original owner chart lifts each arrangement vertex exactly", i, j, e)
			}
		}
	}
	checkThreeTransverseClippedComponents(t, proof, layout, axis, order)
}

func checkThreeTransverseClippedComponents(t *testing.T, proof geom.ClippedSplineGraphPolynomialCut, layout, axis, order int) {
	t.Helper()
	raised, between := threeTransverseClippedComponentVolumes(layout, axis)
	for side, want := range [2][]string{raised, between} {
		result := proof.Orders[(side+order)%2]
		if len(result.Components) != len(want) || len(result.Contacts) != 3 {
			t.Fatal("all independently integrated clipped chambers and contacts", side, result.Components, result.Contacts)
		}
		for i, c := range result.Components {
			if c.Volume.RatString() != want[i] || c.Volume.Sign() <= 0 {
				t.Fatal("independent original positive component volume", side, i, c.Volume, want[i])
			}
		}
		for i, contact := range result.Contacts {
			a, b := contact.Components[0], contact.Components[1]
			want := [2]int{0, 0}
			if side == 1 {
				want = [2]int{i, i + 1}
			} else if layout == 1 {
				want[1] = 1
			}
			if [2]int{min(a, b), max(a, b)} != want || contact.SelfContact != (want[0] == want[1]) {
				t.Fatal("complete three-junction material-component classification", side, i, contact)
			}
		}
	}
}

func checkThreeTransverseClippedFuseLedger(t *testing.T, r *ClippedPolynomialFuse, layout, axis int) {
	t.Helper()
	s := r.Boundary()
	top := s.Topology()
	if len(top.Shells) != 1 || top.EulerCharacteristic != 2 || top.CollapsedEdges != 0 || s.faces[r.ledger.Base].Surface != r.sources[0] {
		t.Fatal("one closed original-source stepped shell", top)
	}
	for i, id := range r.ledger.RoofFaces {
		if s.faces[id].Surface != r.sources[1+r.ledger.RoofOwners[i]] {
			t.Fatal("each roof cap uses the original selected surface", i)
		}
	}
	station := [2][3]string{{"5/2", "3", "7/2"}, {"-1/4", "1/2", "5/4"}}[axis]
	derivative := [2][3]string{{"-15/256", "9/256", "-21/256"}, {"-5/192", "1/64", "-7/192"}}[axis]
	if len(r.ledger.Contacts) != 3 || len(r.ledger.Edges) != 8 || len(r.ledger.Vertices) != 7 || len(r.ledger.Midpoints) != 4 {
		t.Fatal("complete native endpoint, edge, midpoint and junction ledger", r.ledger)
	}
	for i, c := range r.ledger.Contacts {
		if c.Order != 1 || c.Point[axis] != station[i] || c.DerivativeGap != derivative[i] || len(c.Rays) != 4 || len(c.Sectors) != 4 {
			t.Fatal("original exact crossing derivative and four native sectors", i, c)
		}
	}
	midpointIDs := make(map[int]bool)
	for i, branch := range [4]int{1, 2, 5, 6} {
		c := r.ledger.Edges[branch]
		v := r.ledger.Midpoints[i]
		if len(c) != 2 || midpointIDs[v] {
			t.Fatal("each inner original restriction has a distinct subdivision", branch, c, v)
		}
		for _, edge := range c {
			e := s.edges[edge]
			if e.Start != v && e.End != v {
				t.Fatal("native chain shares its own source midpoint", branch, c, v)
			}
		}
		midpointIDs[v] = true
	}
	var risers, tapers int
	for _, w := range r.Walls() {
		for i, owner := range [2]geom.ClippedPolynomialHeightOwner{w.Low, w.High} {
			face := s.faces[w.Face]
			at := 0
			if i == 1 {
				at = 1 + len(w.Stations[1])
			}
			use := face.Outer[at]
			p, ok := graphWallSourceTrim(use.Curve, i == 1, true)
			if !ok || use.Edge != w.TraceEdges[i] || p.surface != r.sources[owner-1] ||
				p.a != w.OwnerUV[i][0] || p.b != w.OwnerUV[i][1] {
				t.Fatal("the wall coedge uses its original source UV clock", i, w)
			}
		}
		if w.Low == geom.ClippedPolynomialBaseHeight {
			continue
		}
		risers++
		if w.EndingRoofs != 1<<uint8(w.High-geom.ClippedPolynomialRoofAHeight) {
			t.Fatal("the upper stock owns the original riser side", w)
		}
		for _, chain := range w.Stations {
			if len(chain) == 0 {
				tapers++
			}
		}
	}
	if risers != [3]int{4, 1, 5}[layout] || tapers != [3]int{4, 0, 4}[layout] {
		t.Fatal("exact riser routes, terminal taper and native step splitting", layout, risers, tapers)
	}
}

func TestNativeClippedPolynomialThreeTransverseFuseRefusals(t *testing.T) {
	ctx := context.Background()
	o := PolynomialCubicCrossingTangencyEnvelopeOptions{}
	_, a, _ := nativeThreePolynomialNodeFixture(t, 0, 4, 128)
	src := clippedTransverseFuseStock(t, 0, 0, a)
	r, e := FuseClippedPolynomialEnvelope(ctx, src[0], src[1], src[2], 0, 1e-9, o)
	if e != nil {
		t.Fatal(e)
	}
	var raw bytes.Buffer
	if e := r.WriteJSON(ctx, &raw); e != nil {
		t.Fatal(e)
	}
	for _, edit := range []func(*clippedPolynomialFuseRecipe){
		func(r *clippedPolynomialFuseRecipe) { r.Ledger.Contacts = r.Ledger.Contacts[:2] },
		func(r *clippedPolynomialFuseRecipe) { r.Ledger.Contacts[2].DerivativeGap = "0" },
		func(r *clippedPolynomialFuseRecipe) { r.Ledger.Midpoints[3] = r.Ledger.Midpoints[1] },
	} {
		var stale clippedPolynomialFuseRecipe
		if e := json.Unmarshal(raw.Bytes(), &stale); e != nil {
			t.Fatal(e)
		}
		edit(&stale)
		var changed bytes.Buffer
		if e := writePolynomialGraphRecipe(ctx, &changed, stale); e != nil {
			t.Fatal(e)
		}
		if out, e := ReadClippedPolynomialFuseJSON(ctx, &changed, JSONLimits{}, o); out != nil || !errors.Is(e, geom.ErrConvergence) {
			t.Fatal("missing or altered original third contact and subdivided spans must fail replay", e)
		}
	}
	_, a, _ = nativeThreePolynomialNodeFixture(t, 0, 3, 128)
	src = clippedTransverseFuseStock(t, 0, 0, a)
	if out, e := FuseClippedPolynomialEnvelope(ctx, src[0], src[1], src[2], 0, 1e-9, o); out != nil || !errors.Is(e, geom.ErrUnrepresentable) {
		t.Fatal("all three roots must be exactly representable for native assembly", e)
	}
	_, a, _ = nativeTwoCrossingsTangencyFixture(t, 0, 2, 4)
	src = clippedTransverseFuseStock(t, 0, 0, a)
	if out, e := FuseClippedPolynomialEnvelope(ctx, src[0], src[1], src[2], 0, 1e-9, o); out != nil || !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("the three-event native admission requires each original contact to be transverse", e)
	}
}
