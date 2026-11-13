package plaincadbridge

import (
	"context"
	"math"
	"testing"

	"plainsolid/brep/parametric"
)

func TestOpposedBlindBoresIntersectAtDifferentDepths(t *testing.T) {
	disk := 4 * math.Pi
	lens := 8*math.Pi/3 - 2*math.Sqrt(3)
	// The disks are distance 2 apart. The left and right tools are active
	// alone for three and four units and together for three units.
	removed := 13*disk - 3*lens
	wall := 7*4*math.Pi + 3*16*math.Pi/3
	area := 600 + wall - 2*lens
	for _, axis := range []int{0, 1, 2} {
		b := New()
		base := box(t, b, vector{-5, -5, -5}, vector{5, 5, 5})
		low, high, n, m := vector{}, vector{}, vector{}, vector{}
		low[axis], high[axis] = -5, 5
		low[(axis+1)%3], high[(axis+1)%3] = -1, 1
		n[axis], m[axis] = 1, -1
		first := circle(t, b, low, n, 2, 6)
		second := circle(t, b, high, m, 2, 7)
		stock := shape(t, b, map[string]any{"op": "cut", "a": base.Handle, "b": first.Handle})
		near(t, overlapMeasure(t, b, stock.Handle, second.Handle), 7*disk-3*lens)
		sequential := shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": second.Handle})
		for _, handles := range [][]string{{first.Handle, second.Handle}, {second.Handle, first.Handle}} {
			batch := shape(t, b, map[string]any{"op": "cutMany", "a": base.Handle, "tools": handles})
			near(t, batch.GeometryAssertions.Volume, sequential.GeometryAssertions.Volume)
			near(t, batch.GeometryAssertions.SurfaceArea, sequential.GeometryAssertions.SurfaceArea)
			near(t, batch.GeometryAssertions.Volume, 1000-removed)
			near(t, batch.GeometryAssertions.SurfaceArea, area)
			near(t, overlapMeasure(t, b, batch.Handle, second.Handle), 0)
			if batch.Bounds != base.Bounds || batch.GeometryAssertions.SolidCount != 1 {
				t.Fatal(batch)
			}
			body := b.shapes[batch.Handle].bore
			if body == nil || body.layered == nil || len(body.layered.FloorFaces) != 2 {
				t.Fatal("opposed floor references missing")
			}
			lowFloor, highFloor := low, high
			lowFloor[axis], highFloor[axis] = 1, -2
			for _, floor := range []struct{ origin, normal vector }{{lowFloor, m}, {highFloor, n}} {
				f, err := body.planarFace(floor.origin.vec(), floor.normal.vec())
				if err != nil {
					t.Fatal(err)
				}
				near(t, f.Area, disk-lens)
				if f.Normal != floor.normal {
					t.Fatalf("incorrect outward floor normal: %+v", f)
				}
			}
			p, err := body.solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-9})
			if err != nil {
				t.Fatal(err)
			}
			near(t, p.Volume, batch.GeometryAssertions.Volume)
			near(t, p.Area, batch.GeometryAssertions.SurfaceArea)
			mesh, err := b.shapes[batch.Handle].tessellate(request{})
			if err != nil {
				t.Fatal(err)
			}
			if err := mesh.ValidateSolid(); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestOpposedBlindBoresMeetingAtOneLevel(t *testing.T) {
	disk := 4 * math.Pi
	lens := 8*math.Pi/3 - 2*math.Sqrt(3)
	for _, offset := range []float64{0, 2} {
		for _, reverse := range []bool{false, true} {
			b := New()
			base := box(t, b, vector{-5, -5, 0}, vector{5, 5, 10})
			first := circle(t, b, vector{0, 0, 0}, vector{0, 0, 1}, 2, 4)
			second := circle(t, b, vector{offset, 0, 10}, vector{0, 0, -1}, 2, 6)
			tools := []string{first.Handle, second.Handle}
			if reverse {
				tools[0], tools[1] = tools[1], tools[0]
			}
			hole := shape(t, b, map[string]any{"op": "cutMany", "a": base.Handle, "tools": tools})
			intersection := lens
			if offset == 0 {
				intersection = disk
			}
			near(t, hole.GeometryAssertions.Volume, 1000-10*disk)
			near(t, hole.GeometryAssertions.SurfaceArea, 600+40*math.Pi-2*intersection)
			s := b.shapes[hole.Handle]
			h := s.bore.layered
			if offset == 0 {
				if len(h.FloorAreas) != 0 {
					t.Fatal("coaxial meeting tools left an internal face")
				}
			} else {
				if len(h.FloorAreas) != 2 {
					t.Fatal("opposed crescent interfaces missing")
				}
				for _, area := range h.FloorAreas {
					near(t, area, disk-intersection)
				}
				if _, err := s.bore.planarFace(vector{0, 0, 4}.vec(), vector{0, 0, 1}.vec()); err == nil {
					t.Fatal("coincident opposite floor normals gave an unambiguous reference")
				}
			}
			p, err := s.bore.solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-9})
			if err != nil {
				t.Fatal(err)
			}
			near(t, p.Volume, hole.GeometryAssertions.Volume)
			near(t, p.Area, hole.GeometryAssertions.SurfaceArea)
			m, err := s.tessellate(request{})
			if err != nil {
				t.Fatal(err)
			}
			if err := m.ValidateSolid(); err != nil {
				t.Fatal(err)
			}
		}
	}
}
