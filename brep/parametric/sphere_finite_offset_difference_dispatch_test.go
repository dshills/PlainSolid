package parametric

import (
	"bytes"
	"context"
	"math"
	"plainsolid/geom"
	"testing"
)

func TestFiniteOffsetDifferenceDispatch(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name         string
		r, d, lo, hi float64
		kind         SphereFiniteOffsetDifferenceKind
	}{
		{"disjoint", 1, 5, -1, 1, SphereOffsetUnchanged}, {"empty", 4, .5, -4, 4, SphereOffsetEmpty},
		{"cavity", 1, .5, -1, 1, SphereOffsetCavity}, {"north", 1, .5, 0, 4, SphereOffsetNorthBlind},
		{"south", 1, .5, -4, 0, SphereOffsetSouthBlind}, {"through", 1, .5, -4, 4, SphereOffsetThrough},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, e := SubtractFiniteOffsetCylinder(ctx, geom.Vec3{}, 3, tc.r, tc.d, tc.lo, tc.hi, 1e-8)
			if e != nil {
				t.Fatal(e)
			}
			if r.Kind != tc.kind {
				t.Fatal(r.Kind)
			}
			seen := make(map[int]bool)
			for _, group := range [][]int{r.SphericalFaces, r.CylindricalFaces, r.LowerCapFaces, r.UpperCapFaces} {
				for _, i := range group {
					if seen[i] || i < 0 || i >= len(r.Solid.Faces()) {
						t.Fatal("ancestry", i)
					}
					seen[i] = true
				}
			}
			if len(seen) != len(r.Solid.Faces()) {
				t.Fatal("missing ancestry")
			}
			var b bytes.Buffer
			if e = WriteJSON(&b, r.Solid); e != nil {
				t.Fatal(e)
			}
			loaded, e := ReadJSON(&b, JSONLimits{})
			if e != nil {
				t.Fatal(e)
			}
			if tc.kind == SphereOffsetEmpty {
				if len(loaded.Faces()) != 0 || r.VolumePiCoefficient == nil || r.VolumePiCoefficient.Sign() != 0 {
					t.Fatal(r)
				}
				return
			}
			p, e := loaded.MassProperties(ctx, IntegrationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			if p.Volume <= 0 || p.Volume > 36*math.Pi+1e-7 {
				t.Fatal(p)
			}
			if r.VolumePiCoefficient != nil {
				v, _ := r.VolumePiCoefficient.Float64()
				if math.Abs(p.Volume-math.Pi*v) > 1e-7 {
					t.Fatal(p, v)
				}
			}
			if tc.kind == SphereOffsetNorthBlind && p.Centroid.Z >= 0 || tc.kind == SphereOffsetSouthBlind && p.Centroid.Z <= 0 {
				t.Fatal("wrong end", p.Centroid)
			}
		})
	}
}

func TestFiniteOffsetDifferenceDispatchRejectsPartial(t *testing.T) {
	// The finite upper cap truncates the crossing wall: blind infinite-end
	// construction would remove extra material and must not be selected.
	for _, ab := range [][2]float64{{0, 2.9}, {-2.9, 0}, {-3, 3}} {
		r, e := SubtractFiniteOffsetCylinder(context.Background(), geom.Vec3{}, 3, 1, .5, ab[0], ab[1], 1e-8)
		if e == nil || len(r.Solid.Faces()) != 0 {
			t.Fatal(ab, r, e)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := SubtractFiniteOffsetCylinder(ctx, geom.Vec3{}, 3, 1, .5, 0, 4, 1e-8); e != context.Canceled {
		t.Fatal(e)
	}
}
