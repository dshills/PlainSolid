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

func restrictOriginalClipStock(t *testing.T, source *geom.BSplineSurface, d geom.ParameterRectangle) *geom.BSplineSurface {
	t.Helper()
	p, e := geom.NewSurfacePatch(source, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	q, e := p.Trim(d)
	if e != nil {
		t.Fatal(e)
	}
	s, e := q.ToBSplineSurface()
	if e != nil {
		t.Fatal("exact represented original stock controls", e)
	}
	return s
}

func clippedPolynomialStock(t *testing.T, axis, profile, layout int) [3]*geom.BSplineSurface {
	t.Helper()
	base, a, b := rectangularPolynomialStock(t, axis, profile)
	if layout == 0 {
		d := geom.ParameterRectangle{UMin: -2, UMax: 2, VMin: 8, VMax: 10}
		if axis == 0 {
			d.UMax = 1.5
		} else {
			d.VMin = 8.25
		}
		a = restrictOriginalClipStock(t, a, d)
	}
	d := geom.ParameterRectangle{UMin: 2, UMax: 4.625, VMin: -1, VMax: 1}
	if layout == 1 {
		d.UMin = 2.375
	}
	b = restrictOriginalClipStock(t, b, d)
	return [3]*geom.BSplineSurface{base, a, b}
}

func clippedCommonMeasures(axis, profile int) (area, volume float64) {
	// Exact antiderivatives of B-base over [1/8,7/8], less the interior
	// layer (105/768) times the integral of |(u-c)^9(u-t)^6|.
	// The physical XY determinant is six. The reversed profile has the
	// same integral by reflection in the middle of the free interval.
	exact, _ := new(big.Rat).SetString("493395797792490953145/41216943789694779392")
	volume, _ = exact.Float64()
	lengths := [2]float64{2, 3}
	slopes := [2]float64{1.0 / 4, 3.0 / 16}
	area = polynomialPairSourceEnvelopeAreaScaled(false, []float64{.125, .25, .75, .875}, cubicCrossingTangencyBranches(profile),
		lengths[axis], lengths[1-axis], [3]float64{1.0 / 8, slopes[axis], slopes[1-axis]})
	return area, volume
}

func TestNativeClippedPolynomialCommon(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	o := PolynomialCubicCrossingTangencyEnvelopeOptions{}
	for layout := 0; layout < 2; layout++ {
		for axis := 0; axis < 2; axis++ {
			// Test both contact orders without coupling them to chart orientation.
			profile := layout
			src := clippedPolynomialStock(t, axis, profile, layout)
			for order, roofs := range [][2]*geom.BSplineSurface{{src[1], src[2]}, {src[2], src[1]}} {
				common, e := CommonClippedPolynomialEnvelope(ctx, src[0], roofs[0], roofs[1], axis, 1e-9, o)
				if e != nil {
					t.Fatal("native Common on independently authored stock extent", layout, axis, order, e)
				}
				checkNativeClippedCommon(t, common, layout, axis, profile, order)
				var raw bytes.Buffer
				if e := common.WriteJSON(ctx, &raw); e != nil {
					t.Fatal(e)
				}
				loaded, e := ReadClippedPolynomialCommonJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, o)
				if e != nil || loaded.History() != common.History() || !reflect.DeepEqual(loaded.Contacts(), common.Contacts()) {
					t.Fatal("canonical original native contact ledger", layout, axis, order, e)
				}
				checkNativeClippedCommon(t, loaded, layout, axis, profile, order)
				for i, source := range common.sources {
					if !geom.SameSplineSurface(source, loaded.sources[i]) {
						t.Fatal("reload changed an independently bounded input", i)
					}
				}
				a, v := clippedCommonMeasures(axis, profile)
				mass, e := loaded.Boundary().MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
				if e != nil || math.Abs(mass.Area-a) > 1e-8 || math.Abs(mass.Volume-v) > 1e-8 {
					t.Fatal("independent clipped physical area and volume", layout, axis, order, mass.Area, a, mass.Volume, v, e)
				}
				var echo bytes.Buffer
				if e := loaded.WriteJSON(ctx, &echo); e != nil || !bytes.Equal(raw.Bytes(), echo.Bytes()) {
					t.Fatal("canonical clipped Common recipe includes exact owner limits", e)
				}
			}
		}
	}
}

