package geom

import (
	"context"
	"errors"
	"math/big"
)

// CertifyProjectedAdjacentSurfaceSeparation proves that two patches meet only
// at a shared full edge using one model coordinate pair. The projected edge
// must be straight, and the two patch projections must lie strictly on
// opposite sides of its supporting line away from that edge. The original
// three-dimensional edge curves must also agree exactly. A curved model
// edge can therefore qualify even when neither face is planar.
func CertifyProjectedAdjacentSurfaceSeparation(ctx context.Context, a, b *SurfacePatch, edgeA, edgeB SurfaceEdge, o SurfaceAdjacencyOptions) (SurfaceAdjacencyCertificate, error) {
	var r SurfaceAdjacencyCertificate
	if ctx == nil || a == nil || b == nil || edgeA > SurfaceEdgeVMax || edgeB > SurfaceEdgeVMax {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxCandidates < 0 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxCells < 0 || o.MaxDepth < 0 {
		return r, ErrInvalid
	}
	endpoints := func(p *SurfacePatch, side SurfaceEdge) ([2]Vec3, error) {
		d := p.Domain()
		u, v := [2]float64{d.UMin, d.UMax}, [2]float64{d.VMin, d.VMax}
		if side < SurfaceEdgeVMin {
			u[0], u[1] = u[side], u[side]
		} else {
			v[0], v[1] = v[side-2], v[side-2]
		}
		var result [2]Vec3
		for i := range result {
			left := o.MaxOperations - r.ExactOperations
			if left < 1 {
				return result, ErrConvergence
			}
			q, e := p.EvaluateRational(ctx, new(big.Rat).SetFloat64(u[i]), new(big.Rat).SetFloat64(v[i]), SurfacePatchEvaluationOptions{
				MaxOperations: left, MaxCoefficientBits: o.MaxCoefficientBits,
			})
			r.ExactOperations += q.ExactOperations
			if e != nil {
				return result, e
			}
			result[i], e = RepresentedExactPoint(q.Point)
			if e != nil {
				return result, e
			}
		}
		return result, nil
	}
	pa, e := endpoints(a, edgeA)
	if e != nil {
		return r, e
	}
	if pa[0] == pa[1] {
		return r, ErrConvergence
	}
	candidates := make([]Vec3, 0, 3)
	for axis := 0; axis < 3; axis++ {
		q := pa[0]
		switch axis {
		case 0:
			q.X = projectedAdjacencyThirdCoordinate(q.X)
		case 1:
			q.Y = projectedAdjacencyThirdCoordinate(q.Y)
		default:
			q.Z = projectedAdjacencyThirdCoordinate(q.Z)
		}
		candidates = append(candidates, q)
	}
	for _, q := range candidates {
		if o.MaxCandidates != 0 && r.Candidates >= o.MaxCandidates {
			return r, ErrConvergence
		}
		r.Candidates++
		var cs [2]SurfaceSupportingPlaneCertificate
		for j, p := range [2]*SurfacePatch{a, b} {
			left := o.MaxOperations - r.ExactOperations
			if left < 1 {
				return r, ErrConvergence
			}
			cells := o.MaxCells
			if cells > 0 {
				cells -= r.VisitedCells
				if cells < 1 {
					return r, ErrConvergence
				}
			}
			q, e := CertifySurfaceSupportingPlane(ctx, p, pa[0], pa[1], q, SurfaceSupportingPlaneOptions{
				MaxDepth: o.MaxDepth, MaxCells: cells, MaxOperations: left, MaxCoefficientBits: o.MaxCoefficientBits,
			})
			r.ExactOperations += q.ExactOperations
			r.VisitedCells += q.VisitedCells
			if e != nil && !errors.Is(e, ErrConvergence) && !errors.Is(e, ErrDegenerate) {
				return r, e
			}
			if e != nil || !q.Complete || q.Coincident || q.SideMask != 1<<[2]SurfaceEdge{edgeA, edgeB}[j] || q.CornerMask != 0 {
				break
			}
			cs[j] = q
		}
		if !cs[1].Complete || cs[0].HalfspaceSign != -cs[1].HalfspaceSign {
			continue
		}
		curves := [2]*SurfaceIsoCurve{}
		for j, p := range [2]*SurfacePatch{a, b} {
			side := [2]SurfaceEdge{edgeA, edgeB}[j]
			d := p.Domain()
			at := [4]float64{d.UMin, d.UMax, d.VMin, d.VMax}[side]
			left := o.MaxOperations - r.ExactOperations
			if left < 1 {
				return r, ErrConvergence
			}
			c, e := p.IsoCurve(int(side)/2, at, SurfacePatchOptions{MaxOperations: left, MaxCoefficientBits: o.MaxCoefficientBits})
			if e != nil {
				return r, e
			}
			r.ExactOperations += c.operations
			curves[j] = c
		}
		for _, reverse := range []bool{false, true} {
			left := o.MaxOperations - r.ExactOperations
			if left < 1 {
				return r, ErrConvergence
			}
			same, e := CompareExactCurvesContext(ctx, curves[0], curves[1], ExactCurveComparisonOptions{
				NormalizeDomains: true, ReverseB: reverse, MaxOperations: left, MaxCoefficientBits: o.MaxCoefficientBits,
			})
			r.ExactOperations += same.ExactOperations
			if e != nil {
				return r, e
			}
			if same.Complete && same.Equal {
				r.Complete, r.Separated, r.ReverseB = true, true, reverse
				r.A, r.B, r.PlanePoints = cs[0], cs[1], [3]Vec3{pa[0], pa[1], q}
				return r, nil
			}
		}
		return r, ErrConvergence
	}
	return r, ErrConvergence
}

func projectedAdjacencyThirdCoordinate(x float64) float64 {
	if x == 0 {
		return 1
	}
	return 0
}
