package trimmed

import (
	"context"
	"fmt"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// ThreePolynomialNodeHistory maps every original branch piece and arrangement
// vertex to native topology. Edges lists each original piece's native
// subdivisions in increasing free order. Midpoints indexes the two interior
// lenses in increasing free order and their sides in original branch order.
type ThreePolynomialNodeHistory struct {
	Arrangement geom.SplineGraphThreePolynomialNodes
	Edges       [8][]int
	Vertices    [7]int
	Midpoints   [2][2]int
}

func assembleThreePolynomialNodeGraphs(ctx context.Context, base, a, b *geom.BSplineSurface, op parametric.BooleanOp, tolerance float64, arrangement geom.SplineGraphThreePolynomialNodes, proof geom.GraphPointContactOptions) (Solid, SplineGraphBooleanHistory, error) {
	fail := func(e error) (Solid, SplineGraphBooleanHistory, error) {
		return Solid{}, SplineGraphBooleanHistory{TopA: -1, TopB: -1, Base: -1}, e
	}
	if !arrangement.Proven || arrangement.Domain != (geom.ParameterRectangle{UMax: 1, VMax: 1}) || arrangement.Graph.ProjectionOrientation != 1 {
		return fail(geom.ErrConvergence)
	}
	if op == parametric.Difference {
		return fail(fmt.Errorf("trimmed: three polynomial nodes bound touching Cut components: %w", geom.ErrDegenerate))
	}
	uv, xyz, e := arrangement.RepresentedVertices()
	if e != nil {
		return fail(e)
	}
	pieces, e := polynomialNodePieces(arrangement.Edges[:], uv[:])
	if e != nil {
		return fail(e)
	}
	loops, e := polynomialNodeRegionLoops(arrangement.Regions[:], uv[:], pieces)
	if e != nil {
		return fail(e)
	}
	solid, history, e := assemblePolynomialNodeEnvelope(ctx, base, a, b, op, tolerance, arrangement.Regions[:], loops, proof)
	if e != nil {
		return fail(e)
	}
	binding, e := bindPolynomialNodeEdges(solid, history.ContourEdges, arrangement.Edges[:], xyz[:], pieces)
	if e != nil {
		return fail(e)
	}
	history.ThreePolynomialNodes = &ThreePolynomialNodeHistory{
		Arrangement: arrangement,
		Edges:       [8][]int(binding.edges),
		Vertices:    [7]int(binding.vertices),
		Midpoints:   [2][2]int{{binding.midpoints[1], binding.midpoints[5]}, {binding.midpoints[2], binding.midpoints[6]}},
	}
	return solid, history, nil
}
