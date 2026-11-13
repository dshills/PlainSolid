package geom

import (
	"context"
	"errors"
	"testing"
)

func TestAdjacentWarpedSurfacesExactSeparation(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u * v} })
	for _, reverse := range []bool{false, true} {
		b := arcTestPatch(t, func(u, v float64) Vec3 {
			if reverse {
				v = 1 - v
			}
			return Vec3{-u, v, 2 * u * v}
		})
		r, e := CertifyAdjacentSurfaceSeparation(context.Background(), a, b, SurfaceEdgeUMin, SurfaceEdgeUMin, SurfaceAdjacencyOptions{})
		if e != nil || !r.Complete || !r.Separated || r.ReverseB != reverse || r.A.HalfspaceSign != -r.B.HalfspaceSign || r.A.SideMask != 1 || r.B.SideMask != 1 {
			t.Fatal(reverse, r, e)
		}
	}
}
func TestAdjacentCurvedPlanarEdge(t *testing.T) {
	patch := func(sign float64) *SurfacePatch {
		poles := make([]Vec3, 6)
		for i := 0; i < 2; i++ {
			for j := 0; j < 3; j++ {
				u, v := float64(i), float64(j)/2
				z := 0.0
				if j == 2 {
					z = 1
				}
				poles[i*3+j] = Vec3{sign * u, v, z + (2-sign)*u*v}
			}
		}
		s, e := NewBSplineSurface(1, 2, 2, 3, poles, []float64{0, 0, 1, 1}, []float64{0, 0, 0, 1, 1, 1}, nil)
		if e != nil {
			t.Fatal(e)
		}
		p, e := NewSurfacePatch(s, SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		return p
	}
	r, e := CertifyAdjacentSurfaceSeparation(context.Background(), patch(1), patch(-1), SurfaceEdgeUMin, SurfaceEdgeUMin, SurfaceAdjacencyOptions{})
	if e != nil || !r.Complete || !r.Separated || r.Candidates != 1 {
		t.Fatal(r, e)
	}
}
func TestAdjacentSurfaceSeparationRejectsAndLimits(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u * v} })
	r, e := CertifyAdjacentSurfaceSeparation(context.Background(), a, a, SurfaceEdgeUMin, SurfaceEdgeUMin, SurfaceAdjacencyOptions{MaxDepth: 2, MaxCells: 128})
	if !errors.Is(e, ErrConvergence) || r.Complete {
		t.Fatal(r, e)
	}
	b := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{-u, v, 2 * u * v} })
	r, e = CertifyAdjacentSurfaceSeparation(context.Background(), a, b, SurfaceEdgeUMin, SurfaceEdgeUMin, SurfaceAdjacencyOptions{MaxOperations: 1})
	if !errors.Is(e, ErrConvergence) || r.Complete {
		t.Fatal(r, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e = CertifyAdjacentSurfaceSeparation(ctx, a, b, SurfaceEdgeUMin, SurfaceEdgeUMin, SurfaceAdjacencyOptions{})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	_, e = CertifyAdjacentSurfaceSeparation(context.Background(), a, b, SurfaceEdgeUMax, SurfaceEdgeUMax, SurfaceAdjacencyOptions{})
	if !errors.Is(e, ErrConvergence) {
		t.Fatal(e)
	}
}

func TestAdjacentNonplanarEdgeRemainsUnresolved(t *testing.T) {
	build := func(sign float64) *SurfacePatch {
		edge := []Vec3{{}, {1, 0, 0}, {0, 1, 0}, {0, 0, 1}}
		poles := make([]Vec3, 8)
		copy(poles, edge)
		for j, p := range edge {
			poles[4+j] = p.Add(Vec3{sign, sign, sign})
		}
		s, _ := NewBSplineSurface(1, 3, 2, 4, poles, []float64{0, 0, 1, 1}, []float64{0, 0, 0, 0, 1, 1, 1, 1}, nil)
		p, _ := NewSurfacePatch(s, SurfacePatchOptions{})
		return p
	}
	r, e := CertifyAdjacentSurfaceSeparation(context.Background(), build(1), build(-1), SurfaceEdgeUMin, SurfaceEdgeUMin, SurfaceAdjacencyOptions{MaxDepth: 2, MaxCells: 128, MaxOperations: 500000})
	if !errors.Is(e, ErrConvergence) || r.Complete || r.Separated {
		t.Fatal(r, e)
	}
}
