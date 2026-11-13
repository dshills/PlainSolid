package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// ExactSplineSeamLineSection preserves one selected model edge and both
// original owners. Contacts are ordered along that edge. Topological
// endpoint hits carry an original vertex index; all others use -1.
type ExactSplineSeamLineSection struct {
	Edge      int
	Faces     [2]int
	Section   ExactQuadraticModelLineSection
	HitVertex []int
}

// IntersectExactSplineSeamLine finds all contacts with the selected native
// rational quadratic spline edge. It verifies both original owner charts
// before using the authoritative model curve. This is a boundary-edge
// query, not a shell crossing or a classification of nearby face contacts.
func (s Solid) IntersectExactSplineSeamLine(ctx context.Context, edge int, origin, direction geom.Vec3, limits [2]*big.Rat,
	compose geom.SurfaceCurveAgreementOptions, o geom.ComposedCurveLineOptions) (ExactSplineSeamLineSection, error) {
	var empty ExactSplineSeamLineSection
	model, owners, e := algebraicSeamSource(ctx, s, edge, compose)
	if e != nil {
		return empty, e
	}
	m := s.edges[edge]
	out := ExactSplineSeamLineSection{Edge: edge, Faces: [2]int{owners[0].face, owners[1].face}}
	out.Section, e = model.curve.IntersectModelFiniteLine(ctx, model.patch, origin, direction, limits, compose, o)
	if e != nil {
		return empty, e
	}
	for _, h := range out.Section.Hits {
		i, t := -1, h.Event.Interval()
		if t.Lower.Sign() == 0 && t.Upper.Sign() == 0 {
			i = m.Start
		} else if t.Lower.Cmp(big.NewRat(1, 1)) == 0 && t.Upper.Cmp(big.NewRat(1, 1)) == 0 {
			i = m.End
		}
		out.HitVertex = append(out.HitVertex, i)
	}
	return out, nil
}
