package collision

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/brep"
	"plainsolid/geom"
)

func TestExactSegmentBoundaryDistanceOfCrossingParallelFaceAndSkewEdge(t *testing.T) {
	ctx := context.Background()
	s, err := brep.Box(geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	l, err := NewPointLocator(ctx, s, LocationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		start, end  geom.Vec3
		at, squared *big.Rat
		boundary    geom.Vec3
		owners      int
	}{
		{geom.Vec3{X: -1, Y: .5, Z: .5}, geom.Vec3{X: 2, Y: .5, Z: .5}, big.NewRat(1, 3), big.NewRat(0, 1), geom.Vec3{Y: .5, Z: .5}, 1},
		{geom.Vec3{X: -1, Y: .5, Z: -1}, geom.Vec3{X: 2, Y: .5, Z: -1}, big.NewRat(1, 3), big.NewRat(1, 1), geom.Vec3{Y: .5}, 2},
		{geom.Vec3{X: -1, Y: -.5, Z: -1}, geom.Vec3{X: -.5, Y: -1, Z: 2}, big.NewRat(1, 2), big.NewRat(9, 8), geom.Vec3{Z: .5}, 2},
		{geom.Vec3{X: -1, Y: .5, Z: .5}, geom.Vec3{X: -1, Y: .5, Z: .5}, big.NewRat(0, 1), big.NewRat(1, 1), geom.Vec3{Y: .5, Z: .5}, 1},
	} {
		r, err := l.DistanceFromSegment(ctx, tc.start, tc.end, SegmentBoundaryDistanceOptions{})
		if err != nil || len(r.Owners) != tc.owners || r.Work <= 0 {
			t.Fatalf("segment %+v clearance %+v: %v", tc, r, err)
		}
		testMassRational(t, r.SquaredDistance, tc.squared)
		testMassRational(t, r.Parameter, tc.at)
		for i, want := range exact(tc.boundary) {
			testMassRational(t, r.BoundaryPoint[i], want)
		}
	}
	if _, err := (&PointLocator{}).DistanceFromSegment(ctx, geom.Vec3{}, geom.Vec3{X: 1}, SegmentBoundaryDistanceOptions{}); !errors.Is(err, ErrEmpty) {
		t.Fatalf("empty boundary did not fail: %v", err)
	}
}

func TestExactSegmentBoundaryDistanceRetainsCompoundContactOwners(t *testing.T) {
	ctx := context.Background()
	xyz := func(i float64) geom.Vec3 { return geom.Vec3{X: i, Y: i, Z: i} }
	a, err := brep.Box(xyz(0), xyz(1))
	if err != nil {
		t.Fatal(err)
	}
	b, err := brep.Box(xyz(1), xyz(2))
	if err != nil {
		t.Fatal(err)
	}
	parts := []brep.Solid{a, b}
	l, err := NewVertexContactAssemblyLocator(ctx, parts, []AssemblyVertexContact{assemblyContactAt(t, parts, 0, 1, xyz(1))}, VertexContactAssemblyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	q := func(n int64) *big.Rat { return big.NewRat(n, 4) }
	start, end := [3]*big.Rat{q(3), q(5), q(3)}, [3]*big.Rat{q(5), q(3), q(5)}
	r, err := l.DistanceFromRationalSegment(ctx, start, end, SegmentBoundaryDistanceOptions{})
	if err != nil || len(r.Owners) != 6 || r.SquaredDistance.Sign() != 0 {
		t.Fatalf("rational contact %+v: %v", r, err)
	}
	testMassRational(t, r.Parameter, big.NewRat(1, 2))
	for i, p := range r.SegmentPoint {
		testMassRational(t, p, big.NewRat(1, 1))
		testMassRational(t, r.BoundaryPoint[i], big.NewRat(1, 1))
	}
	for _, opt := range []SegmentBoundaryDistanceOptions{{MaxWork: 1}, {MaxInputBits: 1}} {
		if out, err := l.DistanceFromRationalSegment(ctx, start, end, opt); !errors.Is(err, ErrLimit) || out.SquaredDistance != nil || out.Owners != nil {
			t.Fatalf("unfinished clearance was admitted %+v: %v", out, err)
		}
	}
}
