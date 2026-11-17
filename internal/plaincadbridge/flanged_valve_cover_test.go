package plaincadbridge

import (
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/brep/parametric"
)

var valveBoltCenters = [...][2]float64{{-32, -22}, {32, 22}, {-32, 22}, {32, -22}}

func valveAnnulus(t *testing.T, b *Bridge, z, outer, inner, height float64) Shape {
	t.Helper()
	return shape(t, b, map[string]any{"op": "mixedExtrude", "profile": map[string]any{
		"kind": "disk", "origin": vector{0, 0, z}, "normal": vector{0, 0, 1}, "xDirection": vector{1, 0, 0},
		"center": [2]float64{}, "height": height, "radius": outer,
		"circles": []map[string]any{{"center": [2]float64{}, "radius": inner}},
	}})
}

func TestFlangedValveCoverThroughPattern(t *testing.T) {
	for _, height := range []float64{32, 36} {
		b := New()
		cover := valveAnnulus(t, b, height+1, 46, 16, 6)
		tools := make([]string, 0, len(valveBoltCenters))
		for _, xy := range valveBoltCenters {
			// The application's through-all length covers the padded native
			// bounds. Its endpoint need not be representable before clipping.
			tool := circle(t, b, vector{xy[0], xy[1], height + 1}, vector{0, 0, 1}, 3.2, 6.0000001)
			volume, err := b.overlapVolume(cover.Handle, tool.Handle)
			if err != nil {
				t.Fatal(err)
			}
			near(t, volume.(map[string]float64)["volume"], math.Pi*3.2*3.2*6)
			tools = append(tools, tool.Handle)
		}
		out := shape(t, b, map[string]any{"op": "cutMany", "a": cover.Handle, "tools": tools})
		p := b.shapes[out.Handle].analytic.stepDisk
		if p == nil || len(p.Holes) != 5 || len(p.Blind) != 0 || p.Holes[0].Radius != 16 {
			t.Fatal("cover loses original center bore or four through bores", p)
		}
		for i, center := range valveBoltCenters {
			hole := p.Holes[i+1]
			if hole.Center.X != center[0] || hole.Center.Y != center[1] || hole.Radius != 3.2 {
				t.Fatal("changed the authored cover bolt pattern", hole)
			}
		}
		wantVolume := math.Pi * (46*46 - 16*16 - 4*3.2*3.2) * 6
		wantArea := 2 * math.Pi * (46*46 - 16*16 - 4*3.2*3.2 + (46+16+4*3.2)*6)
		near(t, out.GeometryAssertions.Volume, wantVolume)
		near(t, out.GeometryAssertions.SurfaceArea, wantArea)
		body := b.shapes[out.Handle].analytic
		integral, err := body.solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
		if err != nil || math.Abs(integral.Volume/wantVolume-1) > 1e-8 || math.Abs(integral.Area/wantArea-1) > 1e-8 {
			t.Fatal("cover source topology mass", integral.Volume, integral.Area, wantVolume, wantArea, err)
		}
		if m, err := body.solid.Tessellate(context.Background(), parametric.TessellationOptions{
			Deflection: .05, Angle: .1, MaxTriangles: 16,
		}); !errors.Is(err, parametric.ErrBudget) || len(m.Triangles) != 0 {
			t.Fatal("cover publishes a partial budget-limited mesh", len(m.Triangles), err)
		}
		m, err := b.shapes[out.Handle].tessellate(request{})
		if err != nil || m.ValidateSolid() != nil || math.Abs(m.SignedVolume()/wantVolume-1) > .02 {
			t.Fatal("cover display lost positive closed material", m.SignedVolume(), wantVolume, err)
		}
	}
}

func TestCircularStockRetainedBoreStationMustBeRepresentable(t *testing.T) {
	b := New()
	cover := valveAnnulus(t, b, 33, 46, 16, 6)
	// 2 + nextafter(1,+Inf) is a nonrepresentable interior station.
	tool := circle(t, b, vector{32, 22, 35}, vector{0, 0, 1}, 3.2, math.Nextafter(1, math.Inf(1)))
	if _, ok := circularStockRecipe(b.shapes[cover.Handle]); !ok {
		t.Fatal("lost proven source cover")
	}
	before, next := len(b.shapes), b.next
	if result, handled, err := b.cutCircularStockBores(b.shapes[cover.Handle], []stored{b.shapes[tool.Handle]}); !handled ||
		err == nil || result.Handle != "" || len(b.shapes) != before || b.next != next {
		t.Fatal("published a rounded interior terminal station", result, handled, err)
	}
}
