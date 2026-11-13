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

func TestPlainCADSideBlindNotchSTEPRoundTrip(t *testing.T) {
	b := New()
	stock := box(t, b, vector{}, vector{20, 20, 8})
	tool := circle(t, b, vector{10, 1, 0}, vector{0, 0, 1}, 3, 6)
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
