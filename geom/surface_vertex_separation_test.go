package geom

import (
	"context"
	"errors"
	"testing"
)

func TestSurfaceVertexSeparationWarpedAndOppositeCorners(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u * v} })
	for _, reverse := range []bool{false, true} {
		b := arcTestPatch(t, func(u, v float64) Vec3 {
			if reverse {
				u, v = 1-u, 1-v
			}
			return Vec3{-u, -v, 2 * u * v}
		})
		corner := 0
		if reverse {
			corner = 2
		}
		r, e := CertifySurfaceVertexSeparation(context.Background(), a, b, 0, corner, SurfaceVertexSeparationOptions{})
		if e != nil || !r.Complete || !r.Separated || r.Point != (Vec3{}) || r.A.CornerMask != 1 || r.B.CornerMask != 1<<corner || r.A.HalfspaceSign != -r.B.HalfspaceSign {
			t.Fatal(reverse, r, e)
		}
	}
}
func TestSurfaceVertexSeparationRejectsEdgeAndOverlap(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u * v} })
	for _, b := range []*SurfacePatch{a, arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{-u, v, 2 * u * v} })} {
		r, e := CertifySurfaceVertexSeparation(context.Background(), a, b, 0, 0, SurfaceVertexSeparationOptions{MaxDepth: 3, MaxCells: 256, MaxOperations: 500000})
		if !errors.Is(e, ErrConvergence) || r.Complete || r.Separated {
			t.Fatal(r, e)
		}
	}
}
func TestSurfaceVertexSeparationLimits(t *testing.T) {
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, u * v} })
	b := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{-u, -v, u * v} })
	for _, o := range []SurfaceVertexSeparationOptions{{MaxOperations: 1}, {MaxCandidates: 1}, {MaxCells: 1}} {
		r, e := CertifySurfaceVertexSeparation(context.Background(), a, b, 0, 0, o)
		if !errors.Is(e, ErrConvergence) || r.Complete {
			t.Fatal(r, e)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e := CertifySurfaceVertexSeparation(ctx, a, b, 0, 0, SurfaceVertexSeparationOptions{})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	_, e = CertifySurfaceVertexSeparation(context.Background(), a, b, 4, 0, SurfaceVertexSeparationOptions{})
	if !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
}
