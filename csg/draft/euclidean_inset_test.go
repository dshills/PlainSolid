package draft

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"

	"plainsolid/brep"
	"plainsolid/geom"
)

func insetTriangleFixture(t *testing.T) brep.Solid {
	t.Helper()
	s, err := brep.Prism([]geom.Vec3{{}, {X: 4}, {Y: 4}}, geom.Vec3{Z: 2})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestStableEuclideanInsetIrrationalNormal(t *testing.T) {
	ctx := context.Background()
	source := insetTriangleFixture(t)
	out, err := InsetStableEuclidean(ctx, source, .25, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Radicand.Cmp(big.NewRat(2, 1)) != 0 || out.ExactVolume.Rational.Cmp(big.NewRat(297, 32)) != 0 ||
		out.ExactVolume.Coefficient.Cmp(big.NewRat(-21, 16)) != 0 || out.MaxSquaredDisplacement.Sign() <= 0 {
		t.Fatal("incorrect exact irrational inset volume", out.Radicand, out.ExactVolume, out.MaxSquaredDisplacement)
	}
	if len(out.ExactVertices) != 6 || len(out.FaceSources) != 5 || out.Solid.Topology().EulerCharacteristic != 2 {
		t.Fatal("original topology was lost", out.Solid.Topology())
	}
	for i, mapped := range out.OldToNew {
		if mapped < 0 || len(out.FaceSources[mapped]) != 1 || out.FaceSources[mapped][0] != i {
			t.Fatal("lost original support identity", i, mapped)
		}
	}
	var maxX float64
	for i, p := range out.Solid.Vertices() {
		maxX = max(maxX, p.X)
		if p.X < .25 || p.Y < .25 || p.Z != .25 && p.Z != 1.75 || p.X+p.Y > 4-.25*math.Sqrt2+1e-15 {
			t.Fatal("offset vertex is outside analytic erosion", i, p)
		}
	}
	if math.Abs(maxX-(3.75-.25*math.Sqrt2)) > 2e-15 {
		t.Fatal("did not move hypotenuse by Euclidean distance", maxX)
	}
	mass, err := out.Solid.MassProperties()
	want := .75 * math.Pow(3.5-.25*math.Sqrt2, 2)
	if err != nil || math.Abs(mass.Volume-want) > 1e-12 {
		t.Fatal("native physical volume", mass, want, err)
	}
}

func TestStableEuclideanInsetRationalAndAtomic(t *testing.T) {
	ctx := context.Background()
	box, err := brep.Box(geom.Vec3{}, geom.Vec3{X: 3, Y: 2, Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	out, err := InsetStableEuclidean(ctx, box, .125, Options{})
	if err != nil || out.ExactVolume.Rational.Cmp(big.NewRat(231, 64)) != 0 || out.ExactVolume.Coefficient.Sign() != 0 || out.MaxSquaredDisplacement.Sign() != 0 {
		t.Fatal("exact rational cuboid", out.ExactVolume, err)
	}
	triangle := insetTriangleFixture(t)
	for _, tc := range []struct {
		ctx      context.Context
		distance float64
		o        Options
		want     error
	}{
		{ctx, 1, Options{}, ErrUnsupported},
		{ctx, -.1, Options{}, ErrInvalid},
		{ctx, .25, Options{MaxWork: 1}, ErrLimit},
		{ctx, .25, Options{MaxCoordinateError: 1e-20}, ErrPrecision},
	} {
		got, err := InsetStableEuclidean(tc.ctx, triangle, tc.distance, tc.o)
		if !errors.Is(err, tc.want) || len(got.ExactVertices) != 0 || got.Radicand != nil {
			t.Fatal("inset failure exposed a partial result", err, tc.want)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if got, err := InsetStableEuclidean(cancelled, triangle, .25, Options{}); !errors.Is(err, context.Canceled) || len(got.ExactVertices) != 0 {
		t.Fatal("cancelled inset exposed a partial result", err)
	}
}

func TestStableEuclideanInsetMultipleRadicals(t *testing.T) {
	ctx := context.Background()
	polygon := []geom.Vec3{{}, {X: 4}, {X: 3, Y: 1}, {X: 1, Y: 2}, {Y: 2}}
	body, err := brep.Prism(polygon, geom.Vec3{Z: 2})
	if err != nil {
		t.Fatal(err)
	}
	if out, err := InsetStableEuclidean(ctx, body, .125, Options{}); !errors.Is(err, ErrUnsupported) || len(out.ExactVertices) != 0 {
		t.Fatal("independent irrational normal classes must remain unresolved", err)
	}
}
