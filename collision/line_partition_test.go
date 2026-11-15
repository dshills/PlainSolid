package collision

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/brep"
	"plainsolid/geom"
	"slices"
	"testing"
)

func testLinePartition(t *testing.T, p RationalLinePartition, parameters []*big.Rat, points, spans []Location) {
	t.Helper()
	if len(p.Stations) != len(parameters) || len(p.Stations) != len(points) || len(p.Spans) != len(spans) || p.Work <= 0 {
		t.Fatalf("unexpected partition %+v", p)
	}
	for i, station := range p.Stations {
		if station.Parameter.Cmp(parameters[i]) != 0 || station.Location != points[i] || (station.Location == OnBoundary) != (len(station.BoundaryFaces) > 0) {
			t.Fatalf("station %d: %+v; want %v and %v", i, station, parameters[i], points[i])
		}
	}
	for i, span := range p.Spans {
		if span.Start != i || span.End != i+1 || span.Location != spans[i] || (span.Location == OnBoundary) != (len(span.BoundaryFaces) > 0) {
			t.Fatalf("span %d: %+v; want %v", i, span, spans[i])
		}
	}
}

func TestPlanarRationalLinePartitionOfCavityAndIsland(t *testing.T) {
	ctx := context.Background()
	box := func(lo, hi float64) brep.Solid {
		t.Helper()
		s, err := brep.Box(geom.Vec3{X: lo, Y: lo, Z: lo}, geom.Vec3{X: hi, Y: hi, Z: hi})
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	s := locationJoin(t, box(0, 8), box(2, 6).Reversed(), box(3, 5))
	l, err := NewPointLocator(ctx, s, LocationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	o, d := geom.Vec3{X: -1, Y: 4, Z: 4}, geom.Vec3{X: 2}
	r, err := l.PartitionLine(ctx, o, d, 0, 5, RationalLinePartitionOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var params []*big.Rat
	for _, i := range []int64{0, 1, 3, 4, 6, 7, 9, 10} {
		params = append(params, big.NewRat(i, 2))
	}
	testLinePartition(t, r, params,
		[]Location{Outside, OnBoundary, OnBoundary, OnBoundary, OnBoundary, OnBoundary, OnBoundary, Outside},
		[]Location{Outside, Inside, Outside, Inside, Outside, Inside, Outside})
	for i, solid := range []int{0, 1, 2, 2, 1, 0} {
		faces := r.Stations[i+1].BoundaryFaces
		if len(faces) != 1 || faces[0]/6 != solid {
			t.Fatalf("crossing %d lost its original shell face: %v", i, faces)
		}
	}
	for _, options := range []RationalLinePartitionOptions{{MaxWork: 1}, {MaxInputBits: 1}} {
		if r, err := l.PartitionLine(ctx, o, d, 0, 5, options); !errors.Is(err, ErrLimit) || r.Stations != nil || r.Spans != nil {
			t.Fatalf("budget %+v partially published: %+v %v", options, r, err)
		}
	}
}

func TestPlanarRationalLinePartitionPreservesCoplanarAndVertexContacts(t *testing.T) {
	ctx := context.Background()
	s, err := brep.Box(geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	l, err := NewPointLocator(ctx, s, LocationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	q := func(i int64) *big.Rat { return big.NewRat(i, 1) }
	for _, tc := range []struct {
		origin geom.Vec3
		owners int
	}{{geom.Vec3{X: -1, Z: .5}, 1}, {geom.Vec3{X: -1}, 2}} {
		r, err := l.PartitionLine(ctx, tc.origin, geom.Vec3{X: 1}, 0, 3, RationalLinePartitionOptions{})
		if err != nil {
			t.Fatal(err)
		}
		// A face's internal triangulation break at x=1/2 must not leak.
		testLinePartition(t, r, []*big.Rat{q(0), q(1), q(2), q(3)}, []Location{Outside, OnBoundary, OnBoundary, Outside}, []Location{Outside, OnBoundary, Outside})
		owners := r.Spans[1].BoundaryFaces
		if len(owners) != tc.owners || len(r.Stations[1].BoundaryFaces) != tc.owners+1 || len(r.Stations[2].BoundaryFaces) != tc.owners+1 || !slices.IsSorted(owners) {
			t.Fatalf("coplanar source owners not retained: %+v", r)
		}
	}
	r, err := l.PartitionLine(ctx, geom.Vec3{}, geom.Vec3{X: 1, Y: -1, Z: 1}, -1, 1, RationalLinePartitionOptions{})
	if err != nil {
		t.Fatal(err)
	}
	testLinePartition(t, r, []*big.Rat{q(-1), q(0), q(1)}, []Location{Outside, OnBoundary, Outside}, []Location{Outside, Outside})
	if len(r.Stations[1].BoundaryFaces) != 3 {
		t.Fatal("isolated original vertex did not preserve all three faces")
	}
	org := [3]*big.Rat{q(-1), big.NewRat(1, 3), big.NewRat(2, 3)}
	dir := [3]*big.Rat{q(3), q(0), q(0)}
	r, err = l.PartitionRationalLine(ctx, org, dir, q(0), q(1), RationalLinePartitionOptions{})
	if err != nil {
		t.Fatal(err)
	}
	testLinePartition(t, r, []*big.Rat{q(0), big.NewRat(1, 3), big.NewRat(2, 3), q(1)}, []Location{Outside, OnBoundary, OnBoundary, Outside}, []Location{Outside, Inside, Outside})
	r.Stations[0].Parameter.SetInt64(50)
	end := big.NewRat(1, 2)
	r, err = l.PartitionRationalLine(ctx, org, dir, end, end, RationalLinePartitionOptions{})
	if err != nil || len(r.Stations) != 1 || r.Stations[0].Location != Inside || end.Cmp(big.NewRat(1, 2)) != 0 {
		t.Fatalf("singleton exact query: %+v %v", r, err)
	}
}
