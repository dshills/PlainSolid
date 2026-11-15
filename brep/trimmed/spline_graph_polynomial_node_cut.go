package trimmed

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// PolynomialNodeCutOptions bounds the original graph and contact proofs.
type PolynomialNodeCutOptions struct {
	Section intersect.SplinePlaneOptions
	Proof   geom.GraphPointContactOptions
}

// PolynomialNodeComponentContact names the exact shared point of two Cut
// components. Vertices are local to their respective components. Sectors
// are indices into the certified counterclockwise node cycle.
type PolynomialNodeComponentContact struct {
	Parameters [2]*big.Rat
	Point      [3]*big.Rat
	Vertices   [2]int
	Sectors    [2]int
}

// PolynomialNodeCut retains two independently embedded, outward native shells
// whose closed materials meet only at an original point. The components retain
// separate topology and vertex namespaces.
type PolynomialNodeCut struct {
	sources    [3]*geom.BSplineSurface
	components [2]Solid
	contact    PolynomialNodeComponentContact
	axis       int
	tolerance  float64
}

// Components returns the two native solids in node-sector order.
func (r *PolynomialNodeCut) Components() [2]Solid {
	if r == nil {
		return [2]Solid{}
	}
	return r.components
}

// Contact returns the exact point and both original vertex identities.
// All rational coordinates in the report are caller-owned copies.
func (r *PolynomialNodeCut) Contact() PolynomialNodeComponentContact {
	if r == nil || r.sources[0] == nil {
		return PolynomialNodeComponentContact{}
	}
	out := r.contact
	for i, x := range out.Parameters {
		out.Parameters[i] = new(big.Rat).Set(x)
	}
	for i, x := range out.Point {
		out.Point[i] = new(big.Rat).Set(x)
	}
	return out
}

// CutSplineGraphPolynomialNodeComponents cuts A by B over their strictly
// positive shared base. It proves two complete cubic/quartic branch factors,
// one interior transverse node, and exact side ownership. The two positive
// sectors yield separate embedded shells. Their sole mutual contact must be
// the exactly representable node and a vertex of both components.
func CutSplineGraphPolynomialNodeComponents(ctx context.Context, base, a, b *geom.BSplineSurface, axis int, tolerance float64, o PolynomialNodeCutOptions) (*PolynomialNodeCut, error) {
	if ctx == nil || base == nil || a == nil || b == nil || axis < 0 || axis > 1 ||
		!finite(tolerance) || tolerance <= 0 || o.Section.Domain != (geom.ParameterRectangle{}) {
		return nil, geom.ErrInvalid
	}
	patches, e := prepareSharedBaseGraphContact(ctx, base, a, b, o.Section)
	if e != nil {
		return nil, e
	}
	node, e := geom.DiscoverSplineGraphPolynomialNode(ctx, patches[0], patches[1], axis, o.Proof)
	if e != nil {
		return nil, e
	}
	if !node.Proven || node.Domain != (geom.ParameterRectangle{UMax: 1, VMax: 1}) || node.Graph.ProjectionOrientation != 1 {
		return nil, geom.ErrConvergence
	}
	loops, e := polynomialNodeSectorLoops(node)
	if e != nil {
		return nil, e
	}
	center, e := geom.RepresentedExactPoint(node.Point)
	if e != nil {
		return nil, e
	}
	var positive []int
	for i, sector := range node.Sectors {
		if sector.HeightSide > 0 {
			positive = append(positive, i)
		}
	}
	if len(positive) != 2 || positive[1]-positive[0] != 2 {
		return nil, geom.ErrConvergence
	}
	// The unique branch crossing makes opposite closed sectors intersect in
	// exactly its XY node. For the two wedges this follows from the opposite
	// free-coordinate bounds. For the two exterior sectors it follows from
	// max(f,g) >= min(f,g) with equality only there. The height gap vanishes
	// at that XY, so their closed solid materials share a single XYZ point.
	r := &PolynomialNodeCut{
		sources: [3]*geom.BSplineSurface{base, a, b}, axis: axis, tolerance: tolerance,
		contact: PolynomialNodeComponentContact{Parameters: node.Parameters, Point: node.Point, Sectors: [2]int(positive)},
	}
	evaluate := exactGraphVertexEvaluator(ctx, o.Proof)
	for i, index := range positive {
		loop := loops[index]
		if len(loop) == 3 {
			// A wedge's only square side has equal roof heights at both ends.
			// Its midpoint retains a noncollapsed upright for the two walls.
			p, e := splitGraphSideAtMidpoint(loop[1])
			if e != nil {
				return nil, e
			}
			loop = []openSectionPiece{loop[0], p[0], p[1], loop[2]}
		}
		part, history, e := assembleOpenSplineGraphPair(a, b, 0, true, tolerance, loop, evaluate)
		if e != nil {
			return nil, e
		}
		vertex, e := validatePolynomialNodeCutComponent(ctx, part, history, center, node.Sectors[index].Witness, o.Proof)
		if e != nil {
			return nil, fmt.Errorf("trimmed: polynomial node Cut sector %d: %w", index, e)
		}
		r.components[i], r.contact.Vertices[i] = part, vertex
	}
	if e = validatePolynomialPointCutIdentity(r); e != nil {
		return nil, e
	}
	return r, nil
}

