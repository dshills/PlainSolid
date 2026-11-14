package trimmed

import (
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

func TestAlgebraicGraphCapCertifiedDisplay(t *testing.T) {
	ctx := context.Background()
	stock := makeWarpedSquareRationalGraphTube(t, 1./8.0)
	section := [3]geom.Vec3{{Z: .5}, {X: 1, Z: .5}, {Y: 1, Z: .5}}
	sectionOptions := RationalGraphShellSectionOptions{ComposeSectionCurves: true}
	for _, tc := range []struct {
		y, area float64
		holes   int
	}{
		{11. / 8, 14./3 + math.Sqrt(3)/4, 0}, // opens the hole and retains its exact irrational terminal
		{1. / 32, 1. / 96, 0},                // narrow curved strip, below even the void
		{1. / 16, 8 - math.Sqrt(.5)/24, 1},   // an untouched full hole, one new outer seam
	} {
		cut := [3]geom.Vec3{{Y: tc.y}, {Y: tc.y, Z: 1}, {X: 1, Y: tc.y}}
		keep := tc.y != 1./32
		clip, e := stock.ClipAlgebraicGraphCap(ctx, section, cut, keep, sectionOptions, RationalGraphShellRegionOptions{})
		if e != nil {
			t.Fatal(e)
		}
		opts := AlgebraicGraphCapMeshOptions{Display: geom.ComposedCurvePolylineOptions{Deflection: .004}}
		view, e := clip.TessellateCertified(ctx, opts)
		if e != nil || len(view.Regions) != 1 || len(view.Regions[0].Holes) != tc.holes ||
			view.Polygon.Status != trimvalidate.Certified || view.MaximumFillError <= 0 || view.MaximumFillError > opts.Display.Deflection {
			t.Fatal("certified exact clipped fill", tc, e)
		}
		area := float64(0)
		type pair struct{ a, b int }
		uses := map[pair]int{}
		for _, tr := range view.Mesh.Triangles {
			a, b, c := view.Mesh.Vertices[tr[0]], view.Mesh.Vertices[tr[1]], view.Mesh.Vertices[tr[2]]
			q := b.Sub(a).Cross(c.Sub(a)).Z / 2
			if q <= 0 || a.Z != .5 || b.Z != .5 || c.Z != .5 {
				t.Fatal("exact represented cap plane and winding", tr, q)
			}
			area += q
			for i, v := range tr {
				u := tr[(i+1)%3]
				uses[pair{min(v, u), max(v, u)}]++
			}
		}
		if math.Abs(area-tc.area) > .025 {
			t.Fatal("mesh area vs independent sheared integral", tc, area)
		}
		for wi, wire := range view.Boundary.Paths {
			for j, path := range wire {
				next := wire[(j+1)%len(wire)]
				if path[len(path)-1] != next[0] || !view.Boundary.Chords[wi][j].Complete {
					t.Fatal("gap in exact endpoint ownership", wi, j)
				}
				for k := 1; k < len(path); k++ {
					a, b := path[k-1], path[k]
					key := pair{min(a, b), max(a, b)}
					if uses[key] != 1 {
						t.Fatal("boundary chord does not appear exactly once", key)
					}
					delete(uses, key)
				}
			}
		}
		for edge, count := range uses {
			if count != 2 {
				t.Fatal("interior triangulation edge is not two-sided", edge)
			}
		}
		opts.MaxPredicates = 1
		if failed, e := clip.TessellateCertified(ctx, opts); !errors.Is(e, geom.ErrConvergence) || len(failed.Mesh.Triangles)+len(failed.Boundary.Vertices) != 0 {
			t.Fatal("failed triangulation must not publish a partial boundary", e)
		}
	}
}
