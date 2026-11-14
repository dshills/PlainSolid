package geom

import (
	"context"
	"math/big"
)

// SurfaceAffineTriangleVertexContact proves that the intersection of two
// original triangular surface restrictions is exactly one specified vertex.
// The remaining vertices lie strictly on opposite sides of a proved affine
// covector plane. No tangent continuity, manifold fan or solid is inferred.
type SurfaceAffineTriangleVertexContact struct {
	sides      *SurfaceAffineTriangleSeparation
	vertices   [2]int
	point      [3]*big.Rat
	operations int
}

func (p *SurfaceAffineTriangleVertexContact) Source(i int) (*SurfacePatch, [3][2]*big.Rat, error) {
	if p == nil {
		return nil, [3][2]*big.Rat{}, ErrInvalid
	}
	return p.sides.Source(i)
}
func (p *SurfaceAffineTriangleVertexContact) Separator() SurfaceAffineTriangleSeparator {
	if p == nil {
		return SurfaceAffineTriangleSeparator{}
	}
	return p.sides.Separator()
}
func (p *SurfaceAffineTriangleVertexContact) Sides() [2]int {
	if p == nil {
		return [2]int{}
	}
	return p.sides.Sides()
}

// OtherVertexMargins are minima at the two NONSHARED vertices of each owner.
// They are not separation distances: the full triangles meet at Point().
func (p *SurfaceAffineTriangleVertexContact) OtherVertexMargins() [2]*big.Rat {
	if p == nil {
		return [2]*big.Rat{}
	}
	return p.sides.Margins()
}
func (p *SurfaceAffineTriangleVertexContact) Vertices() [2]int {
	if p == nil {
		return [2]int{}
	}
	return p.vertices
}
func (p *SurfaceAffineTriangleVertexContact) Point() [3]*big.Rat {
	var out [3]*big.Rat
	if p == nil {
		return out
	}
	for i := range out {
		out[i] = new(big.Rat).Set(p.point[i])
	}
	return out
}
func (p *SurfaceAffineTriangleVertexContact) ExactOperations() int {
	if p == nil {
		return 0
	}
	return p.operations
}

// CertifySurfaceAffineTriangleVertexContact accepts one plane vertex per owner
// and strict nonzero same-owner signs at the other two vertices. Affine
// barycentric positivity excludes every other parameter from the plane.
// Independent exact source evaluations must identify the specified model point;
// merely lying on the same plane is insufficient. All stages share one budget.
func CertifySurfaceAffineTriangleVertexContact(ctx context.Context, a, b *SurfacePatch, triangles [2][3][2]*big.Rat, vertices [2]int, q SurfaceAffineTriangleSeparator, o SurfaceAffineCoordinateOptions) (*SurfaceAffineTriangleVertexContact, error) {
	for _, i := range vertices {
		if i < 0 || i > 2 {
			return nil, ErrInvalid
		}
	}
	opts, e := patchOptions(o.SurfacePatchOptions)
	if e != nil {
		return nil, e
	}
	sides, e := certifySurfaceAffineTriangleSides(ctx, a, b, triangles, q, o, vertices)
	if e != nil {
		return nil, e
	}
	out := &SurfaceAffineTriangleVertexContact{sides: sides, vertices: vertices, operations: sides.operations}
	for owner, p := range sides.source {
		if out.operations >= opts.MaxOperations {
			return nil, ErrConvergence
		}
		uv := sides.triangles[owner][vertices[owner]]
		value, e := p.EvaluateRational(ctx, uv[0], uv[1], SurfacePatchEvaluationOptions{MaxOperations: opts.MaxOperations - out.operations, MaxCoefficientBits: opts.MaxCoefficientBits})
		if e != nil {
			return nil, e
		}
		out.operations += value.ExactOperations
		for j := range value.Point {
			if owner == 0 {
				out.point[j] = new(big.Rat).Set(value.Point[j])
			} else if out.point[j].Cmp(value.Point[j]) != 0 {
				return nil, ErrDomain
			}
		}
	}
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	return out, nil
}
