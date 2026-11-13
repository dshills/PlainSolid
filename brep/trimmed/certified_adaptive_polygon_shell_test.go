package trimmed

import (
	"context"
	"math"
	"testing"
)

func TestCertifiedAdaptivePolygonShell(t *testing.T) {
	s := testWarpedSplinePrism(t, false, false)
	o := CertifiedAdaptivePolygonShellOptions{Deflection: .03}
	adaptive, e := s.TessellateAdaptivePolygonShellCertified(context.Background(), o)
	if e != nil {
		t.Fatal(e)
	}
	uniform, e := s.TessellatePolygonShellCertified(context.Background(), CertifiedPolygonShellOptions{Face: CertifiedPatchMeshOptions{Deflection: .03}})
	if e != nil {
		t.Fatal(e)
	}
	if !adaptive.BothDirections || adaptive.Rounds == 0 || adaptive.Subdivisions != 0 {
		t.Fatal("missing adaptive certificate")
	}
	if e = adaptive.Mesh.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	if len(adaptive.Mesh.Triangles) >= len(uniform.Mesh.Triangles) {
		t.Fatal("local curvature failed to reduce work", len(adaptive.Mesh.Triangles), len(uniform.Mesh.Triangles))
	}
	if math.Abs(adaptive.Mesh.SignedVolume()-5) > .03 {
		t.Fatal("independent roof integral", adaptive.Mesh.SignedVolume())
	}
	byFace := map[int]int{}
	for i, f := range adaptive.OriginalFace {
		byFace[f]++
		bound, _ := adaptive.SquaredErrors[i].Float64()
		if bound > .03*.03 {
			t.Fatal("excess residual")
		}
	}
	if len(byFace) != 6 || byFace[0] >= byFace[1] {
		t.Fatal("flat bottom overrefined", byFace)
	}
	t.Logf("adaptive %d triangles, uniform %d, rounds %d, checks %d", len(adaptive.Mesh.Triangles), len(uniform.Mesh.Triangles), adaptive.Rounds, adaptive.TriangleChecks)
	o.MaxTriangleChecks = 12
	bad, e := s.TessellateAdaptivePolygonShellCertified(context.Background(), o)
	if e == nil || len(bad.Mesh.Triangles) != 0 {
		t.Fatal("partial work-budget output")
	}
	o.MaxTriangleChecks = 0
	o.MaxRounds = 1
	o.Deflection = 1e-8
	bad, e = s.TessellateAdaptivePolygonShellCertified(context.Background(), o)
	if e == nil || len(bad.Mesh.Triangles) != 0 {
		t.Fatal("partial round-budget output")
	}
}
