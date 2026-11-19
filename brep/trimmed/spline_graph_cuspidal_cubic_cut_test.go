package trimmed

import (
	"context"
	"math"
	"slices"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/collision"
)

// For the original roofs A−B=(3/16)*((v−1/2)^2−(u−1/4)^3/3).
// Over the 3×2 footprint, its integral is 33/512. Inside the cusp use
// u=1/4+3*t² with half-width 3*t³ for 0<=t<=1/2. The integral of B−A
// across that width is (3/16)*(4/3)*(3*t³)^3 before the 6*t and 6
// Jacobians. Integration gives the pointed region's exact cut volume.
const cuspidalPointedCutVolume = 243.0 / 22528

func TestNativeCuspidalCubicCutGenericDisplay(t *testing.T) {
	ctx := context.Background()
	src := cuspidalCubicCommonFixture(t)
	o := CuspidalCubicCutOptions{}
	display := parametric.TessellationOptions{Deflection: .005, Angle: .1, MaxSubdivisions: 32, MaxTriangles: 80_000}
	pointed, err := CutCuspidalCubicSplineGraphs(ctx, src[0], src[2], src[1], 1, 1e-9, o)
	if err != nil {
		t.Fatal(err)
	}
	s := pointed.Boundary()
	mesh, err := s.Tessellate(ctx, display)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := collision.NewMeshPointLocator(ctx, mesh, nil, collision.LocationOptions{}); err != nil {
		t.Fatal("pointed display is not closed and embedded", err)
	}
	relativeError := math.Abs(mesh.SignedVolume()/cuspidalPointedCutVolume - 1)
	if len(mesh.Vertices) < len(s.vertices) || !slices.Equal(mesh.Vertices[:len(s.vertices)], s.vertices) || relativeError > .01 {
		t.Fatal("pointed display changed native vertices or differs from the source volume by over 1%", mesh.SignedVolume(), relativeError)
	}
	t.Logf("pointed preview %d triangles volume %.15g, %.3f%% relative error",
		len(mesh.Triangles), mesh.SignedVolume(), 100*relativeError)
	exterior, err := CutCuspidalCubicSplineGraphs(ctx, src[0], src[1], src[2], 0, 1e-9, o)
	if err != nil {
		t.Fatal(err)
	}
	// The exterior part is where A is above B: the integral of A−B over
	// the footprint, 33/512, and the pointed volume once more. Its coarse
	// display used to be refused with "mesh: edge is not closed", because
	// two sampled triangles joined the same two vertices by different
	// chart segments.
	display.Deflection, display.Angle = .05, .15
	mesh, err = exterior.Boundary().Tessellate(ctx, display)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := collision.NewMeshPointLocator(ctx, mesh, nil, collision.LocationOptions{}); err != nil {
		t.Fatal("exterior display is not closed and embedded", err)
	}
	relativeError = math.Abs(mesh.SignedVolume()/(33.0/512+cuspidalPointedCutVolume) - 1)
	if relativeError > .1 {
		t.Fatal("exterior display differs from the source volume by over 10%", mesh.SignedVolume(), relativeError)
	}
	t.Logf("exterior preview %d triangles volume %.15g, %.3f%% relative error",
		len(mesh.Triangles), mesh.SignedVolume(), 100*relativeError)
}
