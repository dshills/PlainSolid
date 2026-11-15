package collision

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/brep"
	"plainsolid/geom"
)

func testMassRational(t *testing.T, actual, expected *big.Rat) {
	t.Helper()
	if actual == nil || actual.Cmp(expected) != 0 {
		t.Fatalf("exact mass term got %v, want %v", actual, expected)
	}
}

func TestExactPlanarMassWithCavityAndIsland(t *testing.T) {
	ctx := context.Background()
	q := func(i int64) *big.Rat { return big.NewRat(i, 1) }
	xyz := func(i float64) geom.Vec3 { return geom.Vec3{X: i, Y: i, Z: i} }
	b := func(lo, hi float64) brep.Solid {
		t.Helper()
		s, err := brep.Box(xyz(lo), xyz(hi))
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	s := locationJoin(t, b(0, 8), b(2, 6).Reversed(), b(3, 5))
	m, err := MeasureExactPlanarMass(ctx, s, LocationOptions{}, ExactMassOptions{})
	if err != nil || m.Operations == 0 {
		t.Fatalf("exact planar mass %+v: %v", m, err)
	}
	testMassRational(t, m.Volume, q(456))
	for i := 0; i < 3; i++ {
		testMassRational(t, m.Centroid[i], q(4))
		testMassRational(t, m.FirstMoments[i], q(1824))
		for j := 0; j < 3; j++ {
			central, origin := int64(0), int64(-7296)
			if i == j {
				central, origin = 5296, 19888
			}
			testMassRational(t, m.InertiaCentroid[i][j], q(central))
			testMassRational(t, m.InertiaOrigin[i][j], q(origin))
		}
	}
	if _, err := MeasureExactPlanarMass(ctx, s.Reversed(), LocationOptions{}, ExactMassOptions{}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid orientation admitted: %v", err)
	}
}

func TestExactPlanarMassRetainsShearedProductsAtLargeTranslation(t *testing.T) {
	ctx := context.Background()
	const translation = 1 << 40
	op := []geom.Vec3{{X: translation, Y: translation}, {X: translation + 2, Y: translation}, {X: translation + 3, Y: translation + 2}, {X: translation + 1, Y: translation + 2}}
	s, err := brep.Prism(op, geom.Vec3{Z: 3})
	if err != nil {
		t.Fatal(err)
	}
	loc, err := NewPointLocator(ctx, s, LocationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	m, err := loc.MassPropertiesExact(ctx, ExactMassOptions{})
	if err != nil {
		t.Fatal(err)
	}
	testMassRational(t, m.Volume, big.NewRat(12, 1))
	testMassRational(t, m.Centroid[0], big.NewRat(2*translation+3, 2))
	testMassRational(t, m.Centroid[1], big.NewRat(translation+1, 1))
	testMassRational(t, m.Centroid[2], big.NewRat(3, 2))
	wanted := [3][3]int64{{13, -2, 0}, {-2, 14, 0}, {0, 0, 9}}
	for i := range wanted {
		for j := range wanted[i] {
			testMassRational(t, m.InertiaCentroid[i][j], big.NewRat(wanted[i][j], 1))
		}
	}
	worldXY := new(big.Rat).Mul(m.Centroid[0], m.Centroid[1])
	worldXY.Mul(worldXY, big.NewRat(12, 1)).Add(worldXY, big.NewRat(2, 1))
	testMassRational(t, m.SecondMoments[0][1], worldXY)
	if m.SecondMoments[0][1] == m.SecondMoments[1][0] {
		t.Fatal("symmetric entries alias caller mutations")
	}
	for _, o := range []ExactMassOptions{{MaxOperations: 5}, {MaxCoefficientBits: 32}} {
		if r, err := loc.MassPropertiesExact(ctx, o); !errors.Is(err, ErrLimit) || r.Volume != nil {
			t.Fatalf("unfinished exact moments published: %+v %v", r, err)
		}
	}
}

func TestExactPlanarMassOfPointContactCompound(t *testing.T) {
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
	m, err := l.MassPropertiesExact(ctx, ExactMassOptions{})
	if err != nil {
		t.Fatal(err)
	}
	testMassRational(t, m.Volume, big.NewRat(2, 1))
	for i := 0; i < 3; i++ {
		testMassRational(t, m.Centroid[i], big.NewRat(1, 1))
		for j := 0; j < 3; j++ {
			want := big.NewRat(-1, 2)
			if i == j {
				want = big.NewRat(4, 3)
			}
			testMassRational(t, m.InertiaCentroid[i][j], want)
		}
	}
	e, err := (&VertexContactAssemblyLocator{}).MassPropertiesExact(ctx, ExactMassOptions{})
	if err != nil || e.Volume.Sign() != 0 || e.Centroid[0] != nil || e.InertiaOrigin[0][0].Sign() != 0 {
		t.Fatalf("empty mass %+v: %v", e, err)
	}
}
