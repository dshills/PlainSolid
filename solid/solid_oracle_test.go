package solid_test

import (
	"math"
	"math/rand"
	"testing"

	"plainsolid/geom"
	"plainsolid/solid"
)

// Integer-coordinate boxes admit an independent unit-voxel membership oracle.
// It never consults the output cell representation or reuses grid decomposition.
func TestRandomBooleanVoxelOracle(t *testing.T) {
	r := rand.New(rand.NewSource(91))
	for trial := 0; trial < 160; trial++ {
		boxes := make([][]geom.Box, 2)
		operands := make([]solid.Solid, 2)
		for j := range boxes {
			for i := 0; i < 3; i++ {
				x, y, z := r.Intn(5), r.Intn(5), r.Intn(5)
				boxes[j] = append(boxes[j], geom.Box{Min: geom.Vec3{X: float64(x), Y: float64(y), Z: float64(z)}, Max: geom.Vec3{X: float64(x + 1 + r.Intn(3)), Y: float64(y + 1 + r.Intn(3)), Z: float64(z + 1 + r.Intn(3))}})
			}
			var e error
			operands[j], e = solid.New(boxes[j]...)
			if e != nil {
				t.Fatal(e)
			}
		}
		for _, op := range []solid.Operation{solid.Union, solid.Intersection, solid.Difference} {
			out, e := operands[0].Boolean(operands[1], op, solid.Options{})
			if e != nil {
				t.Fatal(e)
			}
			want := 0.
			moment := geom.Vec3{}
			for x := 0; x < 8; x++ {
				for y := 0; y < 8; y++ {
					for z := 0; z < 8; z++ {
						p := geom.Vec3{X: float64(x) + .5, Y: float64(y) + .5, Z: float64(z) + .5}
						in := [2]bool{}
						for j := range boxes {
							for _, b := range boxes[j] {
								in[j] = in[j] || (p.X > b.Min.X && p.X < b.Max.X && p.Y > b.Min.Y && p.Y < b.Max.Y && p.Z > b.Min.Z && p.Z < b.Max.Z)
							}
						}
						v := in[0] || in[1]
						if op == solid.Intersection {
							v = in[0] && in[1]
						}
						if op == solid.Difference {
							v = in[0] && !in[1]
						}
						if v {
							want++
							moment = moment.Add(p)
						}
						if out.Contains(p) != v {
							t.Fatalf("trial %d op %d at %v", trial, op, p)
						}
					}
				}
			}
			if out.Volume() != want {
				t.Fatalf("volume trial %d op %d got %g want %g", trial, op, out.Volume(), want)
			}
			if want > 0 {
				c, e := out.Centroid()
				if e != nil || c.Distance(moment.Scale(1/want)) > 1e-12 {
					t.Fatalf("centroid %v %v", c, e)
				}
			}
			m, e := out.Mesh(solid.Options{})
			if e != nil {
				t.Fatal(e)
			}
			if math.Abs(m.SignedVolume()-want) > 1e-9 {
				t.Fatalf("mesh volume trial %d got %g want %g", trial, m.SignedVolume(), want)
			}
		}
	}
}
