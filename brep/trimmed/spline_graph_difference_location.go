package trimmed

import (
	"context"
	"errors"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// LocateDifferenceRational classifies a point where the vertical
// A-minus-B thickness is strictly positive or strictly negative. At an
// equal-height point the closed regularized difference may touch that
// graph or be absent locally. If the query is exactly at that height,
// this generic query returns ErrConvergence instead of guessing.
func (p SharedBaseSplineGraphMaterial) LocateDifferenceRational(ctx context.Context, point [3]*big.Rat,
	o geom.SurfacePatchEvaluationOptions) (parametric.Location, error) {
	if point[2] == nil {
		return parametric.Outside, geom.ErrInvalid
	}
	z, side, e := p.graphHeightsAt(ctx, [2]*big.Rat{point[0], point[1]}, o)
	if errors.Is(e, geom.ErrDomain) {
		return parametric.Outside, nil
	}
	if e != nil {
		return parametric.Outside, e
	}
	sign := z[1].Cmp(z[2])
	if sign <= 0 {
		if sign == 0 && point[2].Cmp(z[1]) == 0 {
			return parametric.Outside, geom.ErrConvergence
		}
		return parametric.Outside, nil
	}
	a, b := point[2].Cmp(z[2]), point[2].Cmp(z[1])
	if a < 0 || b > 0 {
		return parametric.Outside, nil
	}
	if a == 0 || b == 0 || side {
		return parametric.Boundary, nil
	}
	return parametric.Inside, nil
}

// LocateDifferenceRationalWithEllipse proves closure membership also
// along a certified elliptical contact. Both strict sign regions touch
// every point of the contained ellipse, so the limit graph is boundary.
func (p SharedBaseSplineGraphMaterial) LocateDifferenceRationalWithEllipse(ctx context.Context, point [3]*big.Rat,
	hint geom.GraphQuadraticEllipseHint, limits geom.GraphPointContactOptions, o geom.SurfacePatchEvaluationOptions) (parametric.Location, error) {
	location, e := p.LocateDifferenceRational(ctx, point, o)
	if !errors.Is(e, geom.ErrConvergence) {
		return location, e
	}
	proof, e := geom.CertifySplineGraphQuadraticEllipse(ctx, p.a, p.b, hint, limits)
	if e != nil {
		return parametric.Outside, e
	}
	if !proof.Proven {
		return parametric.Outside, geom.ErrConvergence
	}
	section, e := p.SectionEnvelopeVertical(ctx, [2]*big.Rat{point[0], point[1]}, parametric.Union, o)
	if e != nil {
		return parametric.Outside, e
	}
	if len(section.Faces) != 2 || point[2].Cmp(section.Upper) != 0 {
		return parametric.Outside, geom.ErrConvergence
	}
	return parametric.Boundary, nil
}

// LocateDifferenceRationalWithKnotPartition proves closure membership at a
// zero-height knot contact by exact adjacency to a positive open cell.
// This covers knot-aligned shared-base differences, including partially
// coincident faces. Other contact sets remain outside this proof.
func (p SharedBaseSplineGraphMaterial) LocateDifferenceRationalWithKnotPartition(ctx context.Context, point [3]*big.Rat,
	limits geom.GraphPointContactOptions, o geom.SurfacePatchEvaluationOptions) (parametric.Location, error) {
	location, e := p.LocateDifferenceRational(ctx, point, o)
	if !errors.Is(e, geom.ErrConvergence) {
		return location, e
	}
	z, _, e := p.graphHeightsAt(ctx, [2]*big.Rat{point[0], point[1]}, o)
	if e != nil {
		return parametric.Outside, e
	}
	if z[1].Cmp(z[2]) != 0 || point[2].Cmp(z[1]) != 0 {
		return parametric.Outside, geom.ErrConvergence
	}
	q, e := geom.PartitionSplineGraphContact(ctx, p.a, p.b, limits)
	if e != nil {
		return parametric.Outside, e
	}
	if !q.Complete {
		return parametric.Outside, geom.ErrConvergence
	}
	for _, cell := range q.Cells {
		d := cell.Domain
		if cell.Side > 0 && point[0].Cmp(ratInset(d.UMin)) >= 0 && point[0].Cmp(ratInset(d.UMax)) <= 0 &&
			point[1].Cmp(ratInset(d.VMin)) >= 0 && point[1].Cmp(ratInset(d.VMax)) <= 0 {
			return parametric.Boundary, nil
		}
	}
	return parametric.Outside, nil
}
