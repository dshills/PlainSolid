//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"testing"
)

func TestWrappedQuadraticHorizontalLine(t *testing.T) {
	ctx := context.Background()
	w := wrappedOverlapSupport(t)
	loop := wrappedQuadraticLoop(t)
	for _, shift := range []int64{0, 1} {
		original := append([]ExactQuadraticUV(nil), loop...)
		if shift != 0 {
			for i, q := range original {
				h := q.HomogeneousControls()
				for j := range h {
					h[j][0].Add(h[j][0], new(big.Rat).Mul(big.NewRat(4*shift, 1), h[j][2]))
				}
				var e error
				original[i], e = NewExactQuadraticUV(h)
				if e != nil {
					t.Fatal(e)
				}
			}
		}
		for _, reverseFace := range []bool{false, true} {
			face, e := NewWrappedQuadraticFace(ctx, w, original, nil, WrappedQuadraticFaceOptions{SurfaceInjectivityOptions: geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}, Reversed: reverseFace})
			if e != nil {
				t.Fatal(e)
			}
			for _, reverseLine := range []bool{false, true} {
				origin, direction := geom.Vec3{X: -3}, geom.Vec3{X: 2}
				want := big.NewRat(5, 2)
				sign := 1
				if reverseLine {
					origin.X = 3
					direction.X = -2
					want = big.NewRat(1, 2)
					sign = -1
				}
				if reverseFace {
					sign = -sign
				}
				limits := [2]*big.Rat{new(big.Rat), big.NewRat(3, 1)}
				r, e := face.IntersectHorizontalLine(ctx, origin, direction, limits, geom.SurfacePatchOptions{})
				if e != nil || !r.Complete || len(r.Hits) != 1 || r.ExcludedByTrim != 1 {
					t.Fatal("finite original line", r, e)
				}
				h := r.Hits[0]
				if h.Parameter.Cmp(want) != 0 || h.Point[0].Cmp(big.NewRat(2, 1)) != 0 || h.UV[0].Cmp(big.NewRat(4*shift, 1)) != 0 || h.CrossingSign != sign || h.Location != trimvalidate.Inside || len(h.SourceU) != 2 || h.SourceU[0].Sign() != 0 || h.SourceU[1].Cmp(big.NewRat(4, 1)) != 0 {
					t.Fatal("lost signed/periodic identity", h)
				}
				bound, e := face.IntersectHorizontalLine(ctx, origin, direction, [2]*big.Rat{want, want}, geom.SurfacePatchOptions{})
				if e != nil || len(bound.Hits) != 1 || bound.Hits[0].BoundaryMask != 3 {
					t.Fatal("finite exact endpoint", e)
				}
				if q, e := face.IntersectHorizontalLine(ctx, origin, direction, limits, geom.SurfacePatchOptions{MaxOperations: r.ExactOperations - 1}); e == nil || q.Complete || len(q.Hits) != 0 {
					t.Fatal("partial budget")
				}
			}
		}
	}
	face, e := NewWrappedQuadraticFace(ctx, w, loop, nil, WrappedQuadraticFaceOptions{SurfaceInjectivityOptions: geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}})
	if e != nil {
		t.Fatal(e)
	}
	limits := [2]*big.Rat{new(big.Rat), big.NewRat(3, 1)}
	two, e := face.IntersectHorizontalLine(ctx, geom.Vec3{X: 15.0 / 8, Y: -2, Z: -.5}, geom.Vec3{Y: 1}, [2]*big.Rat{new(big.Rat), big.NewRat(4, 1)}, geom.SurfacePatchOptions{})
	if e != nil || len(two.Hits) != 2 || two.Hits[0].Parameter.Cmp(big.NewRat(9, 8)) != 0 || two.Hits[1].Parameter.Cmp(big.NewRat(23, 8)) != 0 || two.Hits[0].CrossingSign != -1 || two.Hits[1].CrossingSign != 1 {
		t.Fatal("two ordered off-seam roots", two, e)
	}
	boundary, e := face.IntersectHorizontalLine(ctx, geom.Vec3{X: -3, Z: .5}, geom.Vec3{X: 2}, limits, geom.SurfacePatchOptions{})
	if e != nil || len(boundary.Hits) != 1 || boundary.Hits[0].Location != trimvalidate.Boundary {
		t.Fatal("curved trim boundary", e)
	}
	for _, line := range [][2]geom.Vec3{{{X: -3, Y: .5}, {X: 2}}, {{X: 2, Y: -3}, {Y: 2}}} {
		r, e := face.IntersectHorizontalLine(ctx, line[0], line[1], limits, geom.SurfacePatchOptions{})
		if e == nil || r.Complete || len(r.Hits) != 0 {
			t.Fatal("irrational or grazing accepted", e)
		}
	}
	empty, e := face.IntersectHorizontalLine(ctx, geom.Vec3{X: -3, Z: 9}, geom.Vec3{X: 2}, limits, geom.SurfacePatchOptions{})
	if e != nil || !empty.Complete || len(empty.Hits) != 0 {
		t.Fatal("height exclusion", e)
	}
	cancel, stop := context.WithCancel(ctx)
	stop()
	if r, e := face.IntersectHorizontalLine(cancel, geom.Vec3{X: -3}, geom.Vec3{X: 2}, limits, geom.SurfacePatchOptions{}); e == nil || r.Complete {
		t.Fatal("cancel")
	}
}
func TestWrappedQuadraticHorizontalLineHole(t *testing.T) {
	ctx := context.Background()
	w := wrappedOverlapSupport(t)
	pts := [][2]*big.Rat{{big.NewRat(-1, 16), big.NewRat(-3, 4)}, {big.NewRat(-1, 16), big.NewRat(-1, 2)}, {big.NewRat(1, 16), big.NewRat(-1, 2)}, {big.NewRat(1, 16), big.NewRat(-3, 4)}}
	var hole []ExactQuadraticUV
	for i, p := range pts {
		q, e := quadraticLine(p, pts[(i+1)%4])
		if e != nil {
			t.Fatal(e)
		}
		hole = append(hole, q)
	}
	face, e := NewWrappedQuadraticFace(ctx, w, wrappedQuadraticLoop(t), [][]ExactQuadraticUV{hole}, WrappedQuadraticFaceOptions{SurfaceInjectivityOptions: geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}})
	if e != nil {
		t.Fatal(e)
	}
	r, e := face.IntersectHorizontalLine(ctx, geom.Vec3{X: -3, Z: -.625}, geom.Vec3{X: 2}, [2]*big.Rat{new(big.Rat), big.NewRat(3, 1)}, geom.SurfacePatchOptions{})
	if e != nil || !r.Complete || len(r.Hits) != 0 || r.ExcludedByTrim != 2 {
		t.Fatal("hole did not filter exact support root", r, e)
	}
}
