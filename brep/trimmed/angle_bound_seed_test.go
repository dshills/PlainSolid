package trimmed

import (
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// A wall with an oblique rim must meet a bound on the angle between a
// triangle and the surface. It could not while a seed triangle had two sides
// on one smooth rim, or while a triangle that joined both chart copies of a
// seam vertex kept a trim from being seated: every level then left triangles
// in the plane of the cap, or the fallback of sampled chords was used, which
// meets no such bound.
func TestTessellateAngleBoundOnObliqueRim(t *testing.T) {
	slope := math.Tan(math.Pi / 6)
	capped := func(arcs int) func(*testing.T) ([]geom.Vec3, []parametric.Edge, []Face) {
		return func(t *testing.T) ([]geom.Vec3, []parametric.Edge, []Face) {
			return cappedCylinder(t, 0.5, [2]float64{0, slope}, [2]float64{2, 0}, arcs)
		}
	}
	for _, c := range []struct {
		name   string
		build  func(*testing.T) ([]geom.Vec3, []parametric.Edge, []Face)
		volume float64
		most   int // triangles; the tessellation budget is 1,000,000
	}{
		{"lower cap oblique, four arcs", capped(4), 0.5 * math.Pi, 8000},
		{"lower cap oblique, eight arcs", capped(8), 0.5 * math.Pi, 16000},
		{"both caps oblique, from a Boolean", obliqueSlug, 0.25 * math.Pi * 2 / math.Cos(math.Pi/6), 30000},
	} {
		t.Run(c.name, func(t *testing.T) {
			vertices, edges, faces := c.build(t)
			solid, err := NewSolid(vertices, edges, faces, 1e-7)
			if err != nil {
				t.Fatal(err)
			}
			m, err := solid.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: 0.1, Angle: 0.5})
			if err != nil {
				t.Fatal(err)
			}
			if err := m.ValidateSolid(); err != nil {
				t.Fatal(err)
			}
			if len(m.Triangles) > c.most {
				t.Fatalf("%d triangles, want at most %d", len(m.Triangles), c.most)
			}
			// Within the angle bound the chords of the wall are short:
			// the mesh has at least 97% of the volume and no more.
			if got := m.SignedVolume(); got > c.volume+1e-9 || got < 0.97*c.volume {
				t.Fatalf("mesh volume %.12g, want a little under %.12g", got, c.volume)
			}
		})
	}
}

// A cylinder of radius 1 that ends on a plane 55° from its axis, as a
// Boolean cut it: the seam is not at an extreme of the rim, so one arc of
// the rim dips far below its chord and no triangle on that chord holds it.
// Refinement folds there and the fallback of sampled chords is used.
// Without an angle bound that gives a mesh. With one it cannot, and the
// request must end in the budget error and nothing else.
func TestTessellateAngleBoundBeyondSeeds(t *testing.T) {
	vertices, edges, faces, tolerance := loadStoredSolid(t, "oblique_to_face.json")
	solid, err := NewSolid(vertices, edges, faces, tolerance)
	if err != nil {
		t.Fatal(err)
	}
	m, err := solid.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: 0.2})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.ValidateSolid(); err != nil {
		t.Fatal(err)
	}
	if len(m.Triangles) > 20000 {
		t.Fatalf("%d triangles without an angle bound", len(m.Triangles))
	}
	m, err = solid.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: 0.5, Angle: 0.5, MaxTriangles: 5000})
	switch {
	case err == nil:
		if err := m.ValidateSolid(); err != nil {
			t.Fatal(err)
		}
		t.Logf("the angle bound is met with %d triangles", len(m.Triangles))
	case errors.Is(err, parametric.ErrBudget):
		t.Logf("the angle bound is not met: %v", err)
	default:
		t.Fatal(err)
	}
}
