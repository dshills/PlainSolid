package trimmed

import (
	"context"
	"fmt"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/collision"
	"plainsolid/geom"
)

func TestNativeClippedPolynomialFuseClosedDisplay(t *testing.T) {
	checkClippedFuseDisplayLayouts(t, clippedPolynomialStock)
}

func checkClippedFuseDisplayLayouts(t *testing.T, stock func(*testing.T, int, int, int) [3]*geom.BSplineSurface) {
	t.Helper()
	ctx := context.Background()
	for layout := range 2 {
		for axis := range 2 {
			for profile := range 2 {
				for order := range 2 {
					t.Run(fmt.Sprintf("layout%d/axis%d/profile%d/order%d", layout, axis, profile, order), func(t *testing.T) {
						t.Parallel()
						src := stock(t, axis, profile, layout)
						r, err := FuseClippedPolynomialEnvelope(ctx, src[0], src[1+order], src[2-order],
							axis, 1e-9, PolynomialCubicCrossingTangencyEnvelopeOptions{})
						if err != nil {
							t.Fatal(err)
						}
						checkClippedFuseDisplay(t, r)
					})
				}
			}
		}
	}
}

func checkClippedFuseDisplay(t *testing.T, r *ClippedPolynomialFuse) {
	t.Helper()
	want, _ := r.ExactVolume().Float64()
	checkClippedShellDisplay(t, r.Boundary(), want)
}

func checkClippedShellDisplay(t *testing.T, boundary Solid, want float64) {
	t.Helper()
	ctx := context.Background()
	opts := parametric.TessellationOptions{Deflection: .05, Angle: .15, MaxSubdivisions: 32, MaxTriangles: 80_000}
	m, err := boundary.Tessellate(ctx, opts)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.ValidateSolid(); err != nil {
		t.Fatal(err)
	}
	report, err := collision.SelfIntersectionsBVHContext(ctx, m, collision.Options{})
	if err != nil || !report.Complete || len(report.Contacts) != 0 {
		t.Fatalf("display intersection scan: %v, %+v", err, report)
	}
	mass, err := m.MassProperties()
	if err != nil || math.Abs(mass.Volume-want) > .003*want {
		t.Fatalf("display volume %g differs from exact native volume %g: %v", mass.Volume, want, err)
	}
	used := make(map[geom.Vec3]bool, len(m.Vertices))
	for _, tri := range m.Triangles {
		for _, id := range tri {
			used[m.Vertices[id]] = true
		}
	}
	for _, p := range boundary.Vertices() {
		if !used[p] {
			t.Fatalf("lost a native wall station or contact: %+v", p)
		}
	}
	t.Logf("%d triangles, relative volume error %.7g", len(m.Triangles), math.Abs(mass.Volume-want)/want)
}
