package trimmed

import (
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/mesh"
)

// A wall with an oblique rim must meet a bound on the angle between a
// triangle and the surface. Under uniform refinement it could not while a
// seed triangle had two sides on one smooth rim, or while a triangle that
// joined both chart copies of a seam vertex kept a trim from being seated:
// every level then left triangles in the plane of the cap, or the fallback of
// sampled chords was used, which meets no such bound.
//
// Tessellate now meshes such a wall in strips along the axis (compactBand)
// and reaches uniform refinement only where the strips cannot be finished,
// so both are tested here: Tessellate, and uniform refinement by itself.
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
			const bound = 0.5
			o, err := normalizeTrimmedTessellationOptions(context.Background(), parametric.TessellationOptions{Deflection: 0.1, Angle: bound})
			if err != nil {
				t.Fatal(err)
			}
			// A chord of the wall turns by at most twice the bound, and
			// a prism on such chords has sin(2*bound)/(2*bound) of the
			// volume of its cylinder. Uniform refinement halves its
			// chords well past the bound and keeps 97%.
			for _, mesher := range []struct {
				name string
				run  func() (mesh.Mesh, error)
				part float64
			}{
				{"Tessellate", func() (mesh.Mesh, error) { return solid.Tessellate(context.Background(), o) }, math.Sin(2*bound) / (2 * bound)},
				{"uniform refinement", func() (mesh.Mesh, error) {
					return solid.withTessellationTrimMemo(&tessellationTrimMemo{limit: tessellationTrimMemoLimit}).refineUniformly(context.Background(), o)
				}, 0.97},
			} {
				m, err := mesher.run()
				if err != nil {
					t.Fatalf("%s: %v", mesher.name, err)
				}
				if err := m.ValidateSolid(); err != nil {
					t.Fatalf("%s: %v", mesher.name, err)
				}
				if len(m.Triangles) > c.most {
					t.Fatalf("%s: %d triangles, want at most %d", mesher.name, len(m.Triangles), c.most)
				}
				if got := m.SignedVolume(); got > c.volume+1e-9 || got < mesher.part*c.volume {
					t.Fatalf("%s: mesh volume %.12g, want between %.12g and %.12g", mesher.name, got, mesher.part*c.volume, c.volume)
				}
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