func checkNativeClippedCommon(t *testing.T, r *AlignedPolynomialEnvelope, layout, axis, profile, order int) {
	t.Helper()
	s, history := r.Boundary(), r.History()
	top := s.Topology()
	fp, ok := r.ClippedFootprint()
	physical := [2]geom.ParameterRectangle{{UMin: 2.25, UMax: 3.75, VMin: -1, VMax: 2}, {UMin: 2, UMax: 4, VMin: -.625, VMax: 1.625}}[axis]
	if !ok || fp.Physical != physical || len(top.Shells) != 1 || top.EulerCharacteristic != 2 || top.CollapsedEdges != 0 ||
		len(s.faces) != 14 || s.faces[history.Base].Surface != r.sources[0] || s.faces[history.Base].Reversed {
		t.Fatal("one embedded physical overlap shell on the original base", fp.Physical, top)
	}
	mask := [2][2][4]uint8{{{3, 1, 3, 2}, {2, 3, 1, 3}}, {{3, 2, 3, 2}, {2, 3, 2, 3}}}[layout][axis]
	if order == 1 {
		for i, side := range mask {
			mask[i] = (side&1)<<1 | (side&2)>>1
		}
	}
	if fp.BoundaryRoofs != mask {
		t.Fatal("original wall clip ownership", fp.BoundaryRoofs, mask)
	}
	for i, source := range r.sources {
		d := source.Data()
		original := geom.ParameterRectangle{UMin: d.KnotsU[0], UMax: d.KnotsU[len(d.KnotsU)-1], VMin: d.KnotsV[0], VMax: d.KnotsV[len(d.KnotsV)-1]}
		if fp.Original[i] != original || fp.Restricted[i].UMin < original.UMin || fp.Restricted[i].UMax > original.UMax ||
			fp.Restricted[i].VMin < original.VMin || fp.Restricted[i].VMax > original.VMax {
			t.Fatal("source chart extent and exact native restriction", i, fp)
		}
	}
	for _, p := range s.vertices {
		if p.X < physical.UMin || p.X > physical.UMax || p.Y < physical.VMin || p.Y > physical.VMax {
			t.Fatal("native vertices stay in the exact stock overlap", p, physical)
		}
	}

	toUnit := func(p geom.Vec3) geom.Vec3 { return geom.Vec3{X: (p.X - 2) / 2, Y: (p.Y + 1) / 3, Z: p.Z} }
	modelEnd := func(u Use) (geom.Vec3, geom.Vec3) {
		e := s.edges[u.Edge]
		if u.Reversed {
			e.Start, e.End = e.End, e.Start
		}
		return s.vertices[e.Start], s.vertices[e.End]
	}
	for i, c := range r.Contacts() {
		p, e := geom.RepresentedExactPoint(c.Point)
		q := toUnit(p)
		if e != nil || c.Order != ([2][2]int{{3, 2}, {2, 3}}[profile][i]) ||
			([2]float64{q.X, q.Y})[axis] != float64(1+2*i)/4 || s.vertices[c.Vertex] != p {
			t.Fatal("absolute clipped station and typed native contact", i, c, e)
		}
	}
	for i, id := range history.RoofFaces {
		owner := 0
		if i == 0 || i == 4 {
			owner = 1
		}
		face := s.faces[id]
		if history.RoofOwners[i] != owner^order || face.Surface != r.sources[1+(owner^order)] {
			t.Fatal("selection uses the original owner support", i, id)
		}
		for _, use := range face.Outer {
			a, b := modelEnd(use)
			if use.UVStart != projectedFixtureUV(owner, axis, toUnit(a)) || use.UVEnd != projectedFixtureUV(owner, axis, toUnit(b)) {
				t.Fatal("clipped native coedge keeps original UV intervals", i, use.Edge)
			}
		}
	}
	var clippedInterior bool
	for _, f := range s.faces[history.Base+1:] {
		bottom, roof := f.Outer[0], f.Outer[2]
		a, b := modelEnd(bottom)
		lo, yes := graphWallSourceTrim(bottom.Curve, false, true)
		if !yes || lo.surface != r.sources[0] || lo.a != clippedBaseUV(toUnit(a)) || lo.b != clippedBaseUV(toUnit(b)) {
			t.Fatal("clipped wall keeps the internal or exterior original base trace")
		}
		side := -1
		switch {
		case a.Y == physical.VMin && b.Y == physical.VMin:
			side = 0
		case a.X == physical.UMax && b.X == physical.UMax:
			side = 1
		case a.Y == physical.VMax && b.Y == physical.VMax:
			side = 2
		case a.X == physical.UMin && b.X == physical.UMin:
			side = 3
		}
		hi, yes := graphWallSourceTrim(roof.Curve, true, true)
		if !yes || side < 0 {
			t.Fatal("every wall bounds the proved physical footprint", side)
		}
		owner := 0
		if hi.surface != r.sources[1+order] {
			owner = 1
		}
		b, a = modelEnd(roof)
		if hi.surface != r.sources[1+(owner^order)] || hi.a != projectedFixtureUV(owner, axis, toUnit(a)) ||
			hi.b != projectedFixtureUV(owner, axis, toUnit(b)) {
			t.Fatal("wall top binds original selected roof over actual intersection perimeter", side, owner)
		}
		clippedInterior = clippedInterior || fp.BoundaryRoofs[side]&(1<<(owner^order)) == 0
	}
	if !clippedInterior {
		t.Fatal("fixture must include clipping a selected roof along the other stock's footprint")
	}
}

func clippedBaseUV(p geom.Vec3) geom.Vec2 { return geom.Vec2{X: 10 - 2*p.Y, Y: 2 - 4*p.X} }

