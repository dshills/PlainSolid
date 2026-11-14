package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"testing"
)

func mixedAlgebraicFiberFace(t *testing.T, nonlinear bool) *WrappedQuadraticFace {
	t.Helper()
	base := algebraicFiberFace(t)
	points := [][2]*big.Rat{{big.NewRat(3, 16), big.NewRat(-1, 8)}, {big.NewRat(3, 16), big.NewRat(1, 8)}, {big.NewRat(1, 4), big.NewRat(1, 8)}, {big.NewRat(1, 4), big.NewRat(-1, 8)}}
	var hole []ExactQuadraticUV
	for i, p := range points {
		q, e := quadraticLine(p, points[(i+1)%4])
		if e != nil {
			t.Fatal(e)
		}
		hole = append(hole, q)
	}
	if nonlinear {
		h := hole[2].HomogeneousControls()
		for axis := 0; axis < 3; axis++ {
			h[1][axis].Mul(h[1][axis], big.NewRat(2, 1))
		}
		q, e := NewExactQuadraticUV(h)
		if e != nil {
			t.Fatal(e)
		}
		hole[2] = q
	}
	face, e := NewWrappedQuadraticFace(context.Background(), base.native.Face.Surface.(*geom.WrappedSurfacePatch), base.loops[0], [][]ExactQuadraticUV{hole}, WrappedQuadraticFaceOptions{SurfaceInjectivityOptions: geom.SurfaceInjectivityOptions{ProjectionAxes: [2]int{1, 2}}})
	if e != nil {
		t.Fatal(e)
	}
	return face
}
func assertFiberRationalRoot(t *testing.T, root *geom.ExactAlgebraicRoot, want *big.Rat) {
	t.Helper()
	kind, e := root.Classify(context.Background(), geom.ExactCurvePlaneEventOptions{})
	if e != nil || kind.Kind != geom.ExactRootKindRational || kind.Rational.Cmp(want) != 0 {
		t.Fatal(kind, want, e)
	}
}
func TestWrappedAlgebraicMixedBoundary(t *testing.T) {
	ctx := context.Background()
	face := mixedAlgebraicFiberFace(t, false)
	for _, drop := range []bool{false, true} {
		origin, dir := geom.Vec3{X: 15.0 / 8, Y: 7.0 / 8}, geom.Vec3{Z: 1}
		limits := [2]*big.Rat{big.NewRat(-1, 1), big.NewRat(1, 1)}
		want := [2]*big.Rat{big.NewRat(-1, 8), big.NewRat(1, 8)}
		params := [2]*big.Rat{big.NewRat(1, 1), new(big.Rat)}
		if drop {
			origin.Z = 2
			dir.Z = -3
			limits = [2]*big.Rat{new(big.Rat), big.NewRat(1, 1)}
			want = [2]*big.Rat{big.NewRat(5, 8), big.NewRat(17, 24)}
			params[0], params[1] = params[1], params[0]
		}
		if r, e := face.ClassifyAlgebraicVerticalFiber(ctx, origin, dir, big.NewRat(17, 4), limits, AlgebraicVerticalFiberOptions{}); e == nil || r.Complete {
			t.Fatal("strict API admitted coincidence")
		}
		r, e := face.ClassifyAlgebraicVerticalFiber(ctx, origin, dir, big.NewRat(17, 4), limits, AlgebraicVerticalFiberOptions{AllowBoundaryIntervals: true})
		if e != nil || !r.Complete || len(r.Intervals) != 2 || len(r.BoundaryIntervals) != 1 {
			t.Fatal(r, e)
		}
		b := r.BoundaryIntervals[0]
		if b.FirstEvent != r.Intervals[0].LastEvent || b.LastEvent != r.Intervals[1].FirstEvent || len(b.Sources) != 1 || b.Sources[0].Loop != 1 || b.Sources[0].Edge != 2 {
			t.Fatal("partition/ancestry", b)
		}
		for j, index := range []int{b.FirstEvent, b.LastEvent} {
			event := r.Events[index]
			assertFiberRationalRoot(t, event.Parameter, want[j])
			assertFiberRationalRoot(t, b.Sources[0].Parameters[j], params[j])
			if event.Location != trimvalidate.Boundary || len(event.Sources) != 2 {
				t.Fatal("shared hole vertex", event)
			}
			found := false
			for _, source := range event.Sources {
				if source.Coincident {
					found = true
					if source.Event == nil || source.Loop != 1 || source.Edge != 2 {
						t.Fatal(source)
					}
					assertFiberRationalRoot(t, source.Parameter, params[j])
				}
			}
			if !found {
				t.Fatal("lost coincident source")
			}
		}
		for _, index := range []int{r.Intervals[0].FirstEvent, r.Intervals[1].LastEvent} {
			event := r.Events[index]
			kind, e := event.Parameter.Classify(ctx, geom.ExactCurvePlaneEventOptions{})
			if e != nil || kind.Kind != geom.ExactRootKindIrrational || len(event.Sources) != 1 || event.Sources[0].Coincident {
				t.Fatal("irrational exterior event", event, kind, e)
			}
		}
		low := AlgebraicVerticalFiberOptions{AllowBoundaryIntervals: true, SurfacePatchOptions: geom.SurfacePatchOptions{MaxOperations: r.ExactOperations - 1}}
		if q, e := face.ClassifyAlgebraicVerticalFiber(ctx, origin, dir, big.NewRat(17, 4), limits, low); e == nil || q.Complete || len(q.Events) > 0 || len(q.BoundaryIntervals) > 0 {
			t.Fatal("atomic budget", q, e)
		}
	}
}
func TestWrappedAlgebraicBoundaryFiniteAndUnsupported(t *testing.T) {
	ctx := context.Background()
	face := mixedAlgebraicFiberFace(t, false)
	origin, dir := geom.Vec3{X: 15.0 / 8, Y: 7.0 / 8}, geom.Vec3{Z: 1}
	bounds := [2]*big.Rat{big.NewRat(-1, 16), big.NewRat(1, 16)}
	r, e := face.ClassifyAlgebraicVerticalFiber(ctx, origin, dir, big.NewRat(1, 4), bounds, AlgebraicVerticalFiberOptions{AllowBoundaryIntervals: true})
	if e != nil || len(r.Intervals) != 0 || len(r.BoundaryIntervals) != 1 || len(r.Events) != 2 {
		t.Fatal(r, e)
	}
	for j, event := range r.Events {
		if len(event.Sources) != 1 || event.Sources[0].Event != nil || !event.Sources[0].Coincident || event.FiniteMask != uint8(1<<j) {
			t.Fatal("clipped interior provenance", event)
		}
		assertFiberRationalRoot(t, event.Sources[0].Parameter, big.NewRat(int64(3-2*j), 4))
	}
	singleton, e := face.ClassifyAlgebraicVerticalFiber(ctx, origin, dir, big.NewRat(1, 4), [2]*big.Rat{new(big.Rat), new(big.Rat)}, AlgebraicVerticalFiberOptions{AllowBoundaryIntervals: true})
	if e != nil || !singleton.Complete || len(singleton.Events) != 1 || len(singleton.Events[0].Sources) != 1 || len(singleton.Intervals) > 0 || len(singleton.BoundaryIntervals) > 0 {
		t.Fatal("singleton", singleton, e)
	}
	nonlinear := mixedAlgebraicFiberFace(t, true)
	if q, e := nonlinear.ClassifyAlgebraicVerticalFiber(ctx, origin, dir, big.NewRat(1, 4), bounds, AlgebraicVerticalFiberOptions{AllowBoundaryIntervals: true}); e == nil || q.Complete || len(q.Events) > 0 {
		t.Fatal("nonlinear coincident speed", q, e)
	}
}
