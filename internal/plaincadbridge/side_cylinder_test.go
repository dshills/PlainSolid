package plaincadbridge

import (
	"context"
	"encoding/base64"
	"math"
	"testing"

	"plainsolid/brep/parametric"
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
	for _, body := range []Shape{cut, moved} {
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
		before := len(b.shapes)
		if _, err := b.cutMany(blind.Handle, []string{upper.Handle}); err == nil || len(b.shapes) != before {
			t.Fatal("a disconnected coaxial notch must be rejected atomically", err)
		}
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
	crossingCenter := vector{origin[0], origin[1] + 3, 0}
	if origin == (vector{}) {
		crossingCenter[0] = 1
	}
	crossing := circle(t, b, crossingCenter, vector{0, 0, 1}, 2, 4)
	before := len(b.shapes)
	if _, err := b.cutMany(stock.Handle, []string{notch.Handle, through.Handle, crossing.Handle}); err == nil || len(b.shapes) != before {
		t.Fatal("intersecting notch batch did not reject atomically", err)
	}
}
