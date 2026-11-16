package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"reflect"
	"slices"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestNativeClippedPolynomialThreeTransverseCommon(t *testing.T) {
	ctx := context.Background()
	o := PolynomialCubicCrossingTangencyEnvelopeOptions{}
	for axis := range 2 {
		_, a, _ := nativeThreePolynomialNodeFixture(t, axis, 4, 128)
		for layout := range 3 {
			src := clippedTransverseFuseStock(t, axis, layout, a)
			for order := range 2 {
				r, e := CommonClippedPolynomialThreeTransverseEnvelope(ctx, src[0], src[1+order], src[2-order], axis, 1e-9, o)
				if e != nil {
					t.Fatal(axis, layout, order, e)
				}
				checkThreeTransverseClippedCommonMeasure(t, r, layout, axis)
				checkThreeTransverseClippedCommonHistory(t, r, layout, axis, order)
				var raw, echo bytes.Buffer
				if e := r.WriteJSON(ctx, &raw); e != nil {
					t.Fatal(e)
				}
				loaded, e := ReadClippedPolynomialThreeTransverseCommonJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, o)
				if e != nil || !reflect.DeepEqual(r.History(), loaded.History()) ||
					!reflect.DeepEqual(r.Contacts(), loaded.Contacts()) || r.footprint != loaded.footprint {
					t.Fatal("the original overlap, shell and third crossing re-prove on reload", axis, layout, order, e)
				}
				for i, source := range r.sources {
					if !geom.SameSplineSurface(source, loaded.sources[i]) {
						t.Fatal("source replay preserves the original stock chart exactly", i)
					}
				}
				if e := loaded.WriteJSON(ctx, &echo); e != nil || !bytes.Equal(raw.Bytes(), echo.Bytes()) {
					t.Fatal("canonical three-crossing source receipt", e)
				}
			}
		}
	}
}

