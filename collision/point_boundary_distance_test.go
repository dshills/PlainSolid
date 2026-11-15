package collision

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/brep"
	"plainsolid/geom"
)

func TestExactPointBoundaryDistanceKeepsAllFaceTies(t *testing.T) {
	ctx := context.Background()
	s, err := brep.Box(geom.Vec3{}, geom.Vec3{X: 1, Y: 1, Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	l, err := NewPointLocator(ctx, s, LocationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	p := geom.Vec3{X: .5, Y: .5, Z: .5}
	r, err := l.DistanceToBoundary(ctx, p, PointBoundaryDistanceOptions{})
	if err != nil || len(r.Witnesses) != 6 || r.Work <= 0 {
		t.Fatalf("missing center ties: %+v %v", r, err)
	}
	testMassRational(t, r.SquaredDistance, big.NewRat(1, 4))
	boundary := map[int]bool{}
	for _, w := range r.Witnesses {
		if len(w.Owners) != 1 || w.Owners[0].Solid != 0 || boundary[w.Owners[0].Face] {
			t.Fatalf("face owner proof %+v", w)
		}
		boundary[w.Owners[0].Face] = true
		moved := 0
		for _, x := range w.Point {
			if x.Sign() == 0 || x.Cmp(big.NewRat(1, 1)) == 0 {
				moved++
			} else if x.Cmp(big.NewRat(1, 2)) != 0 {
				t.Fatal("nearest face foot changed", w.Point)
			}
		}
		if moved != 1 {
			t.Fatal("nearest point does not lie in exactly one original face", w.Point)
		}
	}
	for _, o := range []PointBoundaryDistanceOptions{{MaxWitnesses: 5}, {MaxWork: 1}, {MaxInputBits: 1}} {
		if got, err := l.DistanceToBoundary(ctx, p, o); !errors.Is(err, ErrLimit) || got.SquaredDistance != nil || got.Witnesses != nil {
			t.Fatalf("budget returned incomplete nearest set: %+v %v", got, err)
		}
	}
	if _, err := (&PointLocator{}).DistanceToBoundary(ctx, p, PointBoundaryDistanceOptions{}); !errors.Is(err, ErrEmpty) {
		t.Fatalf("empty boundary distance %v", err)
	}
}

func TestExactPointBoundaryDistanceKeepsNonbinaryEdgeFootAndCompoundOwners(t *testing.T) {
	ctx := context.Background()
	profile := []geom.Vec3{{}, {X: 2}, {X: 3, Y: 2}, {X: 1, Y: 2}}
	s, err := brep.Prism(profile, geom.Vec3{Z: 3})
	if err != nil {
		t.Fatal(err)
	}
	l, err := NewPointLocator(ctx, s, LocationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	got, err := l.DistanceToBoundaryRational(ctx, [3]*big.Rat{big.NewRat(0, 1), big.NewRat(1, 1), big.NewRat(-1, 1)}, PointBoundaryDistanceOptions{})
	if err != nil || len(got.Witnesses) != 1 || len(got.Witnesses[0].Owners) != 2 {
		t.Fatalf("nearest original edge %+v: %v", got, err)
	}
	testMassRational(t, got.SquaredDistance, big.NewRat(6, 5))
	for i, q := range []*big.Rat{big.NewRat(2, 5), big.NewRat(4, 5), big.NewRat(0, 1)} {
		testMassRational(t, got.Witnesses[0].Point[i], q)
	}
	xyz := func(i float64) geom.Vec3 { return geom.Vec3{X: i, Y: i, Z: i} }
	first, err := brep.Box(xyz(0), xyz(1))
	if err != nil {
		t.Fatal(err)
	}
	second, err := brep.Box(xyz(1), xyz(2))
	if err != nil {
		t.Fatal(err)
	}
	parts := []brep.Solid{first, second}
	a, err := NewVertexContactAssemblyLocator(ctx, parts, []AssemblyVertexContact{assemblyContactAt(t, parts, 0, 1, xyz(1))}, VertexContactAssemblyOptions{})
	if err != nil {
		t.Fatal(err)
	}
	r, err := a.DistanceToBoundaryRational(ctx, exact(xyz(1)), PointBoundaryDistanceOptions{MaxWitnesses: 1})
	if err != nil || r.SquaredDistance.Sign() != 0 || len(r.Witnesses) != 1 || len(r.Witnesses[0].Owners) != 6 {
		t.Fatalf("shared point witness %+v: %v", r, err)
	}
	for i, owner := range r.Witnesses[0].Owners {
		if owner.Solid != i/3 {
			t.Fatal("coincident witness lost its component owners", r)
		}
	}
	r.Witnesses[0].Point[0].SetInt64(900)
	again, err := a.DistanceToBoundaryRational(ctx, exact(xyz(1)), PointBoundaryDistanceOptions{})
	if err != nil || again.Witnesses[0].Point[0].Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatalf("caller mutated an exact snapshot: %+v %v", again, err)
	}
}
