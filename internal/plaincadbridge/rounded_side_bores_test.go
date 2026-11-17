package plaincadbridge

import (
	"context"
	"fmt"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func galleryHeaderStock(t *testing.T, b *Bridge, x float64) Shape {
	t.Helper()
	s, err := b.mixedExtrude(mixedProfileRequest{
		Kind: "roundedRectangle", Origin: &vector{0, 0, 6},
		Normal: &vector{0, 0, 1}, XDirection: &vector{1, 0, 0},
		Min: []number{number(x - 10), -50}, Max: []number{number(x + 10), 50},
		Radii: []number{3, 3, 3, 3}, Height: 54,
	})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestGalleryHeatExchangerRoundedHeaderBlindCuts(t *testing.T) {
	// PlainCAD docs/examples/09-heat-exchanger.pcaddoc, Tube sockets
	// features _hole_3 and _hole_5. The negative extrusion still creates
	// a +X cylinder, shifting its start plane to the interior floor.
	for _, length := range []float64{148, 152} {
		for _, side := range []float64{-1, 1} {
			t.Run(fmt.Sprintf("length_%g_side_%g", length, side), func(t *testing.T) {
				b := New()
				entry := side * (length/2 - 5)
				floor := entry + side*18
				base := galleryHeaderStock(t, b, entry+side*10)
				var tools []string
				var removed, walls float64
				for _, y := range []float64{-35, -12, 12, 35} {
					// Match the adapter's radius recovered from the
					// original center and the two sketch-bound endpoints.
					radius := ((y + 4.4) - (y - 4.4)) / 2
					for _, z := range []float64{18, 30, 42, 54} {
						start := vector{min(entry, floor), y, z}
						tool := circle(t, b, start, vector{1, 0, 0}, radius, 18)
						for _, order := range [][2]string{{base.Handle, tool.Handle}, {tool.Handle, base.Handle}} {
							near(t, overlapMeasure(t, b, order[0], order[1]), tool.GeometryAssertions.Volume)
						}
						removed += tool.GeometryAssertions.Volume
						walls += 2 * math.Pi * radius * 18
						tools = append(tools, tool.Handle)
					}
				}
				first := shape(t, b, map[string]any{"op": "cut", "a": base.Handle, "b": tools[0]})
				single := b.shapes[tools[0]].result.GeometryAssertions.Volume
				near(t, first.GeometryAssertions.Volume, base.GeometryAssertions.Volume-single)
				// A repeated cutter in the atomic pattern is redundant.
				cut := shape(t, b, map[string]any{"op": "cutMany", "a": base.Handle, "tools": append(tools, tools[0])})
				if cut.Bounds != base.Bounds || !cut.GeometryAssertions.Valid || cut.GeometryAssertions.SolidCount != 1 {
					t.Fatal("cut changed stock extrema or shell count", cut, base)
				}
				near(t, cut.GeometryAssertions.Volume, base.GeometryAssertions.Volume-removed)
				near(t, cut.GeometryAssertions.SurfaceArea, base.GeometryAssertions.SurfaceArea+walls)
				// Independent OCCT measurements in PlainCAD's VALIDATION.json.
				near(t, cut.GeometryAssertions.Volume, 90066.29870287704)
				near(t, cut.GeometryAssertions.SurfaceArea, 24628.477108785657)

				body := b.shapes[cut.Handle]
				floors, openings := 0, 0
				for _, face := range body.analytic.solid.Faces() {
					plane, ok := face.Surface.(geom.PlaneSurface)
					if !ok || math.Abs(plane.Frame().ZAxis().X) != 1 {
						continue
					}
					switch plane.Frame().Origin().X {
					case entry:
						openings += len(face.Holes)
					case floor:
						floors++
					}
				}
				if openings != 16 || floors != 16 {
					t.Fatal("lost an original blind opening or 18 mm floor", openings, floors)
				}
				actual, err := body.analytic.solid.MassProperties(context.Background(), parametric.IntegrationOptions{})
				if err != nil {
					t.Fatal(err)
				}
				near(t, actual.Volume, cut.GeometryAssertions.Volume)
				near(t, actual.Area, cut.GeometryAssertions.SurfaceArea)
				display, err := body.tessellate(request{LinearDeflection: optionalNumber{value: .5, set: true}})
				if err != nil {
					t.Fatal(err)
				}
				if err := display.ValidateSolid(); err != nil {
					t.Fatal(err)
				}
				if math.Abs(display.SignedVolume()/actual.Volume-1) > .002 {
					t.Fatal("display changed the blind-cut material volume or orientation", display.SignedVolume(), actual.Volume)
				}
			})
		}
	}
}

func TestRoundedHeaderSideBlindCutRejectsUnprovedToolSet(t *testing.T) {
	b := New()
	base := galleryHeaderStock(t, b, 79)
	good := circle(t, b, vector{69, 0, 18}, vector{1, 0, 0}, 4.4, 18)
	for _, bad := range []Shape{
		circle(t, b, vector{70, 12, 18}, vector{1, 0, 0}, 4.4, 18), // interior start
		circle(t, b, vector{69, 46, 18}, vector{1, 0, 0}, 4.4, 18), // crosses rounded wall
		circle(t, b, vector{69, 12, 18}, vector{1, 0, 0}, 4.4, 20), // touches opposite side
		circle(t, b, vector{69, 3, 18}, vector{1, 0, 0}, 4.4, 18),  // overlaps the other tool
	} {
		handles := len(b.shapes)
		_, err := b.cutMany(base.Handle, []string{good.Handle, bad.Handle})
		if err == nil || len(b.shapes) != handles || b.shapes[base.Handle].result != base {
			t.Fatal("unproved batch changed the base or published a result", err)
		}
	}
}
