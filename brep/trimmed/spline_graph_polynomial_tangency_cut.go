package trimmed

import (
	"context"
	"fmt"

	"plainsolid/geom"
)

// PolynomialTangencyCut retains two independently embedded, outward native
// shells whose closures share exactly the original branch tangency. The
// components have separate topology and vertex namespaces.
type PolynomialTangencyCut struct {
	pointCut *PolynomialNodeCut
}

// Components returns the two native solids in their original sector order.
func (r *PolynomialTangencyCut) Components() [2]Solid {
	if r == nil {
		return [2]Solid{}
	}
	return r.pointCut.Components()
}

// Contact returns the exact point and its local vertex ID in each component.
func (r *PolynomialTangencyCut) Contact() PolynomialNodeComponentContact {
	if r == nil {
		return PolynomialNodeComponentContact{}
	}
	return r.pointCut.Contact()
}

// CutSplineGraphPolynomialTangencyComponents cuts the two positive shared-
// base graphs A minus B across one complete original polynomial tangency.
// It accepts only two closed components whose original UV projections and
// zero height gap prove sole contact at a losslessly represented vertex.
func CutSplineGraphPolynomialTangencyComponents(ctx context.Context, base, a, b *geom.BSplineSurface, axis int, tolerance float64, o PolynomialNodeCutOptions) (*PolynomialTangencyCut, error) {
	if ctx == nil || base == nil || a == nil || b == nil || axis < 0 || axis > 1 ||
		!finite(tolerance) || tolerance <= 0 || o.Section.Domain != (geom.ParameterRectangle{}) {
		return nil, geom.ErrInvalid
	}
	prepared, e := preparePolynomialTangency(ctx, base, a, b, axis, o.Section, o.Proof)
	if e != nil {
		return nil, e
	}
	var positive []int
	for i, region := range prepared.proof.Regions {
		if region.HeightSide > 0 {
			positive = append(positive, i)
		}
	}
	if len(positive) != 2 {
		return nil, geom.ErrConvergence
	}
	sectorPair := [2]int(positive)
	if sectorPair != ([2]int{0, 3}) && sectorPair != ([2]int{1, 2}) {
		return nil, geom.ErrConvergence
	}
	// The two exterior projections can meet only where the branch ordinates
	// agree. The two between-branch projections meet only at the shared free
	// bound, where the band contracts to one point. The height gap vanishes
	// there, so in either case the closed layers share only the exact XYZ.
	contact := prepared.proof.Vertices[4]
	r := &PolynomialNodeCut{
		sources: [3]*geom.BSplineSurface{base, a, b}, axis: axis, tolerance: tolerance,
		contact: PolynomialNodeComponentContact{Parameters: contact.Parameters, Point: contact.Point, Sectors: sectorPair},
	}
	evaluate := exactGraphVertexEvaluator(ctx, o.Proof)
	for i, index := range sectorPair {
		loop := prepared.loops[index]
		if len(loop) == 3 {
			// The between-branch perimeter arc collapses at both ends. Its
			// exact midpoint supplies the upright dividing the two wall loops.
			pieces, e := splitGraphSideAtMidpoint(loop[2])
			if e != nil {
				return nil, e
			}
			loop = []openSectionPiece{loop[0], loop[1], pieces[0], pieces[1]}
		}
		part, history, e := assembleOpenSplineGraphPair(a, b, 0, true, tolerance, loop, evaluate)
		if e != nil {
			return nil, e
		}
		if e = bindPolynomialTangencyCutLayer(ctx, part, history, [2]*geom.BSplineSurface{a, b}, loop, o.Proof); e != nil {
			return nil, fmt.Errorf("trimmed: polynomial tangency Cut sector %d: %w", index, e)
		}
		v, e := polynomialCutContactVertex(part, history, prepared.xyz[4])
		if e != nil {
			return nil, e
		}
		r.components[i], r.contact.Vertices[i] = part, v
	}
	if e = validatePolynomialPointCutIdentity(r); e != nil {
		return nil, e
	}
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	return &PolynomialTangencyCut{pointCut: r}, nil
}

func bindPolynomialTangencyCutLayer(ctx context.Context, part Solid, h OpenSectionHistory, roofs [2]*geom.BSplineSurface, loop []openSectionPiece, proof geom.GraphPointContactOptions) error {
	if len(h.ContourEdges) != 2 {
		return geom.ErrConvergence
	}
	for i, index := range [2]int{h.SplineFace, h.PlaneFace} {
		if e := bindPolynomialTangencyFace(part.faces[index], roofs[i], i == 1, loop); e != nil {
			return e
		}
	}
	for i, id := range h.ContourEdges {
		p := loop[i]
		model, ok := part.edges[id].Curve.(surfaceTrimCurve)
		if !ok || model.surface != roofs[0] || model.a != p.a || model.b != p.b || !matchesTangencyTrim(model.curve, p.c) ||
			part.faces[h.SplineFace].Outer[i].Edge != id || part.faces[h.PlaneFace].Outer[i].Edge != id {
			return geom.ErrConvergence
		}
	}
	// Both cap loops have been bound to the signed source sector. Exact
	// continuous owner agreement and a connected vertex link complete its
	// native embedding proof without a generic tangent trim exception.
	return validatePolynomialCutBoundary(ctx, part, h, proof)
}