func TestNativeClippedPolynomialCommonTwoAxes(t *testing.T) {
	ctx := context.Background()
	src := clippedPolynomialStock(t, 0, 0, 0)
	src[1] = restrictOriginalClipStock(t, src[1], geom.ParameterRectangle{UMin: -2, UMax: 1.5, VMin: 8.25, VMax: 10})
	src[2] = restrictOriginalClipStock(t, src[2], geom.ParameterRectangle{UMin: 2, UMax: 4.625, VMin: -.75, VMax: 1})
	o := PolynomialCubicCrossingTangencyEnvelopeOptions{}
	r, e := CommonClippedPolynomialEnvelope(ctx, src[0], src[1], src[2], 0, 1e-9, o)
	if e != nil {
		t.Fatal(e)
	}
	fp, ok := r.ClippedFootprint()
	if !ok || fp.Physical != (geom.ParameterRectangle{UMin: 2.25, UMax: 3.75, VMin: -.625, VMax: 1.625}) ||
		fp.BoundaryRoofs != ([4]uint8{2, 1, 1, 2}) {
		t.Fatal("alternating original extent ownership on all four clipped sides", fp)
	}
	mass, e := r.Boundary().MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
	// The absolute branch-gap integral is unchanged. The independent
	// baseline height integral now covers a 3/4 by 3/4 parameter box.
	exact, _ := new(big.Rat).SetString("370046848335928793529/41216943789694779392")
	want, _ := exact.Float64()
	if e != nil || math.Abs(mass.Volume-want) > 1e-8 {
		t.Fatal("independent physical volume clipped in both axes", mass.Volume, want, e)
	}
	var seed bytes.Buffer
	if e := r.WriteJSON(ctx, &seed); e != nil {
		t.Fatal(e)
	}
	reloaded, e := ReadClippedPolynomialCommonJSON(ctx, bytes.NewReader(seed.Bytes()), JSONLimits{}, o)
	if e != nil {
		t.Fatal(e)
	}
	if q, ok := reloaded.ClippedFootprint(); !ok || q != fp {
		t.Fatal("two-axis original chart clipping survives source rebuild", q)
	}
}

func TestNativeClippedPolynomialCommonRejections(t *testing.T) {
	ctx := context.Background()
	o := PolynomialCubicCrossingTangencyEnvelopeOptions{}
	src := clippedPolynomialStock(t, 0, 0, 0)
	r, e := CommonClippedPolynomialEnvelope(ctx, src[0], src[1], src[2], 0, 1e-9, o)
	if e != nil {
		t.Fatal(e)
	}
	var raw bytes.Buffer
	if e := r.WriteJSON(ctx, &raw); e != nil {
		t.Fatal(e)
	}
	_, fullA, _ := rectangularPolynomialStock(t, 0, 0)
	extended, e := encodePolynomialGraphSupports(ctx, [3]*geom.BSplineSurface{src[0], fullA, src[2]})
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		name string
		edit func(*clippedPolynomialCommonRecipe)
		err  error
	}{
		{"missing footprint", func(p *clippedPolynomialCommonRecipe) { p.Footprint = nil }, geom.ErrInvalid},
		{"forged wall clip owner", func(p *clippedPolynomialCommonRecipe) { p.Footprint.BoundaryRoofs[1] = 3 }, geom.ErrConvergence},
		{"edited shared footprint", func(p *clippedPolynomialCommonRecipe) { p.Footprint.Physical.UMax = 4 }, geom.ErrConvergence},
		{"stale original extent", func(p *clippedPolynomialCommonRecipe) { p.Sources = extended }, geom.ErrConvergence},
		{"changed operation", func(p *clippedPolynomialCommonRecipe) { *p.Operation = parametric.Union }, geom.ErrInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var recipe clippedPolynomialCommonRecipe
			if e := json.Unmarshal(raw.Bytes(), &recipe); e != nil {
				t.Fatal(e)
			}
			tc.edit(&recipe)
			bad, e := json.Marshal(recipe)
			if e != nil {
				t.Fatal(e)
			}
			if r, e := ReadClippedPolynomialCommonJSON(ctx, bytes.NewReader(bad), JSONLimits{}, o); r != nil || !errors.Is(e, tc.err) {
				t.Fatal("replay must validate the clipped stock ledger", tc.err, e)
			}
		})
	}
	shortBase := restrictOriginalClipStock(t, src[0], geom.ParameterRectangle{UMin: 8, UMax: 10, VMin: -1.5, VMax: 2})
	if r, e := CommonClippedPolynomialEnvelope(ctx, shortBase, src[1], src[2], 0, 1e-9, o); r != nil || !errors.Is(e, geom.ErrDomain) {
		t.Fatal("base must cover both full original stocks", e)
	}
	if r, e := ReadClippedPolynomialCommonJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{MaxEntities: 1}, o); r != nil || !errors.Is(e, geom.ErrConvergence) {
		t.Fatal("clipped native topology budget", e)
	}
}