func validatePolynomialPointCutIdentity(r *PolynomialNodeCut) error {
	for i, p := range r.components[0].vertices {
		for j, q := range r.components[1].vertices {
			if p == q && r.contact.Vertices != ([2]int{i, j}) {
				return geom.ErrConvergence
			}
		}
	}
	return nil
}

func validatePolynomialNodeCutComponent(ctx context.Context, part Solid, history OpenSectionHistory, center geom.Vec3, witness [2]*big.Rat, proof geom.GraphPointContactOptions) (int, error) {
	if len(history.ContourEdges) != 2 {
		return -1, geom.ErrConvergence
	}
	if e := validatePolynomialCutLayer(ctx, part, history, witness, proof); e != nil {
		return -1, e
	}
	return polynomialCutContactVertex(part, history, center)
}

func validatePolynomialCutLayer(ctx context.Context, part Solid, history OpenSectionHistory, witness [2]*big.Rat, proof geom.GraphPointContactOptions) error {
	if e := validatePolynomialCutBoundary(ctx, part, history, proof); e != nil {
		return e
	}
	// Both roof trims use this same UV loop. Identity XY and strict graph
	// ordering in the sector interior give an embedded positive layer. The
	// walls are its exact upright fibers over the original perimeter arcs.
	location, loops, e := LocateRationalInFaceTrim(ctx, part.faces[history.SplineFace], witness, trimvalidate.Options{})
	if e != nil {
		return e
	}
	if loops.Status != trimvalidate.Certified || location != trimvalidate.Inside {
		return geom.ErrConvergence
	}
	return nil
}

func validatePolynomialCutBoundary(ctx context.Context, part Solid, history OpenSectionHistory, proof geom.GraphPointContactOptions) error {
	topology := part.Topology()
	if len(topology.Shells) != 1 || topology.EulerCharacteristic != 2 || topology.CollapsedEdges != 0 ||
		part.faces[history.SplineFace].Reversed || !part.faces[history.PlaneFace].Reversed {
		return geom.ErrConvergence
	}
	if e := affineMaterialVertexLinks(ctx, part); e != nil {
		return e
	}
	return validateExactGraphPartitionEdges(ctx, part, proof)
}

func polynomialCutContactVertex(part Solid, history OpenSectionHistory, center geom.Vec3) (int, error) {
	vertex := -1
	for i, p := range part.vertices {
		if p != center {
			continue
		}
		if vertex >= 0 {
			return -1, geom.ErrConvergence
		}
		vertex = i
	}
	if vertex < 0 {
		return -1, geom.ErrConvergence
	}
	degree := 0
	for _, edge := range part.edges {
		if edge.Start == vertex || edge.End == vertex {
			degree++
		}
	}
	if degree != 2 {
		return -1, geom.ErrConvergence
	}
	incident := 0
	for _, id := range history.ContourEdges {
		edge := part.edges[id]
		if edge.Start == vertex || edge.End == vertex {
			incident++
		}
	}
	if incident != 2 {
		return -1, geom.ErrConvergence
	}
	return vertex, nil
}
