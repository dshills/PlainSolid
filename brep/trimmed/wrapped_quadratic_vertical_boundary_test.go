package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestWrappedVerticalBoundaryIntervals(t *testing.T) {
	ctx := context.Background()
	face, e := NewWrappedQuadraticFace(ctx, wrappedOverlapSupport(t), wrappedQuadraticLoop(t), nil, WrappedQuadraticFaceOptions{SurfaceInjectivityOptions: geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}})
	if e != nil {
		t.Fatal(e)
	}
	singleton, e := face.ClassifyVerticalFiber(ctx, geom.Vec3{X: 15.0 / 8, Y: 7.0 / 8}, geom.Vec3{Z: 1}, big.NewRat(1, 4), [2]*big.Rat{big.NewRat(-1, 2), big.NewRat(-1, 2)}, WrappedVerticalFiberOptions{AllowBoundaryIntervals: true})
	if e != nil || !singleton.Complete || len(singleton.Events) != 1 || len(singleton.Events[0].Sources) != 1 || len(singleton.BoundaryIntervals) != 0 || len(singleton.Intervals) != 0 {
		t.Fatal("singleton", singleton, e)
	}
	for _, sign := range []int64{1, -1} {
		origin := geom.Vec3{X: 15.0 / 8, Y: 7.0 / 8}
		dir := geom.Vec3{Z: float64(sign)}
		lo, hi := big.NewRat(-2, 3), big.NewRat(-1, 3)
		if sign < 0 {
			lo, hi = big.NewRat(1, 3), big.NewRat(2, 3)
		}
		limits := [2]*big.Rat{lo, hi}
		if q, e := face.ClassifyVerticalFiber(ctx, origin, dir, big.NewRat(1, 4), limits, WrappedVerticalFiberOptions{}); e == nil || q.Complete {
			t.Fatal("legacy coincidence admitted")
		}
		r, e := face.ClassifyVerticalFiber(ctx, origin, dir, big.NewRat(1, 4), limits, WrappedVerticalFiberOptions{AllowBoundaryIntervals: true})
		if e != nil || !r.Complete || len(r.Intervals) != 0 || len(r.BoundaryIntervals) != 1 {
			t.Fatal(r, e)
		}
		b := r.BoundaryIntervals[0]
		if b.Start.Cmp(lo) != 0 || b.End.Cmp(hi) != 0 || len(b.Sources) != 1 || b.Sources[0].Loop != 0 || b.Sources[0].Edge != 1 {
			t.Fatal(b)
		}
		for j, event := range r.Events {
			want := new(big.Rat).Add(big.NewRat(1, 1), new(big.Rat).Mul(big.NewRat(sign, 1), event.Parameter))
			if b.Sources[0].Parameters[j].Cmp(want) != 0 || len(event.Sources) != 1 || event.Sources[0].Parameter.Cmp(want) != 0 {
				t.Fatal("original clipped ancestry", event, b)
			}
		}
		low := WrappedVerticalFiberOptions{AllowBoundaryIntervals: true, SurfacePatchOptions: geom.SurfacePatchOptions{MaxOperations: r.ExactOperations - 1}}
		if q, e := face.ClassifyVerticalFiber(ctx, origin, dir, big.NewRat(1, 4), limits, low); e == nil || q.Complete || len(q.BoundaryIntervals) != 0 {
			t.Fatal("non atomic budget")
		}
	}
}

func TestWrappedVerticalBoundaryHole(t *testing.T) {
	ctx := context.Background()
	pts := [][2]*big.Rat{{big.NewRat(-1, 16), big.NewRat(-3, 4)}, {big.NewRat(-1, 16), big.NewRat(-1, 2)}, {big.NewRat(1, 16), big.NewRat(-1, 2)}, {big.NewRat(1, 16), big.NewRat(-3, 4)}}
	var hole []ExactQuadraticUV
	for i, p := range pts {
		q, e := quadraticLine(p, pts[(i+1)%4])
		if e != nil {
			t.Fatal(e)
		}
		hole = append(hole, q)
	}
	face, e := NewWrappedQuadraticFace(ctx, wrappedOverlapSupport(t), wrappedQuadraticLoop(t), [][]ExactQuadraticUV{hole}, WrappedQuadraticFaceOptions{SurfaceInjectivityOptions: geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}})
	if e != nil {
		t.Fatal(e)
	}
	r, e := face.ClassifyVerticalFiber(ctx, geom.Vec3{X: 255.0 / 128, Y: 31.0 / 128}, geom.Vec3{Z: 1}, big.NewRat(1, 16), [2]*big.Rat{big.NewRat(-2, 1), big.NewRat(1, 1)}, WrappedVerticalFiberOptions{AllowBoundaryIntervals: true})
	if e != nil || !r.Complete || len(r.Intervals) != 2 || len(r.BoundaryIntervals) != 1 {
		t.Fatal(r, e)
	}
	b := r.BoundaryIntervals[0]
	if b.Start.Cmp(big.NewRat(-3, 4)) != 0 || b.End.Cmp(big.NewRat(-1, 2)) != 0 || len(b.Sources) != 1 || b.Sources[0].Loop != 1 || b.Sources[0].Edge != 2 || b.Sources[0].Parameters[0].Cmp(big.NewRat(1, 1)) != 0 || b.Sources[0].Parameters[1].Sign() != 0 {
		t.Fatal(b)
	}
}

func TestWrappedVerticalBoundaryUnsupportedSpeed(t *testing.T) {
	ctx := context.Background()
	loop := wrappedQuadraticLoop(t)
	h := loop[1].HomogeneousControls()
	// Same straight geometric side, but positive nonconstant homogeneous weights
	// make the original parameter speed nonlinear. The narrow opt-in rejects it.
	for axis := 0; axis < 3; axis++ {
		h[1][axis].Mul(h[1][axis], big.NewRat(2, 1))
	}
	q, e := NewExactQuadraticUV(h)
	if e != nil {
		t.Fatal(e)
	}
	loop[1] = q
	face, e := NewWrappedQuadraticFace(ctx, wrappedOverlapSupport(t), loop, nil, WrappedQuadraticFaceOptions{SurfaceInjectivityOptions: geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}})
	if e != nil {
		t.Fatal(e)
	}
	r, e := face.ClassifyVerticalFiber(ctx, geom.Vec3{X: 15.0 / 8, Y: 7.0 / 8}, geom.Vec3{Z: 1}, big.NewRat(1, 4), [2]*big.Rat{big.NewRat(-1, 1), new(big.Rat)}, WrappedVerticalFiberOptions{AllowBoundaryIntervals: true})
	if e == nil || r.Complete || len(r.Events) != 0 || len(r.BoundaryIntervals) != 0 {
		t.Fatal("nonlinear map accepted", r, e)
	}
}
