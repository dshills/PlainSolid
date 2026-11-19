//go:build research

package trimmed

import (
	"context"
	"errors"
	"math"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestCertifiedPolygonShellWarpedPrism(t *testing.T) {
	s := testWarpedSplinePrism(t, false, false)
	o := CertifiedPolygonShellOptions{Face: CertifiedPatchMeshOptions{Deflection: .03, MaxSubdivisions: 16}}
	r, err := s.TessellatePolygonShellCertified(context.Background(), o)
	if err != nil {
		t.Fatal(err)
	}
	if !r.BothDirections || r.Subdivisions < 2 {
		t.Fatal("missing curved refinement", r.Subdivisions)
	}
	if err = r.Mesh.ValidateSolid(); err != nil {
		t.Fatal(err)
	}
	seen := map[int]bool{}
	for i, f := range r.OriginalFace {
		seen[f] = true
		b, _ := r.SquaredErrors[i].Float64()
		if b > .03*.03 {
			t.Fatal("uncertified triangle")
		}
	}
	if len(seen) != 6 {
		t.Fatal("lost face ancestry")
	}
	// Roof 1+xy/4 over [0,2]^2 has exact integral 5. Piecewise linear
	// interpolation converges quadratically; no mass certificate is inferred.
	if math.Abs(r.Mesh.SignedVolume()-5) > .02 {
		t.Fatal("independent roof integral", r.Mesh.SignedVolume())
	}
	// Six quad faces on a common n x n triangulation have 6n²+2 unique
	// vertices. This detects unwelded shared edges and accidental seam merging.
	if len(r.Mesh.Vertices) != 6*r.Subdivisions*r.Subdivisions+2 {
		t.Fatal("shared topology identity", len(r.Mesh.Vertices), r.Subdivisions)
	}
	again, err := s.TessellatePolygonShellCertified(context.Background(), o)
	if err != nil || len(again.Mesh.Vertices) != len(r.Mesh.Vertices) {
		t.Fatal(err)
	}
	for i, p := range r.Mesh.Vertices {
		if p != again.Mesh.Vertices[i] {
			t.Fatal("nondeterministic samples")
		}
	}
	o.MaxTriangles = 1
	bad, err := s.TessellatePolygonShellCertified(context.Background(), o)
	if err == nil || len(bad.Mesh.Vertices) != 0 {
		t.Fatal("partial budget result")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = s.TessellatePolygonShellCertified(ctx, o)
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	// A reversed model parameter convention without matching p-curve reversal
	// must never be repaired by coordinate welding.
	broken := s
	broken.faces = s.Faces()
	broken.faces[0].Outer[0].Reversed = !broken.faces[0].Outer[0].Reversed
	bad, err = broken.TessellatePolygonShellCertified(context.Background(), CertifiedPolygonShellOptions{Face: CertifiedPatchMeshOptions{Deflection: .1}})
	if err == nil || len(bad.Mesh.Vertices) != 0 {
		t.Fatal("accepted incompatible edge parameterization")
	}
}

func TestCertifiedPolygonShellRationalSupports(t *testing.T) {
	slowtest.Skip(t)
	original := testWarpedSplinePrism(t, false, false)
	var surfaces []*geom.BSplineSurface
	var domains []geom.ParameterRectangle
	var reversed []bool
	for _, f := range original.faces {
		var poles []geom.Vec3
		var weights []float64
		axis := []float64{1, 2, 1}
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				p, e := f.Surface.Eval(float64(i)/2, float64(j)/2)
				if e != nil {
					t.Fatal(e)
				}
				poles = append(poles, p)
				weights = append(weights, axis[i]*axis[j])
			}
		}
		knots := []float64{0, 0, 0, 1, 1, 1}
		bs, e := geom.NewBSplineSurface(2, 2, 3, 3, poles, knots, knots, weights)
		if e != nil {
			t.Fatal(e)
		}
		surfaces = append(surfaces, bs)
		domains = append(domains, geom.ParameterRectangle{})
		reversed = append(reversed, f.Reversed)
	}
	// The symmetric rational coordinate map q(1-t)=1-q(t) preserves
	// all edge orientations. It is nonaffine but maps the same warped body.
	s, _, e := SewSplineRectangles(context.Background(), surfaces, domains, reversed, SewOptions{Tolerance: 1e-9})
	if e != nil {
		t.Fatal(e)
	}
	r, e := s.TessellatePolygonShellCertified(context.Background(), CertifiedPolygonShellOptions{Face: CertifiedPatchMeshOptions{Deflection: .05, MaxSubdivisions: 32}})
	if e != nil || !r.BothDirections {
		t.Fatal(e)
	}
	if e = r.Mesh.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	if math.Abs(r.Mesh.SignedVolume()-5) > .03 {
		t.Fatal("rational reparameterization changed body", r.Mesh.SignedVolume())
	}
	if len(r.Mesh.Vertices) != 6*r.Subdivisions*r.Subdivisions+2 {
		t.Fatal("rational shared edge samples")
	}
	adaptive, e := s.TessellateAdaptivePolygonShellCertified(context.Background(), CertifiedAdaptivePolygonShellOptions{Deflection: .05})
	if e != nil || !adaptive.BothDirections {
		t.Fatal("adaptive rational shell", e)
	}
	if e = adaptive.Mesh.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	if math.Abs(adaptive.Mesh.SignedVolume()-5) > .03 {
		t.Fatal("adaptive rational independent volume", adaptive.Mesh.SignedVolume())
	}

}
