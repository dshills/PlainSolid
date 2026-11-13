package trimmed

import (
	"context"
	"errors"
	"math"
	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func compactBoreFixture(t *testing.T) Solid {
	t.Helper()
	base, err := brep.Box(geom.Vec3{X: -10, Y: -5}, geom.Vec3{X: 10, Y: 5, Z: 10})
	if err != nil {
		t.Fatal(err)
	}
	body, _, err := BoreCylinder(base, 0, 1, geom.Vec3{}, 3, 1e-9)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func TestCompactCylinderMeshLinearGrowthAndBudgets(t *testing.T) {
	body := compactBoreFixture(t)
	curved := make([]bool, len(body.edges))
	for _, f := range body.faces {
		if _, ok := f.Surface.(geom.CylinderSurface); ok {
			curved[f.Outer[0].Edge] = true
			curved[f.Outer[2].Edge] = true
		}
	}
	for _, n := range []int{4, 8, 16} {
		expected := 16*n + 16
		opts := parametric.TessellationOptions{Deflection: 100, MaxSubdivisions: 32, MaxTriangles: expected}
		m, err := body.compactCylinderCandidate(context.Background(), n, curved, opts)
		if err != nil {
			t.Fatalf("n%d: %v", n, err)
		}
		if len(m.Triangles) != expected {
			t.Fatalf("n%d: %d triangles, want%d", n, len(m.Triangles), expected)
		}
		if err = m.ValidateSolid(); err != nil {
			t.Fatal(err)
		}
		opts.MaxTriangles--
		if _, err = body.compactCylinderCandidate(context.Background(), n, curved, opts); !errors.Is(err, parametric.ErrBudget) {
			t.Fatalf("triangle budget: %v", err)
		}
	}
	opts := parametric.TessellationOptions{Deflection: .5, Angle: .1, MaxSubdivisions: 32, MaxTriangles: 1000}
	m, ok, err := body.compactCylinderMesh(context.Background(), opts)
	if !ok || err != nil {
		t.Fatalf("compact path: %v %v", ok, err)
	}
	if len(m.Triangles) > 200 {
		t.Fatalf("unnecessary axial/interior subdivisions: %d triangles", len(m.Triangles))
	}
	opts.MaxSubdivisions = 1
	if _, ok, err = body.compactCylinderMesh(context.Background(), opts); !ok || !errors.Is(err, parametric.ErrBudget) {
		t.Fatalf("subdivision budget: %v %v", ok, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, ok, err = body.compactCylinderMesh(ctx, opts); !ok || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v %v", ok, err)
	}
}

func TestCompactCylinderMeshDeclinesUnsupportedDomains(t *testing.T) {
	body := compactBoreFixture(t)
	opts := parametric.TessellationOptions{Deflection: .1, Angle: .2, MaxSubdivisions: 32, MaxTriangles: 1000, AnalyticBound: true}
	if _, ok, _ := body.compactCylinderMesh(context.Background(), opts); ok {
		t.Fatal("sampled mesh substituted for analytic certificate")
	}
	opts.AnalyticBound = false
	base, err := brep.Box(geom.Vec3{}, geom.Vec3{X: 4, Y: 5, Z: 6})
	if err != nil {
		t.Fatal(err)
	}
	planar, err := FromPlanar(base, 1e-9)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := planar.compactCylinderMesh(context.Background(), opts); ok {
		t.Fatal("pure planar domain claimed")
	}
	oblique, _, err := BoreCylinderAlong(base, 0, 1, geom.Vec3{X: 1, Y: 2}, geom.Vec3{X: 1, Z: 3}, .35, 1e-10)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := oblique.compactCylinderMesh(context.Background(), opts); ok {
		t.Fatal("oblique curved wall trims claimed")
	}
}

func TestCompactCylinderMeshFineBoundaryFallsBack(t *testing.T) {
	base, err := brep.Box(geom.Vec3{X: -4, Y: -4}, geom.Vec3{X: 4, Y: 4, Z: 8})
	if err != nil {
		t.Fatal(err)
	}
	body, _, err := BoreCylinder(base, 0, 1, geom.Vec3{}, 1, 1e-9)
	if err != nil {
		t.Fatal(err)
	}
	opts := parametric.TessellationOptions{Deflection: .0002, Angle: .2, MaxSubdivisions: 128, MaxTriangles: 1000000}
	if _, ok, err := body.compactCylinderMesh(context.Background(), opts); !ok || err != nil {
		t.Fatalf("bounded larger compact boundary must succeed: %v %v", ok, err)
	}
	finer := opts
	finer.Deflection = .000001
	finer.MaxSubdivisions = 512
	if _, ok, err := body.compactCylinderMesh(context.Background(), finer); ok || err != nil {
		t.Fatalf("boundary beyond compact cap must delegate: %v %v", ok, err)
	}
	m, err := body.Tessellate(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if err = m.ValidateSolid(); err != nil {
		t.Fatal(err)
	}
	if len(m.Triangles) > opts.MaxTriangles {
		t.Fatal("generic fallback exceeded triangle budget")
	}
	// Delegation is limited to the compact chart limit, never a relaxation of
	// the caller's triangle or subdivision budgets.
	opts.MaxTriangles = 1
	if _, err := body.Tessellate(context.Background(), opts); !errors.Is(err, parametric.ErrBudget) {
		t.Fatalf("triangle budget after fallback: %v", err)
	}
	opts.MaxTriangles = 1000000
	opts.MaxSubdivisions = 1
	if _, err := body.Tessellate(context.Background(), opts); !errors.Is(err, parametric.ErrBudget) {
		t.Fatalf("subdivision budget after fallback: %v", err)
	}
}

func TestCompactCylinderMeshMappedSupports(t *testing.T) {
	body := compactBoreFixture(t)
	// The long-edge fillet constructor places an authored section with this
	// cyclic coordinate map. Its planes, walls and straight edges stay mapped.
	body, err := body.Transformed(geom.Affine{{0, 0, 1, 7}, {1, 0, 0, 8}, {0, 1, 0, 9}})
	if err != nil {
		t.Fatal(err)
	}
	opts := parametric.TessellationOptions{Deflection: .5, Angle: .1, MaxSubdivisions: 32, MaxTriangles: 200}
	m, err := body.Tessellate(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if err = m.ValidateSolid(); err != nil {
		t.Fatal(err)
	}
	want := 2000 - 90*math.Pi
	if got := m.SignedVolume(); math.Abs(got-want) > .002*want {
		t.Fatalf("mapped mesh volume = %g, want approximately %g", got, want)
	}
}
