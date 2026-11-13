package geom

import (
	"context"
	"errors"
	"testing"
)

func TestCommonXYBoundaryContactConfinement(t *testing.T) {
	makePatch := func(z [4]float64, fold bool) *SurfacePatch {
		poles := []Vec3{{0, 0, z[0]}, {0, 1, z[1]}, {1, 0, z[2]}, {1, 1, z[3]}}
		if fold {
			poles[2].X = 0
			poles[3].X = 0
		}
		s, e := NewBSplineSurface(1, 1, 2, 2, poles, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, nil)
		if e != nil {
			t.Fatal(e)
		}
		p, e := NewSurfacePatch(s, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		return p
	}
	a := makePatch([4]float64{}, false)
	for _, tc := range []struct {
		name              string
		z                 [4]float64
		applied, excluded bool
		bounds            [4][2]float64
	}{
		{"corner", [4]float64{0, 1, 1, 2}, true, false, [4][2]float64{}},
		{"side", [4]float64{0, 0, 1, 1}, true, false, [4][2]float64{{0, 0}, {0, 1}, {0, 0}, {0, 1}}},
		{"strict", [4]float64{1, 2, 2, 3}, true, true, [4][2]float64{}},
		{"crossing", [4]float64{-1, 1, -1, 1}, false, false, [4][2]float64{}},
		{"coincident", [4]float64{}, false, false, [4][2]float64{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bounds, applied, excluded, _, e := commonXYBoundaryContactBounds(context.Background(), a, makePatch(tc.z, false), 1000000, 16384)
			if e != nil || applied != tc.applied || excluded != tc.excluded || bounds != tc.bounds {
				t.Fatal(bounds, applied, excluded, e)
			}
		})
	}
	_, applied, excluded, _, e := commonXYBoundaryContactBounds(context.Background(), a, makePatch([4]float64{0, 1, 1, 2}, true), 1000000, 16384)
	if e != nil || applied || excluded {
		t.Fatal(applied, excluded, e)
	}
	_, _, _, _, e = commonXYBoundaryContactBounds(context.Background(), a, makePatch([4]float64{0, 1, 1, 2}, false), 1, 16384)
	if !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
}
