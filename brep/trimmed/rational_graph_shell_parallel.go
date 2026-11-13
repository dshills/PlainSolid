package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

type RationalGraphParallelSections struct {
	Sections [2]RationalGraphShellSection
	// ProjectedGap is the exact difference in the dropped coordinate at
	// equal values of the other two coordinates. World normal separation
	// is ProjectedGap/sqrt(NormalSquared).
	ProjectedGap, NormalSquared *big.Rat
	// Lower is the index of the input with the smaller dropped coordinate.
	Lower int
}

// SectionParallelRationalGraphSplineShell independently certifies two exact
// distinct parallel sections of the same original shell, using one embedding
// proof. Planes may be supplied in either order and orientation. No trimmed
// intermediate is reclassified. Section limits apply separately to each
// plane, and no partial table is returned.
func (s Solid) SectionParallelRationalGraphSplineShell(ctx context.Context, points [2][3]geom.Vec3, o RationalGraphShellSectionOptions) (RationalGraphParallelSections, error) {
	r, _, _, _, e := s.discoverParallelRationalGraphSplineShell(ctx, points, o)
	return r, e
}

func (s Solid) discoverParallelRationalGraphSplineShell(ctx context.Context, points [2][3]geom.Vec3, o RationalGraphShellSectionOptions) (RationalGraphParallelSections, []splineEmbeddingFace, [2]polynomialPlaneEquation, [2][]int, error) {
	var zero RationalGraphParallelSections
	var planes [2]polynomialPlaneEquation
	var signs [2][]int
	if o.MaxPieces == 0 {
		o.MaxPieces = 128
	}
	if ctx == nil || o.MaxPieces < 1 || o.MaxPieces > 1024 || o.Spans.MaxSpans < 0 || o.Spans.MaxSpans > 128 {
		return zero, nil, planes, signs, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, nil, planes, signs, e
	}
	for i, p := range points {
		for _, x := range p {
			if !x.IsFinite() {
				return zero, nil, planes, signs, geom.ErrInvalid
			}
		}
		q, e := polynomialPlaneFromPoints(p)
		if e != nil {
			return zero, nil, planes, signs, e
		}
		planes[i] = q
	}
	if planes[0].dropped != planes[1].dropped {
		return zero, nil, planes, signs, geom.ErrDomain
	}
	for i, a := range planes[0].normal {
		if a.Cmp(planes[1].normal[i]) != 0 {
			return zero, nil, planes, signs, geom.ErrDomain
		}
	}
	delta := new(big.Rat).Sub(planes[1].offset, planes[0].offset)
	if delta.Sign() == 0 {
		return zero, nil, planes, signs, geom.ErrDegenerate
	}
	out := RationalGraphParallelSections{ProjectedGap: delta, NormalSquared: new(big.Rat)}
	if delta.Sign() < 0 {
		out.Lower = 1
		delta.Neg(delta)
	}
	for _, a := range planes[0].normal {
		out.NormalSquared.Add(out.NormalSquared, new(big.Rat).Mul(a, a))
	}
	var faces []splineEmbeddingFace
	proof, e := s.validateSplineRectangleEmbedding(ctx, o.Spans.Faces, &faces)
	if e != nil {
		return zero, nil, planes, signs, e
	}
	if !proof.Complete {
		return zero, nil, planes, signs, geom.ErrConvergence
	}
	for i, p := range points {
		r, h, e := s.discoverRationalGraphShellSection(ctx, p, planes[i], faces, o)
		if e != nil {
			return zero, nil, planes, signs, e
		}
		out.Sections[i] = r
		signs[i] = h
	}
	return out, faces, planes, signs, nil
}
