package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// SolidMixedFaceMeshOptions applies face limits once and exact model-edge
// comparison limits independently to each original coedge (at most256).
// Higher-degree SplineUV additionally performs one bounded exact restriction
// and one whole-UV identity check; those proof limits apply separately.
type SolidMixedFaceMeshOptions struct {
	Face           MixedExactMultiSpanMeshOptions
	ModelAgreement edgecheck.Options
}

// MixedFaceCoedgeSamples maps one original face/loop/use to its model edge.
// TrimParameters follow the original pcurve, independent of face reversal.
// ModelParameters are exact values in the stored edge's actual Range, including
// Use.Reversed. Vertices refer to the final display. FaceReversed records the
// physical boundary traversal reversal; sample arrays themselves remain in the
// normalized loop proof order, which may reverse a hole's original traversal.
type MixedFaceCoedgeSamples struct {
	Face, Loop, Use, Edge           int
	FaceReversed, UseReversed       bool
	TrimParameters, ModelParameters []*big.Rat
	Vertices                        []int
}
type CertifiedSolidMixedFaceMesh struct {
	CertifiedMixedExactCurvedMesh
	OriginalFace           int
	Coedges                []MixedFaceCoedgeSamples
	ModelEdgeUsesCertified int
}

// TessellateMixedFaceCertified certifies one stored spline face, preserving its
// exact represented affine placement and physical orientation. Straight uses,
// native ExactQuadraticUV/ExactBezierUV, and single-span degree<=4 SplineUV
// are supported when endpoints and complete model correspondence are proved.
// Each complete pcurve/model-edge correspondence is independently proved before
// meshing. Unsupported periodic/collapsed topology, curves, parameterizations or
// proof budgets reject atomically. This does not stitch neighboring faces or
// prove the whole Solid embedded. Nonbinary native corners are rounded only by
// the existing source/display certificate; no vertex-tolerance weld is used.
func (s Solid) TessellateMixedFaceCertified(ctx context.Context, face int, o SolidMixedFaceMeshOptions) (CertifiedSolidMixedFaceMesh, error) {
	var empty CertifiedSolidMixedFaceMesh
	if ctx == nil || face < 0 || face >= len(s.faces) {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	f := s.faces[face]
	patch, e := exactFaceSupportPatch(f, o.Face.Face.Boundary.Composition.SurfacePatchOptions)
	if e != nil {
		return empty, e
	}
	uses := append([][]Use{f.Outer}, f.Holes...)
	if len(uses) > 65 {
		return empty, geom.ErrInvalid
	}
	loops := make([][]PCurve, len(uses))
	count := 0
	for li, loop := range uses {
		if len(loop) < 2 || len(loop) > 256-count {
			return empty, geom.ErrInvalid
		}
		count += len(loop)
		for _, u := range loop {
			if u.Edge < 0 || u.Edge >= len(s.edges) {
				return empty, geom.ErrInvalid
			}
			edge := s.edges[u.Edge]
			if edge.Curve == nil || edge.Start == edge.End || !finite(edge.Range.Min) || !finite(edge.Range.Max) || edge.Range.Min >= edge.Range.Max {
				return empty, geom.ErrConvergence
			}
			curve, e := mixedStoredUseCurveContext(ctx, u, o.Face.Face.Boundary.Composition.SurfacePatchOptions)
			if e != nil {
				return empty, e
			}
			for j, t := range []*big.Rat{new(big.Rat), big.NewRat(1, 1)} {
				uv, e := mixedExactEvaluate(curve, t)
				if e != nil {
					return empty, e
				}
				expected := []geom.Vec2{u.UVStart, u.UVEnd}[j]
				for axis, value := range []float64{expected.X, expected.Y} {
					x, _ := uv[axis].Float64()
					if x != value {
						return empty, geom.ErrConvergence
					}
				}
			}
			proofUse := u
			switch q := proofUse.Curve.(type) {
			case *ExactQuadraticUV:
				proofUse.Curve = *q
			case *ExactBezierUV:
				proofUse.Curve = *q
			case *SplineUV:
				proofUse.Curve = *q
			}
			if q, ok := proofUse.Curve.(SplineUV); ok && q.Curve != nil && q.Curve.Degree() >= 3 {
				proofUse.Curve = curve
				limits := geom.SurfacePatchOptions{MaxOperations: o.ModelAgreement.MaxWork, MaxCoefficientBits: o.ModelAgreement.MaxBits}
				affine, e := mixedStoredSplineAffineIdentity(ctx, curve, u, limits)
				if e != nil {
					return empty, e
				}
				if affine {
					proofUse.Curve = nil
				}
			}
			proven := sameSplineTrim(edge, f.Surface, proofUse) || mixedFaceAffineRangeIdentity(edge, f.Surface, proofUse)
			if !proven {
				proven, e = compareSplineEdgeUse(ctx, edge, f.Surface, proofUse, o.ModelAgreement)
				if e != nil {
					return empty, e
				}
			}
			if !proven {
				return empty, geom.ErrConvergence
			}
			loops[li] = append(loops[li], curve)
		}
	}
	result, e := TessellateMixedExactMultiSpanRegionCertified(ctx, patch, loops[0], loops[1:], o.Face)
	if e != nil {
		return empty, e
	}
	out := CertifiedSolidMixedFaceMesh{CertifiedMixedExactCurvedMesh: result, OriginalFace: face, ModelEdgeUsesCertified: count}
	for _, sample := range result.UVRegion.Samples {
		u := uses[sample.Loop][sample.Trim]
		edge := s.edges[u.Edge]
		item := MixedFaceCoedgeSamples{Face: face, Loop: sample.Loop, Use: sample.Trim, Edge: u.Edge, FaceReversed: f.Reversed, UseReversed: u.Reversed, Vertices: append([]int(nil), sample.Vertices...)}
		for _, t := range sample.Parameters {
			item.TrimParameters = append(item.TrimParameters, new(big.Rat).Set(t))
			q := new(big.Rat).Set(t)
			if u.Reversed {
				q.Sub(big.NewRat(1, 1), q)
			}
			item.ModelParameters = append(item.ModelParameters, new(big.Rat).Add(ratInset(edge.Range.Min), new(big.Rat).Mul(new(big.Rat).Sub(ratInset(edge.Range.Max), ratInset(edge.Range.Min)), q)))
		}
		out.Coedges = append(out.Coedges, item)
	}
	for i := range out.CertifiedCurvedQuadraticTrimMesh.OriginalFace {
		out.CertifiedCurvedQuadraticTrimMesh.OriginalFace[i] = face
	}
	if f.Reversed {
		for i := range out.Mesh.Triangles {
			out.Mesh.Triangles[i][1], out.Mesh.Triangles[i][2] = out.Mesh.Triangles[i][2], out.Mesh.Triangles[i][1]
		}
		out.BoundaryLoops = append([][]int(nil), out.BoundaryLoops...)
		for i, loop := range out.BoundaryLoops {
			x := append([]int(nil), loop...)
			for j, k := 0, len(x)-1; j < k; j, k = j+1, k-1 {
				x[j], x[k] = x[k], x[j]
			}
			out.BoundaryLoops[i] = x
		}
	}
	return out, nil
}
func mixedStoredUseCurve(u Use) (PCurve, error) {
	return mixedStoredUseCurveContext(context.Background(), u, geom.SurfacePatchOptions{})
}

func mixedStoredUseCurveContext(ctx context.Context, u Use, o geom.SurfacePatchOptions) (PCurve, error) {
	if ctx == nil {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	switch q := u.Curve.(type) {
	case ExactQuadraticUV, ExactBezierUV:
		return q, nil
	case *ExactQuadraticUV:
		if q != nil {
			return *q, nil
		}
	case *ExactBezierUV:
		if q != nil {
			return *q, nil
		}
	case *SplineUV:
		if q != nil {
			return mixedStoredUseCurveContext(ctx, Use{Curve: *q}, o)
		}
	case SplineUV:
		if q.Curve != nil && q.Curve.Degree() >= 3 {
			return mixedStoredHigherSplineUV(ctx, q, o)
		}
		h, e := quadraticTrimSourceControls(q)
		if e != nil {
			return nil, e
		}
		return NewRationalEndpointQuadraticUV(h)
	case nil:
		h := [3][3]*big.Rat{}
		for k, p := range []geom.Vec2{u.UVStart, u.UVEnd} {
			if !finite(p.X) || !finite(p.Y) {
				return nil, geom.ErrInvalid
			}
			h[2*k] = [3]*big.Rat{ratInset(p.X), ratInset(p.Y), big.NewRat(1, 1)}
		}
		for k := 0; k < 2; k++ {
			h[1][k] = new(big.Rat).Quo(new(big.Rat).Add(h[0][k], h[2][k]), big.NewRat(2, 1))
		}
		h[1][2] = big.NewRat(1, 1)
		return NewExactQuadraticUV(h)
	}
	return nil, geom.ErrConvergence
}

// Exact affine UV correspondence handles any finite stored model range without
// broadening generic spline-edge comparison. Equal endpoints of two affine maps
// prove their complete equality, on the same represented original surface/map.
func mixedFaceAffineRangeIdentity(edge parametric.Edge, surface geom.Surface, use Use) bool {
	model, ok := edge.Curve.(surfaceTrimCurve)
	if !ok || model.curve != nil || use.Curve != nil {
		return false
	}
	a, ma := parametric.SurfaceMapping(model.surface)
	b, mb := parametric.SurfaceMapping(surface)
	sa, ok := a.(*geom.BSplineSurface)
	sb, other := b.(*geom.BSplineSurface)
	if !ok || !other || ma != mb || !geom.SameSplineSurface(sa, sb) {
		return false
	}
	ts := [2]float64{edge.Range.Min, edge.Range.Max}
	if use.Reversed {
		ts[0], ts[1] = ts[1], ts[0]
	}
	for j, point := range []geom.Vec2{use.UVStart, use.UVEnd} {
		for axis, value := range []float64{point.X, point.Y} {
			x, y := []float64{model.a.X, model.a.Y}[axis], []float64{model.b.X, model.b.Y}[axis]
			if !finite(x) || !finite(y) || !finite(value) {
				return false
			}
			want := new(big.Rat).Add(ratInset(x), new(big.Rat).Mul(new(big.Rat).Sub(ratInset(y), ratInset(x)), ratInset(ts[j])))
			if want.Cmp(ratInset(value)) != 0 {
				return false
			}
		}
	}
	return true
}
