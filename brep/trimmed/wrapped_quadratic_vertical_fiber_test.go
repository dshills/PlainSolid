//go:build research

package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"testing"
)

func TestWrappedVerticalFiberHoles(t *testing.T) {
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
	for _, drop := range []bool{false, true} {
		origin, direction := geom.Vec3{X: 2, Z: -2}, geom.Vec3{Z: 2}
		want := [][2]*big.Rat{{big.NewRat(1, 2), big.NewRat(5, 8)}, {big.NewRat(3, 4), big.NewRat(5, 4)}}
		if drop {
			origin.Z = 2
			direction.Z = -2
			want = [][2]*big.Rat{{big.NewRat(3, 4), big.NewRat(5, 4)}, {big.NewRat(11, 8), big.NewRat(3, 2)}}
		}
		limits := [2]*big.Rat{new(big.Rat), big.NewRat(2, 1)}
		r, e := face.ClassifyVerticalFiber(ctx, origin, direction, big.NewRat(4, 1), limits, WrappedVerticalFiberOptions{})
		if e != nil || !r.Complete || len(r.Intervals) != 2 || r.FaceU.Sign() != 0 || r.PeriodShift.Cmp(big.NewInt(-1)) != 0 || r.SuppliedU.Cmp(big.NewRat(4, 1)) != 0 {
			t.Fatal("fiber material", r, e)
		}
		for i, interval := range r.Intervals {
			if interval.Start.Cmp(want[i][0]) != 0 || interval.End.Cmp(want[i][1]) != 0 || r.Events[interval.FirstEvent].Location != trimvalidate.Boundary || r.Events[interval.LastEvent].Location != trimvalidate.Boundary {
				t.Fatal("hole parity", interval)
			}
		}
		found := false
		for _, event := range r.Events {
			if event.Point[2].Cmp(big.NewRat(1, 2)) == 0 {
				found = true
				if len(event.Sources) != 1 || event.Sources[0].Loop != 0 || event.Sources[0].Edge != 2 || event.Sources[0].Parameter.Cmp(big.NewRat(2, 3)) != 0 {
					t.Fatal("lost original curved edge event")
				}
			}
		}
		if !found {
			t.Fatal("missing top curve event")
		}
		low := WrappedVerticalFiberOptions{SurfacePatchOptions: geom.SurfacePatchOptions{MaxOperations: r.ExactOperations - 1}}
		if q, e := face.ClassifyVerticalFiber(ctx, origin, direction, new(big.Rat), limits, low); e == nil || q.Complete || len(q.Events) != 0 {
			t.Fatal("atomic work")
		}
		if q, e := face.ClassifyVerticalFiber(ctx, origin, direction, new(big.Rat), limits, WrappedVerticalFiberOptions{MaxEvents: 2}); e == nil || q.Complete || len(q.Events) != 0 {
			t.Fatal("atomic events")
		}
	}
	clipped, e := face.ClassifyVerticalFiber(ctx, geom.Vec3{X: 2, Z: -2}, geom.Vec3{Z: 2}, new(big.Rat), [2]*big.Rat{big.NewRat(4, 5), big.NewRat(1, 1)}, WrappedVerticalFiberOptions{})
	if e != nil || len(clipped.Intervals) != 1 || len(clipped.Events) != 2 || clipped.Events[0].Location != trimvalidate.Inside || clipped.Events[0].FiniteMask != 1 || clipped.Events[1].FiniteMask != 2 {
		t.Fatal("finite interior clip", clipped, e)
	}
	for _, u := range []*big.Rat{big.NewRat(2, 1), new(big.Rat)} {
		origin := geom.Vec3{X: 2}
		if u.Sign() == 0 {
			origin.X = math.Nextafter(2, 3)
		}
		if q, e := face.ClassifyVerticalFiber(ctx, origin, geom.Vec3{Z: 1}, u, [2]*big.Rat{new(big.Rat), big.NewRat(1, 1)}, WrappedVerticalFiberOptions{}); e == nil || q.Complete {
			t.Fatal("false physical XY witness")
		}
	}
	cancel, stop := context.WithCancel(ctx)
	stop()
	if q, e := face.ClassifyVerticalFiber(cancel, geom.Vec3{X: 2}, geom.Vec3{Z: 1}, new(big.Rat), [2]*big.Rat{new(big.Rat), big.NewRat(1, 1)}, WrappedVerticalFiberOptions{}); e == nil || q.Complete {
		t.Fatal("cancel")
	}
	// The line is on a whole original trim side; no isolated event list is faked.
	if q, e := face.ClassifyVerticalFiber(ctx, geom.Vec3{X: 15.0 / 8, Y: 7.0 / 8}, geom.Vec3{Z: 1}, big.NewRat(1, 4), [2]*big.Rat{big.NewRat(-2, 1), big.NewRat(2, 1)}, WrappedVerticalFiberOptions{}); e == nil || q.Complete {
		t.Fatal("coincident trim span accepted")
	}
}
func TestWrappedVerticalFiberStationaryAndSharedVertex(t *testing.T) {
	ctx := context.Background()
	w := wrappedOverlapSupport(t)
	points := [][2]*big.Rat{{big.NewRat(-1, 4), big.NewRat(-1, 1)}, {big.NewRat(1, 8), big.NewRat(-1, 1)}, {big.NewRat(1, 8), big.NewRat(1, 1)}, {big.NewRat(-1, 4), big.NewRat(1, 1)}}
	var loop []ExactQuadraticUV
	for i, p := range points {
		q, e := quadraticLine(p, points[(i+1)%len(points)])
		if e != nil {
			t.Fatal(e)
		}
		loop = append(loop, q)
	}
	var e error
	loop[1], e = NewExactQuadraticUV([3][3]*big.Rat{{big.NewRat(1, 8), big.NewRat(-1, 1), big.NewRat(1, 1)}, {big.NewRat(1, 2), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(1, 8), big.NewRat(1, 1), big.NewRat(1, 1)}})
	if e != nil {
		t.Fatal(e)
	}
	face, e := NewWrappedQuadraticFace(ctx, w, loop, nil, WrappedQuadraticFaceOptions{SurfaceInjectivityOptions: geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}})
	if e != nil {
		t.Fatal(e)
	}
	touch, e := face.ClassifyVerticalFiber(ctx, geom.Vec3{X: 231.0 / 128, Y: 135.0 / 128, Z: -2}, geom.Vec3{Z: 1}, big.NewRat(5, 16), [2]*big.Rat{new(big.Rat), big.NewRat(4, 1)}, WrappedVerticalFiberOptions{})
	if e != nil || !touch.Complete || len(touch.Intervals) != 0 {
		t.Fatal("isolated tangent", touch, e)
	}
	found := false
	for _, event := range touch.Events {
		if event.Location == trimvalidate.Boundary {
			if found || event.Parameter.Cmp(big.NewRat(2, 1)) != 0 || len(event.Sources) != 1 || event.Sources[0].Parameter.Cmp(big.NewRat(1, 2)) != 0 {
				t.Fatal("stationary event")
			}
			found = true
		}
	}
	if !found {
		t.Fatal("lost isolated touch")
	}
	if q, e := face.ClassifyVerticalFiber(ctx, geom.Vec3{X: 15.0 / 8, Y: 7.0 / 8}, geom.Vec3{Z: 1}, big.NewRat(1, 4), [2]*big.Rat{big.NewRat(-2, 1), big.NewRat(2, 1)}, WrappedVerticalFiberOptions{}); e == nil || q.Complete || len(q.Events) != 0 {
		t.Fatal("irrational trim parameters rounded")
	}
	points = [][2]*big.Rat{{big.NewRat(-1, 4), big.NewRat(-1, 1)}, {big.NewRat(1, 4), big.NewRat(-1, 1)}, {new(big.Rat), big.NewRat(1, 1)}}
	loop = nil
	for i, p := range points {
		q, e := quadraticLine(p, points[(i+1)%3])
		if e != nil {
			t.Fatal(e)
		}
		loop = append(loop, q)
	}
	face, e = NewWrappedQuadraticFace(ctx, w, loop, nil, WrappedQuadraticFaceOptions{SurfaceInjectivityOptions: geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}})
	if e != nil {
		t.Fatal(e)
	}
	r, e := face.ClassifyVerticalFiber(ctx, geom.Vec3{X: 2}, geom.Vec3{Z: 1}, big.NewRat(8, 1), [2]*big.Rat{big.NewRat(-2, 1), big.NewRat(2, 1)}, WrappedVerticalFiberOptions{})
	if e != nil || len(r.Intervals) != 1 {
		t.Fatal("shared vertex fiber", e)
	}
	event := r.Events[r.Intervals[0].LastEvent]
	if event.Parameter.Cmp(big.NewRat(1, 1)) != 0 || len(event.Sources) != 2 {
		t.Fatal("shared vertex ownership not retained")
	}
}

