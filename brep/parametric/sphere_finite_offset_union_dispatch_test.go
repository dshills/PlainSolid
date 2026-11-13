package parametric

import (
	"bytes"
	"context"
	"math"
	"plainsolid/collision"
	"plainsolid/geom"
	"testing"
)

func TestFiniteOffsetUnionDispatch(t *testing.T) {
	ctx := context.Background()
	volumes := map[SphereFiniteOffsetUnionKind]float64{}
	for _, tc := range []struct {
		name         string
		r, d, lo, hi float64
		kind         SphereFiniteOffsetUnionKind
	}{
		{"disjoint", 1, 5, -1, 1, SphereOffsetUnionDisjoint}, {"sphere", 1, .5, -1, 1, SphereOffsetUnionSphere},
		{"cylinder", 4, .5, -4, 4, SphereOffsetUnionCylinder}, {"north", 1, .5, 0, 4, SphereOffsetUnionNorth},
		{"south", 1, .5, -4, 0, SphereOffsetUnionSouth}, {"both", 1, .5, -4, 4, SphereOffsetUnionBoth},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, e := UnionFiniteOffsetCylinder(ctx, geom.Vec3{}, 3, tc.r, tc.d, tc.lo, tc.hi, 1e-8)
			if e != nil {
				t.Fatal(e)
			}
			if r.Kind != tc.kind {
				t.Fatal(r.Kind)
			}
			for body, s := range r.Solids {
				seen := make(map[int]bool)
				for _, groups := range [][][]int{r.SphericalFaces, r.CylindricalFaces, r.LowerCapFaces, r.UpperCapFaces} {
					for _, i := range groups[body] {
						if seen[i] || i < 0 || i >= len(s.Faces()) {
							t.Fatal("ancestry", i)
						}
						seen[i] = true
					}
				}
				if len(seen) != len(s.Faces()) {
					t.Fatal("missing faces")
				}
				var b bytes.Buffer
				if e = WriteJSON(&b, s); e != nil {
					t.Fatal(e)
				}
				loaded, e := ReadJSON(&b, JSONLimits{})
				if e != nil {
					t.Fatal(e)
				}
				mass, e := loaded.MassProperties(ctx, IntegrationOptions{})
				if e != nil {
					t.Fatal(e)
				}
				volumes[tc.kind] += mass.Volume
				if r.VolumePiCoefficients != nil {
					v, _ := r.VolumePiCoefficients[body].Float64()
					if math.Abs(mass.Volume-math.Pi*v) > 1e-7 {
						t.Fatal(mass, v)
					}
				}
				if tc.kind == SphereOffsetUnionNorth && mass.Centroid.Z <= 0 || tc.kind == SphereOffsetUnionSouth && mass.Centroid.Z >= 0 || tc.kind == SphereOffsetUnionBoth && math.Abs(mass.Centroid.Z) > 1e-8 {
					t.Fatal("orientation", mass.Centroid)
				}
				if tc.kind == SphereOffsetUnionBoth {
					m, e := loaded.Tessellate(ctx, TessellationOptions{Deflection: .25, MaxSubdivisions: 128, MaxTriangles: 100000})
					if e != nil {
						t.Fatal(e)
					}
					if e = m.ValidateSolid(); e != nil {
						t.Fatal(e)
					}
					report, e := collision.SelfIntersectionsBVHContext(ctx, m, collision.Options{})
					if e != nil || !report.Complete || len(report.Contacts) != 0 {
						t.Fatal(e, len(report.Contacts))
					}
				}
			}
		})
	}
	if !t.Failed() {
		if math.Abs(volumes[SphereOffsetUnionNorth]-volumes[SphereOffsetUnionSouth]) > 1e-7 || math.Abs(volumes[SphereOffsetUnionBoth]-(2*volumes[SphereOffsetUnionNorth]-36*math.Pi)) > 1e-7 {
			t.Fatal("independent reflected additive identity", volumes)
		}
	}
}
func TestFiniteOffsetUnionRejects(t *testing.T) {
	for _, ab := range [][2]float64{{0, 2.9}, {-2.9, 0}, {-3, 3}} {
		r, e := UnionFiniteOffsetCylinder(context.Background(), geom.Vec3{}, 3, 1, .5, ab[0], ab[1], 1e-8)
		if e == nil || len(r.Solids) != 0 {
			t.Fatal(ab, r, e)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := UnionFiniteOffsetCylinder(ctx, geom.Vec3{}, 3, 1, .5, 0, 4, 1e-8); e != context.Canceled {
		t.Fatal(e)
	}
}
