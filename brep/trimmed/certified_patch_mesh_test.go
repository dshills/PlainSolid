package trimmed

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/geom"
)

func TestCertifiedPatchMeshIndependent(t *testing.T) {
	ctx := context.Background()
	k := []float64{0, 0, 1, 1}
	s, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 1}, {X: 1}, {X: 1, Y: 1, Z: 1}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	p, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := TessellatePatchCertified(ctx, p, CertifiedPatchMeshOptions{Deflection: .2, MaxSubdivisions: 16})
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Mesh.Triangles) != 2*r.Subdivisions*r.Subdivisions || len(r.Mesh.Vertices) != (r.Subdivisions+1)*(r.Subdivisions+1) {
		t.Fatal("uniform grid")
	}
	limit := ratInset(.2)
	limit.Mul(limit, limit)
	incidence := map[[2]int]int{}
	for i, tri := range r.Mesh.Triangles {
		if r.SquaredErrors[i].Cmp(limit) > 0 {
			t.Fatal("false error certificate")
		}
		for j, a := range tri {
			b := tri[(j+1)%3]
			if a > b {
				a, b = b, a
			}
			incidence[[2]int{a, b}]++
		}
		// Independently known bilinear support image at the UV centroid.
		uv := r.UVTriangles[i]
		u := (uv[0].X + uv[1].X + uv[2].X) / 3
		v := (uv[0].Y + uv[1].Y + uv[2].Y) / 3
		point := geom.Vec3{X: u, Y: v, Z: u * v}
		target := newExactBoundTriangle([3]geom.Vec3{r.Mesh.Vertices[tri[0]], r.Mesh.Vertices[tri[1]], r.Mesh.Vertices[tri[2]]})
		if target.distance(exactBoundPoint(point)).Cmp(r.SquaredErrors[i]) > 0 {
			t.Fatal("interior deviation")
		}
	}
	boundary := 0
	for _, count := range incidence {
		if count == 1 {
			boundary++
		} else if count != 2 {
			t.Fatal("nonconforming incidence")
		}
	}
	if boundary != 4*r.Subdivisions {
		t.Fatal("open boundary", boundary)
	}
	denied, e := TessellatePatchCertified(ctx, p, CertifiedPatchMeshOptions{Deflection: 1e-9, MaxTriangleChecks: 2})
	if !errors.Is(e, geom.ErrConvergence) || len(denied.Mesh.Vertices) != 0 {
		t.Fatal("budget partial result", e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	denied, e = TessellatePatchCertified(canceled, p, CertifiedPatchMeshOptions{Deflection: .2})
	if !errors.Is(e, context.Canceled) || len(denied.Mesh.Vertices) != 0 {
		t.Fatal(e)
	}
}

func TestCertifiedWrappedPatchMeshIndependent(t *testing.T) {
	r, e := TessellateWrappedPatchCertified(context.Background(), independentWrappedToroid(t), CertifiedPatchMeshOptions{Deflection: 1, MaxSubdivisions: 16})
	if e != nil {
		t.Fatal(e)
	}
	if e := r.Mesh.ValidateSolid(); e != nil {
		t.Fatal("certified synchronized seams", e)
	}
	if len(r.Mesh.Vertices) != r.Subdivisions*r.Subdivisions {
		t.Fatal("seam indices not shared")
	}
	for _, b := range r.SquaredErrors {
		if b.Cmp(big.NewRat(1, 1)) > 0 {
			t.Fatal("false wrapped bound")
		}
	}
	if r.Mesh.SignedVolume() <= 0 {
		t.Fatal("orientation")
	}
}

func TestCertifiedAffineZeroAndBilinearQuadraticConvergence(t *testing.T) {
	ctx := context.Background()
	knots := []float64{2, 2, 6, 6}
	for _, curved := range []bool{false, true} {
		z := .0
		if curved {
			z = 1
		}
		source, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{X: .25, Y: -.5}, {X: .25, Y: .5}, {X: 2.25, Y: -.5}, {X: 2.25, Y: .5, Z: z}}, knots, knots, []float64{.5, .5, .5, .5})
		if e != nil {
			t.Fatal(e)
		}
		p, e := geom.NewSurfacePatch(source, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		deflection := 1e-12
		if curved {
			deflection = .01
		}
		r, e := TessellatePatchCertified(ctx, p, CertifiedPatchMeshOptions{Deflection: deflection, MaxSubdivisions: 16})
		if e != nil {
			t.Fatal(e)
		}
		if !r.BothDirections {
			t.Fatal("missing two-sided correspondence certificate")
		}
		if !curved {
			if r.Subdivisions != 1 {
				t.Fatal("affine unnecessary refinement")
			}
			for _, err := range r.SquaredErrors {
				if err.Sign() != 0 {
					t.Fatal("affine exact points must have zero error")
				}
			}
		} else if r.Subdivisions > 8 {
			t.Fatal("bilinear lost quadratic convergence", r.Subdivisions)
		}
	}
}
