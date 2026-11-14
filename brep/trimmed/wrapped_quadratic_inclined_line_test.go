package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestWrappedQuadraticInclinedLine(t *testing.T) {
	ctx := context.Background()
	w := wrappedOverlapSupport(t)
	for _, reversed := range []bool{false, true} {
		face, e := NewWrappedQuadraticFace(ctx, w, wrappedQuadraticLoop(t), nil, WrappedQuadraticFaceOptions{SurfaceInjectivityOptions: geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}, Reversed: reversed})
		if e != nil {
			t.Fatal(e)
		}
		for _, drop := range []bool{false, true} {
			origin, direction := geom.Vec3{X: -3, Z: -1}, geom.Vec3{X: 2, Z: .25}
			want, sign := big.NewRat(5, 2), 1
			if drop {
				origin = geom.Vec3{X: 3, Z: -.25}
				direction = geom.Vec3{X: -2, Z: -.25}
				want = big.NewRat(1, 2)
				sign = -1
			}
			if reversed {
				sign = -sign
			}
			limits := [2]*big.Rat{new(big.Rat), big.NewRat(3, 1)}
			r, e := face.IntersectInclinedLine(ctx, origin, direction, limits, geom.SurfacePatchOptions{})
			if e != nil || !r.Complete || len(r.Hits) != 1 {
				t.Fatal("inclined finite seam", r, e)
			}
			h := r.Hits[0]
			if h.Parameter.Cmp(want) != 0 || h.UV[0].Sign() != 0 || h.UV[1].Cmp(big.NewRat(-3, 8)) != 0 || h.Point[2].Cmp(big.NewRat(-3, 8)) != 0 || h.CrossingSign != sign || len(h.SourceU) != 2 {
				t.Fatal("original inclined witness", h)
			}
			if q, e := face.IntersectInclinedLine(ctx, origin, direction, limits, geom.SurfacePatchOptions{MaxOperations: r.ExactOperations - 1}); e == nil || q.Complete || len(q.Hits) != 0 {
				t.Fatal("atomic work")
			}
			if q, e := face.IntersectHorizontalLine(ctx, origin, direction, limits, geom.SurfacePatchOptions{}); e == nil || q.Complete {
				t.Fatal("legacy horizontal scope changed")
			}
		}
		// Exact known oblique shadow endpoints on separate quadratic quadrants.
		for _, reverseLine := range []bool{false, true} {
			origin, direction := geom.Vec3{X: 15.0 / 8, Y: -7.0 / 8, Z: -.5}, geom.Vec3{X: 3.0 / 32, Y: 43.0 / 32, Z: .25}
			if reverseLine {
				origin = geom.Vec3{X: 63.0 / 32, Y: 15.0 / 32, Z: -.25}
				direction = geom.Vec3{X: -3.0 / 32, Y: -43.0 / 32, Z: -.25}
			}
			r, e := face.IntersectInclinedLine(ctx, origin, direction, [2]*big.Rat{new(big.Rat), big.NewRat(1, 1)}, geom.SurfacePatchOptions{})
			if e != nil || len(r.Hits) != 2 || r.Hits[0].Parameter.Sign() != 0 || r.Hits[1].Parameter.Cmp(big.NewRat(1, 1)) != 0 || r.Hits[0].BoundaryMask != 1 || r.Hits[1].BoundaryMask != 2 {
				t.Fatal("oblique shadow roots", r, e)
			}
			want := -1
			if reversed {
				want = 1
			}
			if r.Hits[0].CrossingSign != want || r.Hits[1].CrossingSign != -want {
				t.Fatal("inclined oriented normal")
			}
		}
		nonbinary, e := face.IntersectInclinedLine(ctx, geom.Vec3{}, geom.Vec3{X: 3, Z: .5}, [2]*big.Rat{new(big.Rat), big.NewRat(1, 1)}, geom.SurfacePatchOptions{})
		if e != nil || len(nonbinary.Hits) != 1 || nonbinary.Hits[0].Parameter.Cmp(big.NewRat(2, 3)) != 0 || nonbinary.Hits[0].UV[1].Cmp(big.NewRat(1, 3)) != 0 {
			t.Fatal("nonbinary exact height", nonbinary, e)
		}
		empty, e := face.IntersectInclinedLine(ctx, geom.Vec3{X: -3, Z: 3}, geom.Vec3{X: 2, Z: 1}, [2]*big.Rat{new(big.Rat), big.NewRat(3, 1)}, geom.SurfacePatchOptions{})
		if e != nil || !empty.Complete || len(empty.Hits) != 0 || empty.ExcludedByHeight != 2 {
			t.Fatal("finite support height clipping", empty, e)
		}
		for _, d := range []geom.Vec3{{Z: 1}, {Z: -1}} {
			if q, e := face.IntersectInclinedLine(ctx, geom.Vec3{X: 2}, d, [2]*big.Rat{new(big.Rat), big.NewRat(1, 1)}, geom.SurfacePatchOptions{}); e == nil || q.Complete {
				t.Fatal("vertical generator admitted")
			}
		}
		cancel, stop := context.WithCancel(ctx)
		stop()
		if q, e := face.IntersectInclinedLine(cancel, geom.Vec3{}, geom.Vec3{X: 3, Z: .5}, [2]*big.Rat{new(big.Rat), big.NewRat(1, 1)}, geom.SurfacePatchOptions{}); e == nil || q.Complete {
			t.Fatal("cancel")
		}
	}
}
func TestWrappedQuadraticInclinedLineHole(t *testing.T) {
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
	r, e := face.IntersectInclinedLine(ctx, geom.Vec3{X: -3, Z: -1.25}, geom.Vec3{X: 2, Z: .25}, [2]*big.Rat{new(big.Rat), big.NewRat(3, 1)}, geom.SurfacePatchOptions{})
	if e != nil || !r.Complete || len(r.Hits) != 0 || r.ExcludedByTrim != 2 || r.ExcludedByHeight != 0 {
		t.Fatal("inclined hole filter", r, e)
	}
}

func TestWrappedQuadraticInclinedOriginalLift(t *testing.T) {
	ctx := context.Background()
	w := wrappedOverlapSupport(t)
	loop := wrappedQuadraticLoop(t)
	for i, q := range loop {
		h := q.HomogeneousControls()
		for j := range h {
			h[j][0].Add(h[j][0], new(big.Rat).Mul(big.NewRat(4, 1), h[j][2]))
		}
		var e error
		loop[i], e = NewExactQuadraticUV(h)
		if e != nil {
			t.Fatal(e)
		}
	}
	face, e := NewWrappedQuadraticFace(ctx, w, loop, nil, WrappedQuadraticFaceOptions{SurfaceInjectivityOptions: geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}})
	if e != nil {
		t.Fatal(e)
	}
	r, e := face.IntersectInclinedLine(ctx, geom.Vec3{}, geom.Vec3{X: 3, Z: .5}, [2]*big.Rat{new(big.Rat), big.NewRat(1, 1)}, geom.SurfacePatchOptions{})
	if e != nil || len(r.Hits) != 1 || r.Hits[0].UV[0].Cmp(big.NewRat(4, 1)) != 0 || r.Hits[0].UV[1].Cmp(big.NewRat(1, 3)) != 0 || len(r.Hits[0].SourceU) != 2 {
		t.Fatal("inclined original lifted occurrence", r, e)
	}
}
