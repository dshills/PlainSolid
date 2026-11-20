package plaincadbridge

import (
	"bytes"
	"context"
	"encoding/base64"
	"math"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func TestPlacedSideCrossingCircularCut(t *testing.T) {
	for _, x := range []float64{28, 30, 31} {
		b := New()
		base := box(t, b, vector{}, vector{30, 20, 8})
		tool := circle(t, b, vector{x, 7, -1}, vector{0, 0, 1}, 3, 10)
		out := shape(t, b, map[string]any{"op": "cut", "a": base.Handle, "b": tool.Handle})
		scoped := shape(t, b, map[string]any{"op": "cutMany", "a": base.Handle, "tools": []string{tool.Handle}})
		if scoped.Bounds != out.Bounds || scoped.GeometryAssertions != out.GeometryAssertions {
			t.Fatal("single scoped through notch differs from direct Cut", scoped, out)
		}
		if out.Bounds != base.Bounds {
			t.Fatal("notch changed surviving stock extrema", out.Bounds, base.Bounds)
		}
		a := 30 - x
		area := 9*(math.Pi/2+math.Asin(a/3)) + a*math.Sqrt(9-a*a)
		near(t, out.GeometryAssertions.Volume, 4800-8*area)
		s := b.shapes[out.Handle].analytic.solid
		p, e := s.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-9})
		if e != nil {
			t.Fatal(e)
		}
		near(t, p.Volume, out.GeometryAssertions.Volume)
		near(t, p.Area, out.GeometryAssertions.SurfaceArea)
		m, e := s.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .5, Angle: .1, MaxTriangles: 100000, MaxSubdivisions: 128})
		if e != nil {
			t.Fatal(e)
		}
		if e = m.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
	}
}

func TestPlacedSideCrossingBlindCylinderCut(t *testing.T) {
	for _, axis := range []int{0, 1, 2} {
		for _, direction := range []float64{-1, 1} {
			b := New()
			lo, hi := vector{16, 32, 48}, vector{36, 52, 68}
			base := box(t, b, lo, hi)
			for _, offset := range []float64{-1, 1} {
				normal := vector{}
				normal[axis] = direction
				origin := lo
				if direction < 0 {
					origin[axis] = hi[axis]
				}
				origin[(axis+1)%3] += 10
				origin[(axis+2)%3] += offset
				inset := offset
				sweep := math.Pi + 2*math.Asin(inset/3)
				chord := 2 * math.Sqrt(9-inset*inset)
				area := (9*sweep + inset*chord) / 2
				for _, reversed := range []bool{false, true} {
					point, n := origin, normal
					if reversed {
						point[axis] += 6 * direction
						n[axis] = -direction
					}
					tool := circle(t, b, point, n, 3, 6)
					near(t, overlapMeasure(t, b, base.Handle, tool.Handle), 6*area)
					near(t, overlapMeasure(t, b, tool.Handle, base.Handle), 6*area)
					for _, q := range []map[string]any{
						{"op": "cut", "a": base.Handle, "b": tool.Handle},
						{"op": "cutMany", "a": base.Handle, "tools": []string{tool.Handle}},
					} {
						cut := shape(t, b, q)
						near(t, cut.GeometryAssertions.Volume, 8000-6*area)
						near(t, cut.GeometryAssertions.SurfaceArea, 2400+6*(3*sweep-chord))
						if cut.Bounds != base.Bounds {
							t.Fatalf("stock extrema changed: %+v", cut.Bounds)
						}
						mesh, err := b.shapes[cut.Handle].tessellate(request{})
						if err != nil {
							t.Fatal(axis, direction, offset, err)
						}
						if err := mesh.ValidateSolid(); err != nil {
							t.Fatal(err)
						}
					}
				}
			}
		}
	}
}

