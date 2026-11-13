package trimmed

import (
	"context"
	"errors"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func proveSplineShellsTouchOnly(ctx context.Context, a, b []splineEmbeddingFace, o geom.SurfaceSupportingPlaneOptions) (bool, error) {
	// Try original face planes. Exact support tests include every patch;
	// unlike an AABB touch, this also works on oblique shells.
	for _, pair := range [2][2][]splineEmbeddingFace{{a, b}, {b, a}} {
		for _, f := range pair[0] {
			if e := ctx.Err(); e != nil {
				return false, e
			}
			if f.planeSign == 0 {
				continue
			}
			signs := [2]int{}
			proved := true
			for i, group := range pair {
				for _, p := range group {
					q, e := geom.CertifySurfaceSupportingPlane(ctx, p.patch, f.plane[0], f.plane[1], f.plane[2], o)
					if e != nil && !errors.Is(e, geom.ErrConvergence) {
						return false, e
					}
					if !q.Complete || q.HalfspaceSign != 0 && signs[i] != 0 && q.HalfspaceSign != signs[i] {
						proved = false
						break
					}
					if q.HalfspaceSign != 0 {
						signs[i] = q.HalfspaceSign
					}
				}
				if !proved {
					break
				}
			}
			if proved && signs[0] != 0 && signs[1] == -signs[0] {
				return true, nil
			}
		}
	}
	return false, nil
}

func regularizedTouchResult(a, b Solid, op parametric.BooleanOp) (Solid, SplineShellCombineHistory) {
	h := SplineShellCombineHistory{FacesA: make([][]int, len(a.faces)), FacesB: make([][]int, len(b.faces)), Method: "boundary-contact"}
	if op == parametric.Intersection {
		return Solid{}, h
	}
	for i := range h.FacesA {
		h.FacesA[i] = []int{i}
	}
	return a, h
}
