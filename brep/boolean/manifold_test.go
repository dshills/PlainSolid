package boolean

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"

	"plainsolid/brep"
	"plainsolid/geom"
)

// TestManifold covers N1 to N3: which check refuses the union of two
// solids that only touch, and that the cut of the same pair is A.
func TestManifold(t *testing.T) {
	cube := lifted(t, box(t, 0, 0, 0, 2, 2, 2))
	turn, err := geom.Rotation(geom.Vec3{Y: 1}, -math.Pi/4)
	if err != nil {
		t.Fatal(err)
	}
	// A unit cube on one corner with its long diagonal upright: its
	// corners have irrational coordinates.
	tilt, err := geom.Rotation(geom.Vec3{X: 1, Y: -1}, math.Acos(1/math.Sqrt(3)))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name  string
		b     brep.Solid
		check string
		touch int
	}{
		{"an edge shared", box(t, 2, 0, 2, 3, 2, 3), "N1", 0},
		{"a vertex shared", box(t, 2, 2, 2, 3, 3, 3), "N2", 0},
		{"a vertex on an edge", moved(t, box(t, 0, 0, 0, 1, 1, 1), geom.Affine{{0.5, -0.5, 0.25, 2}, {0.5, 0.25, -0.5, 1}, {0.25, 0.5, 0.5, 2}}), "N2", 0},
		{"an edge resting in a face", moved(t, box(t, 0, 0.5, 0, 1, 1.5, 1), geom.Translation(geom.Vec3{X: 1, Z: 2}).Mul(turn)), "N3", 1},
		{"a vertex resting in a face", moved(t, box(t, 0, 0, 0, 1, 1, 1), geom.Translation(geom.Vec3{X: 1, Y: 1, Z: 2}).Mul(tilt)), "N3", 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			b := lifted(t, c.b)
			_, err := Combine(context.Background(), cube, b, Union, matrixOptions)
			if !errors.Is(err, ErrNonManifold) || !strings.Contains(err.Error(), c.check) {
				t.Fatalf("union: got %v, want ErrNonManifold from %s", err, c.check)
			}
			r, err := Combine(context.Background(), cube, b, Cut, matrixOptions)
			if err != nil {
				t.Fatal(err)
			}
			if top := r.Solid.Topology(); top.Faces != 6 || top.Vertices != 8 || math.Abs(r.Volume-8) > 1e-9 || len(r.Touching) != c.touch || len(r.TouchingAt) != c.touch {
				t.Fatalf("cut: %d faces, %d vertices, volume %g, %d touching contacts", top.Faces, top.Vertices, r.Volume, len(r.Touching))
			}
			if r, err = Combine(context.Background(), cube, b, Common, matrixOptions); err != nil || r.Solids != 0 {
				t.Fatalf("common: %d solids, %v", r.Solids, err)
			}
		})
	}
}
