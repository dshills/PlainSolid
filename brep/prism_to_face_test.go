package brep_test

import (
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/brep"
	"plainsolid/geom"
)

func stopRectangle(x0, y0, x1, y1, z float64) []geom.Vec3 {
	return []geom.Vec3{{X: x0, Y: y0, Z: z}, {X: x1, Y: y0, Z: z}, {X: x1, Y: y1, Z: z}, {X: x0, Y: y1, Z: z}}
}

func stopStock(t *testing.T, outer []geom.Vec3, holes [][]geom.Vec3, offset geom.Vec3) brep.Solid {
	t.Helper()
	s, err := brep.PrismWithHoles(outer, holes, offset)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func stopVolume(t *testing.T, s brep.Solid, want float64) {
	t.Helper()
	p, err := s.MassProperties()
	if err != nil || math.Abs(p.Volume-want) > 1e-12 {
		t.Fatalf("volume = %.15g, want %.15g; %v", p.Volume, want, err)
	}
}

func TestPrismToFaceFollowsFiniteSelectedCap(t *testing.T) {
	ctx := context.Background()
	profile := stopRectangle(0, 0, 10, 5, 0)
	for _, z := range []float64{10, 20} {
		stock := stopStock(t, stopRectangle(-5, -5, 15, 10, 0), nil, geom.Vec3{Z: z})
		tool, err := brep.PrismToFace(ctx, profile, nil, geom.Vec3{Z: .125}, stock, 1)
		if err != nil {
			t.Fatal(err)
		}
		stopVolume(t, tool, 50*z)
		box, err := tool.Bounds()
		if err != nil || box.Max.Z != z || box.Min.Z != 0 {
			t.Fatalf("bounds = %+v; %v", box, err)
		}
		stopVolume(t, stock, 300*z)
	}
	if _, err := brep.PrismToFace(ctx, profile, nil, geom.Vec3{Z: 1}, brep.Solid{}, 1); !errors.Is(err, geom.ErrInvalid) {
		t.Fatalf("lost face: %v", err)
	}
}

func TestPrismToFaceObliqueCapAndReverseDirection(t *testing.T) {
	ctx := context.Background()
	outer := stopRectangle(-1, -2, 3, 3, 0)
	for i := range outer {
		outer[i].Z = 2 + outer[i].X
	}
	stock := stopStock(t, outer, nil, geom.Vec3{Z: 1})
	profile := stopRectangle(0, 0, 2, 1, 0)
	// Reverse the caller's source winding; forward orientation is still outward.
	profile[1], profile[3] = profile[3], profile[1]
	forward, err := brep.PrismToFace(ctx, profile, nil, geom.Vec3{Z: 3}, stock, 0)
	if err != nil {
		t.Fatal(err)
	}
	stopVolume(t, forward, 6)
	for i := range profile {
		profile[i].Z = 5
	}
	reverse, err := brep.PrismToFace(ctx, profile, nil, geom.Vec3{Z: -1}, stock, 0)
	if err != nil {
		t.Fatal(err)
	}
	stopVolume(t, reverse, 4)
	for _, s := range []brep.Solid{forward, reverse} {
		for _, p := range s.Vertices()[4:] {
			if p.Z != p.X+2 {
				t.Fatalf("vertex missed exact stop plane: %v", p)
			}
		}
	}
	// Direction need not be normal to the start or end cap.
	flat := stopStock(t, stopRectangle(-1, -1, 8, 8, 1), nil, geom.Vec3{Z: 2})
	diagonal, err := brep.PrismToFace(ctx, stopRectangle(0, 0, 1, 1, 0), nil, geom.Vec3{X: 1, Z: 2}, flat, 1)
	if err != nil {
		t.Fatal(err)
	}
	stopVolume(t, diagonal, 3)
	for _, p := range diagonal.Vertices()[4:] {
		if p.Z != 3 || p.X != 1.5 && p.X != 2.5 {
			t.Fatalf("wrong ray endpoint: %v", p)
		}
	}
}

func TestPrismToFaceFiniteCoverageWithHolesAndConcavity(t *testing.T) {
	ctx := context.Background()
	up := geom.Vec3{Z: 1}
	profile := stopRectangle(0, 0, 6, 6, 0)
	hole := stopRectangle(2, 2, 4, 4, 1)
	stock := stopStock(t, stopRectangle(0, 0, 6, 6, 1), [][]geom.Vec3{hole}, up)
	if _, err := brep.PrismToFace(ctx, profile, nil, up, stock, 1); !errors.Is(err, brep.ErrTerminationUncovered) {
		t.Fatalf("projected cap covers the target hole: %v", err)
	}
	for _, h := range [][]geom.Vec3{stopRectangle(2, 2, 4, 4, 0), stopRectangle(1, 1, 5, 5, 0)} {
		tool, err := brep.PrismToFace(ctx, profile, [][]geom.Vec3{h}, up, stock, 1)
		if err != nil {
			t.Fatal(err)
		}
		stopVolume(t, tool, 2*(36-(h[1].X-h[0].X)*(h[2].Y-h[1].Y)))
	}
	// A notch enters the profile between its vertices. All four projected
	// corners are covered; a vertex-only check would accept the extrusion.
	notch := []geom.Vec3{
		{X: 0, Y: 0, Z: 1}, {X: 6, Y: 0, Z: 1}, {X: 6, Y: 6, Z: 1},
		{X: 4, Y: 6, Z: 1}, {X: 4, Y: 2, Z: 1}, {X: 2, Y: 2, Z: 1},
		{X: 2, Y: 6, Z: 1}, {X: 0, Y: 6, Z: 1},
	}
	stock = stopStock(t, notch, nil, up)
	if _, err := brep.PrismToFace(ctx, stopRectangle(1, 1, 5, 5, 0), nil, up, stock, 1); !errors.Is(err, brep.ErrTerminationUncovered) {
		t.Fatalf("profile bridges a concave notch: %v", err)
	}
	// Coincident concave boundaries are valid.
	for i := range notch {
		notch[i].Z = 0
	}
	tool, err := brep.PrismToFace(ctx, notch, nil, up, stock, 1)
	if err != nil {
		t.Fatal(err)
	}
	stopVolume(t, tool, 56)

	stock = stopStock(t, stopRectangle(0, 0, 1, 1, 1), nil, up)
	ulp := math.Nextafter(1, 2)
	if _, err := brep.PrismToFace(ctx, stopRectangle(0, 0, ulp, 1, 0), nil, up, stock, 1); !errors.Is(err, brep.ErrTerminationUncovered) {
		t.Fatalf("one-ULP uncovered strip: %v", err)
	}
}

func TestPrismToFaceRejectsInvalidConstruction(t *testing.T) {
	ctx := context.Background()
	up := geom.Vec3{Z: 1}
	stock := stopStock(t, stopRectangle(-2, -2, 3, 3, 1), nil, up)
	profile := stopRectangle(0, 0, 1, 1, 0)
	for _, tc := range []struct {
		name string
		face int
		dir  geom.Vec3
		want error
	}{
		{"parallel to target", 2, up, geom.ErrDomain},
		{"parallel to source", 1, geom.Vec3{Y: 1}, geom.ErrDomain},
		{"behind source", 1, geom.Vec3{Z: -1}, geom.ErrDomain},
		{"nonbinary endpoint", 0, geom.Vec3{X: 1, Z: 3}, geom.ErrUnrepresentable},
		{"bad face", -1, up, geom.ErrInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, err := brep.PrismToFace(ctx, profile, nil, tc.dir, stock, tc.face)
			if !errors.Is(err, tc.want) || len(s.Faces()) != 0 {
				t.Fatalf("failed construction returned %d faces, error %v (want %v)", len(s.Faces()), err, tc.want)
			}
		})
	}
	profile[2].Z = math.SmallestNonzeroFloat64
	if _, err := brep.PrismToFace(ctx, profile, nil, up, stock, 1); !errors.Is(err, geom.ErrDomain) {
		t.Fatalf("nonplanar represented input: %v", err)
	}
	cancel, done := context.WithCancel(ctx)
	done()
	if _, err := brep.PrismToFace(cancel, profile, nil, up, stock, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled: %v", err)
	}
}