func TestPlainCADSideNotchSTEPRoundTrip(t *testing.T) {
	for _, depth := range []float64{6, 10} {
		checkPlainCADSideNotchSTEP(t, vector{10, 1, 0}, depth)
		checkPlainCADSideNotchSTEP(t, vector{}, depth)
	}
}

func TestCornerBlindCutQualifiesEitherToolEndpoint(t *testing.T) {
	b := New()
	stock := box(t, b, vector{}, vector{20, 20, 8})
	interior := circle(t, b, vector{0, 0, 5}, vector{0, 0, -1}, 3, 5)
	cap := circle(t, b, vector{}, vector{0, 0, 1}, 3, 5)
	want := shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": cap.Handle})
	near(t, overlapMeasure(t, b, stock.Handle, interior.Handle), 45*math.Pi/4)
	near(t, overlapMeasure(t, b, interior.Handle, stock.Handle), 45*math.Pi/4)
	for _, op := range []map[string]any{
		{"op": "cut", "a": stock.Handle, "b": interior.Handle},
		{"op": "cutMany", "a": stock.Handle, "tools": []string{interior.Handle}},
	} {
		cut := shape(t, b, op)
		if cut.Bounds != want.Bounds || cut.GeometryAssertions != want.GeometryAssertions {
			t.Fatal(cut, want)
		}
		if _, err := b.exportSTEP([]string{cut.Handle}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestNotchedStockSTEPUnitsRetainRecipe(t *testing.T) {
	b := New()
	frame, err := geom.FrameFromNormal(geom.Vec3{X: 1, Y: 2, Z: 3}, geom.Vec3{Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	notch := step.SideNotchBody{
		Name: "corner bore", Frame: frame, Right: 20, Back: 20,
		Length: 8, Radius: 3, Depth: 6,
		Blind: []trimmed.ExtrusionBlindHole{{
			ExtrusionCircle: trimmed.ExtrusionCircle{Center: geom.Vec2{X: 10, Y: 10}, Radius: 1},
			Depth:           4, OppositeCap: true,
		}},
	}
	var source bytes.Buffer
	if err := step.WriteSideNotch(&source, step.SideNotchModel{Body: notch, LengthUnit: .01}); err != nil {
		t.Fatal(err)
	}
	imported, err := b.importSTEP(source.String())
	if err != nil {
		t.Fatal(err)
	}
	near(t, imported.GeometryAssertions.Volume, (3200-17.5*math.Pi)*1000)
	if imported.Bounds != (Bounds{Min: vector{10, 20, 30}, Max: vector{210, 220, 110}}) {
		t.Fatal(imported.Bounds)
	}
	tool := circle(t, b, vector{150, 60, 30}, vector{0, 0, 1}, 10, 80)
	cut, err := b.cutMany(imported.Handle, []string{tool.Handle})
	if err != nil {
		t.Fatal(err)
	}
	near(t, cut.GeometryAssertions.Volume, imported.GeometryAssertions.Volume-8000*math.Pi)
	if _, err := b.exportSTEP([]string{cut.Handle}); err != nil {
		t.Fatal(err)
	}
}

func checkPlainCADSideNotchSTEP(t *testing.T, origin vector, depth float64) {
	t.Helper()
	b := New()
	stock := box(t, b, vector{}, vector{20, 20, 8})
	tool := circle(t, b, origin, vector{0, 0, 1}, 3, depth)
	cut, err := b.cutMany(stock.Handle, []string{tool.Handle})
	if err != nil {
		t.Fatal(err)
	}
	moved := shape(t, b, map[string]any{"op": "transform", "handle": cut.Handle, "matrix": [][]float64{{0, 0, 1, 7}, {1, 0, 0, 8}, {0, 1, 0, 9}}})
	reflected := shape(t, b, map[string]any{"op": "transform", "handle": cut.Handle, "matrix": [][]float64{{-1, 0, 0, 30}, {0, 1, 0, 2}, {0, 0, 1, 0}}})
	for _, body := range []Shape{cut, moved, reflected} {
		file, err := b.exportSTEP([]string{body.Handle})
		if err != nil {
			t.Fatal(err)
		}
		data, err := base64.StdEncoding.DecodeString(file.Base64)
		if err != nil {
			t.Fatal(err)
		}
		reread, err := b.importSTEP(string(data))
		if err != nil {
			t.Fatal(err)
		}
		near(t, reread.GeometryAssertions.Volume, body.GeometryAssertions.Volume)
		near(t, reread.GeometryAssertions.SurfaceArea, body.GeometryAssertions.SurfaceArea)
		if reread.Bounds != body.Bounds {
			t.Fatalf("STEP changed placed bounds: %+v, want %+v", reread.Bounds, body.Bounds)
		}
		mesh, err := b.shapes[reread.Handle].tessellate(request{})
		if err != nil {
			t.Fatal(err)
		}
		if err := mesh.ValidateSolid(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSideNotchMixedBoreBatchIsAtomic(t *testing.T) {
	slowtest.Skip(t)
	for _, depth := range []float64{6, 10} {
		checkSideNotchMixedBoreBatch(t, vector{10, 1, 0}, depth)
		checkSideNotchMixedBoreBatch(t, vector{}, depth)
	}
}

func TestSideNotchRetainsConnectedCoaxialCuts(t *testing.T) {
	for _, origin := range []vector{{10, 1, 0}, {0, 0, 0}} {
		b := New()
		stock := box(t, b, vector{}, vector{20, 20, 12})
		lower := circle(t, b, origin, vector{0, 0, 1}, 3, 5)
		middle := circle(t, b, vector{origin[0], origin[1], 4}, vector{0, 0, 1}, 3, 5)
		upper := circle(t, b, vector{origin[0], origin[1], 12}, vector{0, 0, -1}, 3, 4)
		all := circle(t, b, origin, vector{0, 0, 1}, 3, 12)
		blind := shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": lower.Handle})
		want := shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": all.Handle})
		near(t, overlapMeasure(t, b, blind.Handle, all.Handle), blind.GeometryAssertions.Volume-want.GeometryAssertions.Volume)
		// The legacy notch holds one connected axial span and declines a
		// second notch apart from the first. The general Boolean cuts it:
		// the same footprint as the first, which is five deep, over four.
		apart := declined(t, b, "coaxial notch cuts require connected axial spans", "cutMany", blind.Handle, upper.Handle)
		footprint := (stock.GeometryAssertions.Volume - blind.GeometryAssertions.Volume) / 5
		near(t, apart.GeometryAssertions.Volume, blind.GeometryAssertions.Volume-4*footprint)
		for _, tc := range []struct {
			base  Shape
			tools []string
		}{
			{blind, []string{upper.Handle, middle.Handle}},
			{blind, []string{all.Handle}},
			{stock, []string{lower.Handle, upper.Handle, middle.Handle}},
		} {
			cut, err := b.cutMany(tc.base.Handle, tc.tools)
			if err != nil {
				t.Fatal(origin, err)
			}
			near(t, cut.GeometryAssertions.Volume, want.GeometryAssertions.Volume)
			near(t, cut.GeometryAssertions.SurfaceArea, want.GeometryAssertions.SurfaceArea)
			near(t, overlapMeasure(t, b, cut.Handle, all.Handle), 0)
			if _, err := b.exportSTEP([]string{cut.Handle}); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestNotchedStockMergesInteriorBoresInAnyOrder(t *testing.T) {
	b := New()
	stock := box(t, b, vector{}, vector{20, 20, 12})
	notch := circle(t, b, vector{}, vector{0, 0, 1}, 3, 5)
	base := shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": notch.Handle})
	lower := circle(t, b, vector{10, 10, 0}, vector{0, 0, 1}, 2, 5)
	upper := circle(t, b, vector{10, 10, 12}, vector{0, 0, -1}, 2, 4)
	through := circle(t, b, vector{10, 10, 0}, vector{0, 0, 1}, 2, 12)
	prior, err := b.cutMany(base.Handle, []string{lower.Handle})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		base  Shape
		tools []string
	}{
		{base, []string{lower.Handle, upper.Handle, through.Handle}},
		{base, []string{through.Handle, upper.Handle, lower.Handle}},
		{prior, []string{upper.Handle, through.Handle}},
	} {
		cut, err := b.cutMany(tc.base.Handle, tc.tools)
		if err != nil {
			t.Fatal(err)
		}
		near(t, cut.GeometryAssertions.Volume, base.GeometryAssertions.Volume-48*math.Pi)
		near(t, cut.GeometryAssertions.SurfaceArea, base.GeometryAssertions.SurfaceArea+40*math.Pi)
		if _, err := b.exportSTEP([]string{cut.Handle}); err != nil {
			t.Fatal(err)
		}
	}
	separated, err := b.cutMany(prior.Handle, []string{upper.Handle})
	if err != nil {
		t.Fatal(err)
	}
	near(t, separated.GeometryAssertions.Volume, base.GeometryAssertions.Volume-36*math.Pi)
	near(t, separated.GeometryAssertions.SurfaceArea, base.GeometryAssertions.SurfaceArea+36*math.Pi)
	near(t, overlapMeasure(t, b, separated.Handle, through.Handle), 12*math.Pi)
	if _, err := b.exportSTEP([]string{separated.Handle}); err != nil {
		t.Fatal(err)
	}
	opened, err := b.cutMany(separated.Handle, []string{through.Handle})
	if err != nil {
		t.Fatal(err)
	}
	near(t, opened.GeometryAssertions.Volume, base.GeometryAssertions.Volume-48*math.Pi)
	near(t, opened.GeometryAssertions.SurfaceArea, base.GeometryAssertions.SurfaceArea+40*math.Pi)
}

func checkSideNotchMixedBoreBatch(t *testing.T, origin vector, depth float64) {
	t.Helper()
	b := New()
	stock := box(t, b, vector{}, vector{20, 20, 8})
	notch := circle(t, b, origin, vector{0, 0, 1}, 3, depth)
	blind := circle(t, b, vector{5, 10, 0}, vector{0, 0, 1}, 2, 3)
	through := circle(t, b, vector{15, 10, -1}, vector{0, 0, 1}, 2, 10)
	opposed := circle(t, b, vector{10, 14, 8}, vector{0, 0, -1}, 1, 5)
	cut := shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": notch.Handle})
	for _, tc := range []struct {
		base  Shape
		tools []string
	}{
		{stock, []string{notch.Handle, blind.Handle, through.Handle, opposed.Handle}},
		{stock, []string{through.Handle, opposed.Handle, blind.Handle, notch.Handle}},
		{cut, []string{opposed.Handle, through.Handle, blind.Handle}},
	} {
		before := len(b.shapes)
		result, err := b.cutMany(tc.base.Handle, tc.tools)
		if err != nil || len(b.shapes) != before+1 {
			t.Fatal("mixed batch did not publish exactly one result", err)
		}
		near(t, result.GeometryAssertions.Volume, cut.GeometryAssertions.Volume-49*math.Pi)
		near(t, result.GeometryAssertions.SurfaceArea, cut.GeometryAssertions.SurfaceArea+46*math.Pi)
		near(t, overlapMeasure(t, b, result.Handle, blind.Handle), 0)
		near(t, overlapMeasure(t, b, result.Handle, through.Handle), 0)
		near(t, overlapMeasure(t, b, result.Handle, opposed.Handle), 0)
		near(t, overlapMeasure(t, b, result.Handle, notch.Handle), 0)
		file, err := b.exportSTEP([]string{result.Handle})
		if err != nil {
			t.Fatal(err)
		}
		raw, err := base64.StdEncoding.DecodeString(file.Base64)
		if err != nil {
			t.Fatal(err)
		}
		reread, err := b.importSTEP(string(raw))
		if err != nil {
			t.Fatal(err)
		}
		near(t, reread.GeometryAssertions.Volume, result.GeometryAssertions.Volume)
		near(t, reread.GeometryAssertions.SurfaceArea, result.GeometryAssertions.SurfaceArea)
		near(t, overlapMeasure(t, b, reread.Handle, blind.Handle), 0)
		mesh, err := b.shapes[reread.Handle].tessellate(request{})
		if err != nil {
			t.Fatal(err)
		}
		if err := mesh.ValidateSolid(); err != nil {
			t.Fatal(err)
		}
	}
	reflected := shape(t, b, map[string]any{"op": "transform", "handle": cut.Handle, "matrix": [][]float64{{-1, 0, 0, 30}, {0, 1, 0, 2}, {0, 0, 1, 0}}})
	newTool := circle(t, b, vector{25, 12, 0}, vector{0, 0, 1}, 2, 3)
	newCut, err := b.cutMany(reflected.Handle, []string{newTool.Handle})
	if err != nil {
		t.Fatal(err)
	}
	near(t, newCut.GeometryAssertions.Volume, reflected.GeometryAssertions.Volume-12*math.Pi)
	if _, err := b.exportSTEP([]string{newCut.Handle}); err != nil {
		t.Fatal(err)
	}
	crossingCenter := vector{origin[0], origin[1] + 3, 0}
	if origin == (vector{}) {
		crossingCenter[0] = 1
	}
	// A batch with a bore of radius two and depth four across the notch
	// cannot publish a partial result. Either it is refused and nothing is
	// published, or it is cut whole, as one new shape. Its volume is then
	// the stock less the through bore, the notch through its depth, and
	// through the first four units the bore less what it shares with the
	// notch. With the notch on a side face, the bore is a whole disk and
	// they share the lens of two circles three apart. With the notch at
	// the corner, the face x = 0 cuts the bore, and they share that lens
	// of two circles √10 apart less its part beyond the face, which a
	// stretch of the face, an arc of the notch and an arc of the bore
	// bound.
	notched, bore, shared := 9*math.Pi-(9*math.Acos(1./3)-math.Sqrt(8)), 4*math.Pi, 9*math.Acos(7./9)+4*math.Acos(1./3)-math.Sqrt(128)/2
	if origin == (vector{}) {
		d := math.Sqrt(10)
		y := (45 + math.Sqrt(135)) / 20
		x := 7.5 - 3*y
		beyond := 4.5*(math.Atan2(y, x)-math.Pi/2) + 2*(4*math.Pi/3-(math.Atan2(y-3, x-1)+2*math.Pi)) + (3-math.Sqrt(3)-y+3*x)/2
		notched, bore = 9*math.Pi/4, 8*math.Pi/3+math.Sqrt(3)
		shared = 9*math.Acos(15/(6*d)) + 4*math.Acos(5/(4*d)) - math.Sqrt(135)/2 - beyond
	}
	crossing := circle(t, b, crossingCenter, vector{0, 0, 1}, 2, 4)
	before := len(b.shapes)
	whole, err := b.cutMany(stock.Handle, []string{notch.Handle, through.Handle, crossing.Handle})
	if err != nil {
		if len(b.shapes) != before {
			t.Fatal("the refused batch published a shape", err)
		}
		return
	}
	near(t, whole.GeometryAssertions.Volume, 3200-32*math.Pi-notched*math.Min(depth, 8)-(bore-shared)*4)
	if len(b.shapes) != before+1 || whole.Bounds != stock.Bounds || whole.GeometryAssertions.SolidCount != 1 {
		t.Fatal("the batch across the notch did not publish one whole shape", whole)
	}
}
