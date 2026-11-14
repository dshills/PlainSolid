package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"testing"
)

func algebraicFiberFace(t *testing.T) *WrappedQuadraticFace {
	t.Helper()
	pts := [][2]*big.Rat{{big.NewRat(-1, 4), big.NewRat(-1, 1)}, {big.NewRat(1, 8), big.NewRat(-1, 1)}, {big.NewRat(1, 8), big.NewRat(1, 1)}, {big.NewRat(-1, 4), big.NewRat(1, 1)}}
	var loop []ExactQuadraticUV
	for i, p := range pts {
		q, e := quadraticLine(p, pts[(i+1)%4])
		if e != nil {
			t.Fatal(e)
		}
		loop = append(loop, q)
	}
	q, e := NewExactQuadraticUV([3][3]*big.Rat{{big.NewRat(1, 8), big.NewRat(-1, 1), big.NewRat(1, 1)}, {big.NewRat(1, 2), new(big.Rat), big.NewRat(1, 1)}, {big.NewRat(1, 8), big.NewRat(1, 1), big.NewRat(1, 1)}})
	if e != nil {
		t.Fatal(e)
	}
	loop[1] = q
	face, e := NewWrappedQuadraticFace(context.Background(), wrappedOverlapSupport(t), loop, nil, WrappedQuadraticFaceOptions{SurfaceInjectivityOptions: geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}})
	if e != nil {
		t.Fatal(e)
	}
	return face
}
func TestWrappedAlgebraicVerticalFiber(t *testing.T) {
	ctx := context.Background()
	face := algebraicFiberFace(t)
	for _, drop := range []bool{false, true} {
		origin, dir := geom.Vec3{X: 15.0 / 8, Y: 7.0 / 8}, geom.Vec3{Z: 1}
		limits := [2]*big.Rat{big.NewRat(-2, 1), big.NewRat(2, 1)}
		if drop {
			origin.Z = 2
			dir.Z = -3
			limits = [2]*big.Rat{new(big.Rat), big.NewRat(1, 1)}
		}
		r, e := face.ClassifyAlgebraicVerticalFiber(ctx, origin, dir, big.NewRat(17, 4), limits, AlgebraicVerticalFiberOptions{})
		if e != nil || !r.Complete || len(r.Intervals) != 1 || r.PeriodShift.Cmp(big.NewInt(-1)) != 0 {
			t.Fatal(r, e)
		}
		interval := r.Intervals[0]
		for j, index := range []int{interval.FirstEvent, interval.LastEvent} {
			event := r.Events[index]
			if len(event.Sources) != 1 || event.Sources[0].Loop != 0 || event.Sources[0].Edge != 1 || event.Location != trimvalidate.Boundary {
				t.Fatal("source", event)
			}
			kind, e := event.Parameter.Classify(ctx, geom.ExactCurvePlaneEventOptions{})
			if e != nil || kind.Kind != geom.ExactRootKindIrrational {
				t.Fatal("rounded root", kind, e)
			}
			lo, hi := "-1", "0"
			if (j == 1) != drop {
				lo, hi = "0", "1"
			}
			expected, e := geom.ImportExactRoot(ctx, geom.ExactRootRecipe{Version: 1, Coefficients: []string{"-1", "0", "3"}, Lower: lo, Upper: hi}, geom.ExactRootCodecOptions{})
			if e != nil {
				t.Fatal(e)
			}
			equal, e := event.V.Compare(ctx, expected, geom.ExactCurvePlaneEventOptions{})
			if e != nil || !equal.Proven || equal.Order != 0 {
				t.Fatal("independent quadratic", equal, e)
			}
			// V=2*s-1 and Z=origin.Z+direction.Z*t on this independent fixture.
			v, _, e := event.Sources[0].Event.Root().AffineMap(ctx, big.NewRat(-1, 1), big.NewRat(2, 1), geom.ExactCurvePlaneEventOptions{})
			if e != nil {
				t.Fatal(e)
			}
			same, e := v.Compare(ctx, event.V, geom.ExactCurvePlaneEventOptions{})
			if e != nil || same.Order != 0 {
				t.Fatal("source parameter lost", same, e)
			}
			z, _, e := event.Parameter.AffineMap(ctx, ratInset(origin.Z), ratInset(dir.Z), geom.ExactCurvePlaneEventOptions{})
			if e != nil {
				t.Fatal(e)
			}
			same, e = z.Compare(ctx, event.V, geom.ExactCurvePlaneEventOptions{})
			if e != nil || same.Order != 0 {
				t.Fatal("original line units", same, e)
			}
		}
		low := AlgebraicVerticalFiberOptions{SurfacePatchOptions: geom.SurfacePatchOptions{MaxOperations: r.ExactOperations - 1}}
		if q, e := face.ClassifyAlgebraicVerticalFiber(ctx, origin, dir, big.NewRat(1, 4), limits, low); e == nil || q.Complete || len(q.Events) > 0 {
			t.Fatal("non atomic budget", q, e)
		}
	}
	// Finite rational clips strictly inside the irrational boundary stay Inside.
	r, e := face.ClassifyAlgebraicVerticalFiber(ctx, geom.Vec3{X: 15.0 / 8, Y: 7.0 / 8}, geom.Vec3{Z: 1}, big.NewRat(1, 4), [2]*big.Rat{big.NewRat(-1, 3), big.NewRat(1, 3)}, AlgebraicVerticalFiberOptions{})
	if e != nil || len(r.Intervals) != 1 || len(r.Events) != 2 || r.Events[0].Location != trimvalidate.Inside || r.Events[1].Location != trimvalidate.Inside || r.Events[0].FiniteMask != 1 || r.Events[1].FiniteMask != 2 {
		t.Fatal("finite clipping", r, e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if q, e := face.ClassifyAlgebraicVerticalFiber(canceled, geom.Vec3{X: 15.0 / 8, Y: 7.0 / 8}, geom.Vec3{Z: 1}, big.NewRat(1, 4), [2]*big.Rat{big.NewRat(-2, 1), big.NewRat(2, 1)}, AlgebraicVerticalFiberOptions{}); e == nil || q.Complete || len(q.Events) > 0 {
		t.Fatal("cancel")
	}
}

func TestWrappedAlgebraicVerticalFiberHoleAndTouch(t *testing.T) {
	ctx := context.Background()
	original := algebraicFiberFace(t)
	pts := [][2]*big.Rat{{big.NewRat(3, 16), big.NewRat(-1, 8)}, {big.NewRat(3, 16), big.NewRat(1, 8)}, {big.NewRat(9, 32), big.NewRat(1, 8)}, {big.NewRat(9, 32), big.NewRat(-1, 8)}}
	var hole []ExactQuadraticUV
	for i, p := range pts {
		q, e := quadraticLine(p, pts[(i+1)%4])
		if e != nil {
			t.Fatal(e)
		}
		hole = append(hole, q)
	}
	face, e := NewWrappedQuadraticFace(ctx, original.native.Face.Surface.(*geom.WrappedSurfacePatch), original.loops[0], [][]ExactQuadraticUV{hole}, WrappedQuadraticFaceOptions{SurfaceInjectivityOptions: geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}})
	if e != nil {
		t.Fatal(e)
	}
	limits := [2]*big.Rat{big.NewRat(-1, 1), big.NewRat(1, 1)}
	r, e := face.ClassifyAlgebraicVerticalFiber(ctx, geom.Vec3{X: 15.0 / 8, Y: 7.0 / 8}, geom.Vec3{Z: 1}, big.NewRat(1, 4), limits, AlgebraicVerticalFiberOptions{})
	if e != nil || !r.Complete || len(r.Intervals) != 2 {
		t.Fatal(r, e)
	}
	for j, index := range []int{r.Intervals[0].LastEvent, r.Intervals[1].FirstEvent} {
		ev := r.Events[index]
		c, e := ev.Parameter.Classify(ctx, geom.ExactCurvePlaneEventOptions{})
		want := big.NewRat(-1, 8)
		if j == 1 {
			want.Neg(want)
		}
		if e != nil || c.Kind != geom.ExactRootKindRational || c.Rational.Cmp(want) != 0 || len(ev.Sources) != 1 || ev.Sources[0].Loop != 1 {
			t.Fatal("hole source", ev, c, e)
		}
	}
	touch, e := original.ClassifyAlgebraicVerticalFiber(ctx, geom.Vec3{X: 231.0 / 128, Y: 135.0 / 128}, geom.Vec3{Z: 1}, big.NewRat(5, 16), limits, AlgebraicVerticalFiberOptions{})
	if e != nil || !touch.Complete || len(touch.Intervals) != 0 {
		t.Fatal("touch", touch, e)
	}
	count := 0
	for _, ev := range touch.Events {
		if ev.Location == trimvalidate.Boundary {
			count++
			if len(ev.Sources) != 1 || ev.Sources[0].Event.Multiplicity() != 2 {
				t.Fatal(ev)
			}
		}
	}
	if count != 1 {
		t.Fatal("touch count", count)
	}
	for _, options := range []AlgebraicVerticalFiberOptions{{MaxEvents: 2}, {SurfacePatchOptions: geom.SurfacePatchOptions{MaxOperations: 1}}} {
		q, e := face.ClassifyAlgebraicVerticalFiber(ctx, geom.Vec3{X: 15.0 / 8, Y: 7.0 / 8}, geom.Vec3{Z: 1}, big.NewRat(1, 4), limits, options)
		if e == nil || q.Complete || len(q.Events) != 0 || len(q.Intervals) != 0 {
			t.Fatal("atomic rejection", q, e)
		}
	}
	// Constant-U left edge remains a distinct unsupported coincidence domain.
	q, e := original.ClassifyAlgebraicVerticalFiber(ctx, geom.Vec3{X: 15.0 / 8, Y: -7.0 / 8}, geom.Vec3{Z: 1}, big.NewRat(-1, 4), limits, AlgebraicVerticalFiberOptions{})
	if e == nil || q.Complete || len(q.Events) != 0 {
		t.Fatal("coincidence admitted", q, e)
	}
}