func TestWrappedVerticalFiberHoleKiss(t *testing.T) {
	ctx := context.Background()
	w := wrappedOverlapSupport(t)
	p := [][2]*big.Rat{{new(big.Rat), big.NewRat(-1, 2)}, {big.NewRat(1, 8), big.NewRat(-1, 4)}, {big.NewRat(1, 8), big.NewRat(-3, 4)}}
	var hole []ExactQuadraticUV
	for i, a := range p {
		q, e := quadraticLine(a, p[(i+1)%3])
		if e != nil {
			t.Fatal(e)
		}
		hole = append(hole, q)
	}
	face, e := NewWrappedQuadraticFace(ctx, w, wrappedQuadraticLoop(t), [][]ExactQuadraticUV{hole}, WrappedQuadraticFaceOptions{SurfaceInjectivityOptions: geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}})
	if e != nil {
		t.Fatal(e)
	}
	r, e := face.ClassifyVerticalFiber(ctx, geom.Vec3{X: 2}, geom.Vec3{Z: 1}, new(big.Rat), [2]*big.Rat{big.NewRat(-2, 1), big.NewRat(2, 1)}, WrappedVerticalFiberOptions{})
	if e != nil || len(r.Intervals) != 2 {
		t.Fatal("kiss toggled material parity", r, e)
	}
	first, last := r.Intervals[0], r.Intervals[1]
	if first.Start.Cmp(big.NewRat(-1, 1)) != 0 || first.End.Cmp(big.NewRat(-1, 2)) != 0 || last.Start.Cmp(first.End) != 0 || last.End.Cmp(big.NewRat(1, 2)) != 0 {
		t.Fatal("incorrect material around kiss")
	}
	event := r.Events[first.LastEvent]
	if len(event.Sources) != 2 || event.Location != trimvalidate.Boundary || first.LastEvent != last.FirstEvent {
		t.Fatal("lost touching hole vertex identity")
	}
}