func checkThreeTransverseClippedCommonMeasure(t *testing.T, r *ClippedPolynomialThreeTransverseCommon, layout, axis int) {
	t.Helper()
	// Integrate B-base in the exact overlap and subtract the four
	// between-branch fiber integrals (105/768)|f-g|³, where
	// g-f=(1-4u)(1-2u)(3-4u)(1+u)/128. The XY determinant is six.
	volumes := [3]string{"26955626929285653/2251799813685248", "6580961624997/549755813888", "20216720162646549/2251799813685248"}
	v, _ := new(big.Rat).SetString(volumes[layout])
	value, _ := v.Float64()
	// Independent 32-point Gauss-Legendre quadrature on the original
	// signed roof sectors, four wall height intervals and sloping base.
	areas := [3][2]float64{
		{32.985201663206595, 31.655121195484170},
		{31.655074821630230, 32.985172005039715},
		{26.734014357843741, 26.734014796565077},
	}
	ctx := context.Background()
	p, e := r.Boundary().MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
	if e != nil || math.Abs(p.Volume-value) > 1e-8 || math.Abs(p.Area-areas[layout][axis]) > 1e-8 {
		t.Fatal("native shell agrees with independent physical Common integrals", axis, layout, p, value, areas[layout][axis], e)
	}
	var patches [3]*geom.SurfacePatch
	for i, source := range r.sources {
		patches[i], e = geom.NewSurfacePatch(source, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
	}
	proof, e := geom.IntegrateClippedSplineGraphPolynomialMaterial(ctx, patches[0], patches[1], patches[2], axis, geom.GraphPointContactOptions{})
	if e != nil || proof.Common.Cmp(v) != 0 {
		t.Fatal("exact original overlap proof agrees with the independent material volume", proof.Common, v, e)
	}
}

func checkThreeTransverseClippedCommonHistory(t *testing.T, r *ClippedPolynomialThreeTransverseCommon, layout, axis, order int) {
	t.Helper()
	s := r.Boundary()
	h := r.History()
	top := s.Topology()
	if len(top.Shells) != 1 || top.EulerCharacteristic != 2 || top.CollapsedEdges != 0 || s.faces[h.Base].Surface != r.sources[0] {
		t.Fatal("one original-source Common shell", top)
	}
	if len(h.Vertices) != 7 || len(h.Edges) != 8 || len(h.Midpoints) != 4 || len(h.RoofFaces) != 6 || len(h.RoofOwners) != 6 {
		t.Fatal("no original endpoint, branch, split or roof sector is omitted", h)
	}
	for i, id := range h.RoofFaces {
		if s.faces[id].Surface != r.sources[1+h.RoofOwners[i]] {
			t.Fatal("original selected roof surface", i, id)
		}
	}
	for i, branch := range [4]int{1, 2, 5, 6} {
		vertex := h.Midpoints[i]
		for _, edge := range h.Edges[branch] {
			e := s.edges[edge]
			if e.Start != vertex && e.End != vertex {
				t.Fatal("interior source split binds both native edge pieces", branch, edge, vertex)
			}
		}
	}
	station := [2][3]string{{"5/2", "3", "7/2"}, {"-1/4", "1/2", "5/4"}}[axis]
	derivative := [2][3]string{{"-15/256", "9/256", "-21/256"}, {"-5/192", "1/64", "-7/192"}}[axis]
	contacts := r.Contacts()
	if len(contacts) != 3 {
		t.Fatal("three independently bound source crossings", contacts)
	}
	for i, c := range contacts {
		if c.Order != 1 || c.Point[axis].RatString() != station[i] || c.DerivativeGap.RatString() != derivative[i] {
			t.Fatal("the source crossing keeps its exact physical station and jet", i, c)
		}
		for j, ray := range c.Rays {
			e := s.edges[ray]
			if (e.Start == c.Vertex) == (e.End == c.Vertex) || !slices.Contains(h.RoofFaces, c.Sectors[j]) {
				t.Fatal("original ray and sector uses bind to native edges and caps", i, j, c)
			}
		}
	}
	fp, ok := r.ClippedFootprint()
	if !ok {
		t.Fatal("missing exact original footprint")
	}
	both := geom.ParameterRectangle{UMin: 2.25, UMax: 3.75, VMin: -.625, VMax: 1.625}
	free := [2]geom.ParameterRectangle{{UMin: 2.25, UMax: 3.75, VMin: -1, VMax: 2}, {UMin: 2, UMax: 4, VMin: -.625, VMax: 1.625}}
	if fp.Physical != [3]geom.ParameterRectangle{free[axis], free[1-axis], both}[layout] {
		t.Fatal("exact original physical footprint", fp.Physical)
	}
	checkClippedCommonWallOwners(t, s, h.Base, r.sources, fp, layout, axis, order)

	h.RoofOwners[0], h.Edges[0][0], h.Vertices[0], h.Midpoints[0], h.RoofFaces[0] = -1, -1, -1, -1, -1
	contacts[2].Point[axis].SetInt64(0)
	contacts[2].DerivativeGap.SetInt64(0)
	again := r.History()
	if again.RoofOwners[0] < 0 || again.Edges[0][0] < 0 || again.Vertices[0] < 0 || again.Midpoints[0] < 0 ||
		again.RoofFaces[0] < 0 || r.Contacts()[2].Point[axis].RatString() != station[2] || r.Contacts()[2].DerivativeGap.RatString() != derivative[2] {
		t.Fatal("the complete exposed ledger has caller-owned values")
	}
}

func TestNativeClippedPolynomialThreeTransverseCommonRefusals(t *testing.T) {
	ctx := context.Background()
	o := PolynomialCubicCrossingTangencyEnvelopeOptions{}
	_, a, _ := nativeThreePolynomialNodeFixture(t, 0, 4, 128)
	src := clippedTransverseFuseStock(t, 0, 0, a)
	r, e := CommonClippedPolynomialThreeTransverseEnvelope(ctx, src[0], src[1], src[2], 0, 1e-9, o)
	if e != nil {
		t.Fatal(e)
	}
	var raw bytes.Buffer
	if e := r.WriteJSON(ctx, &raw); e != nil {
		t.Fatal(e)
	}
	for _, edit := range []func(*clippedPolynomialCommonRecipe){
		func(r *clippedPolynomialCommonRecipe) { r.Ledger.Contacts[2].DerivativeGap = "0" },
		func(r *clippedPolynomialCommonRecipe) { r.Ledger.Contacts[2].Rays[0] = -1 },
		func(r *clippedPolynomialCommonRecipe) { r.Ledger.Midpoints[3] = r.Ledger.Midpoints[0] },
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
		if out, e := ReadClippedPolynomialThreeTransverseCommonJSON(ctx, &changed, JSONLimits{}, o); out != nil || !errors.Is(e, geom.ErrConvergence) {
			t.Fatal("original third junction, branch splits and footprint roles must re-prove", e)
		}
	}
	if out, e := ReadClippedPolynomialCommonJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, o); out != nil || !errors.Is(e, geom.ErrInvalid) {
		t.Fatal("the legacy two-contact reader keeps its own source kind", e)
	}
	if out, e := CommonClippedPolynomialEnvelope(ctx, src[0], src[1], src[2], 0, 1e-9, o); out != nil || !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("the legacy result cannot truncate a native three-contact shell", e)
	}
	_, a, _ = nativeThreePolynomialNodeFixture(t, 0, 3, 128)
	src = clippedTransverseFuseStock(t, 0, 0, a)
	if out, e := CommonClippedPolynomialThreeTransverseEnvelope(ctx, src[0], src[1], src[2], 0, 1e-9, o); out != nil ||
		!errors.Is(e, geom.ErrUnrepresentable) {
		t.Fatal("every crossing must have an exactly represented native vertex", e)
	}
}
