package trimmed

import (
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// A rim that bulges into its wall leaves the seed triangle on its chord
// when the triangle is narrow at a corner where the rim starts out along
// the far side. Refinement then folds at that corner at every level, and
// NewSolid has no mesh to validate.
func TestNewSolidCurvedTrimLeavesItsSeed(t *testing.T) {
	lens := 8*math.Acos(0.5) - math.Sqrt(12)
	capped := func(radius float64, arcs int, low, high, height float64) func(*testing.T) ([]geom.Vec3, []parametric.Edge, []Face, float64) {
		return func(t *testing.T) ([]geom.Vec3, []parametric.Edge, []Face, float64) {
			vertices, edges, faces := cappedCylinder(t, radius, [2]float64{0, low}, [2]float64{height, high}, arcs)
			return vertices, edges, faces, 1e-7
		}
	}
	for _, c := range []struct {
		name       string
		build      func(*testing.T) ([]geom.Vec3, []parametric.Edge, []Face, float64)
		volume     float64
		deflection float64
	}{
		{"oblique caps, eight arcs", capped(0.5, 8, -2, 0.5, 2), 0.5 * math.Pi, 0.05},
		{"steep oblique caps, six arcs", capped(0.5, 6, -3, 0.5, 3), 0.75 * math.Pi, 0.05},
		{"steep lower cap, six arcs", capped(0.5, 6, 3, 1, 2), 0.5 * math.Pi, 0.05},
		// The block [-5,5]³ with parallel blind bores of radius 2 and
		// depths 6 and 7, their axes 2 apart, which overlap over a length
		// of 3, as a Boolean cut it.
		{"two overlapping bores", func(t *testing.T) ([]geom.Vec3, []parametric.Edge, []Face, float64) {
			return loadStoredSolid(t, "unequal_bores.json")
		}, 1000 - 4*math.Pi*(6+7) + 3*lens, 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			vertices, edges, faces, tolerance := c.build(t)
			solid, err := NewSolid(vertices, edges, faces, tolerance)
			if err != nil {
				t.Fatal(err)
			}
			checkClosedDisplay(t, solid, c.volume, c.deflection)
		})
	}
}
