package trimmed

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// TwoPolynomialNodeHistory maps the certified source arrangement to native
// topology. Edges lists the native pieces of each source edge in increasing
// free-coordinate order. Midpoints indexes the two middle-branch vertices
// in original branch order.
type TwoPolynomialNodeHistory struct {
	Arrangement geom.SplineGraphTwoPolynomialNodes
	Edges       [6][]int
	Vertices    [6]int
	Midpoints   [2]int
}

func assembleTwoPolynomialNodeGraphs(ctx context.Context, base, a, b *geom.BSplineSurface, op parametric.BooleanOp, tolerance float64, arrangement geom.SplineGraphTwoPolynomialNodes, proof geom.GraphPointContactOptions) (Solid, SplineGraphBooleanHistory, error) {
	fail := func(e error) (Solid, SplineGraphBooleanHistory, error) {
		return Solid{}, SplineGraphBooleanHistory{TopA: -1, TopB: -1, Base: -1}, e
	}
	if !arrangement.Proven || arrangement.Domain != (geom.ParameterRectangle{UMax: 1, VMax: 1}) || arrangement.Graph.ProjectionOrientation != 1 {
		return fail(geom.ErrConvergence)
	}
	if op == parametric.Difference {
		return fail(fmt.Errorf("trimmed: two polynomial nodes bound touching Cut components: %w", geom.ErrDegenerate))
	}
	uv, xyz, e := arrangement.RepresentedVertices()
	if e != nil {
		return fail(e)
	}
	pieces, e := twoPolynomialNodePieces(arrangement, uv)
	if e != nil {
		return fail(e)
	}
	regions := make([]graphEnvelopeRegion, len(arrangement.Regions))
	for i, region := range arrangement.Regions {
		var loop []openSectionPiece
		for _, use := range region.Edges {
			chain := pieces[use.Edge]
			if use.Reversed {
				for j := len(chain) - 1; j >= 0; j-- {
					p := chain[j]
					loop = append(loop, openSectionPiece{a: p.b, b: p.a, c: reversePCurve(p.c), side: -1})
				}
			} else {
				loop = append(loop, chain...)
			}
		}
		if p := region.Perimeter; p != nil {
			if p.ParameterDirection != 1 {
				return fail(geom.ErrConvergence)
			}
			arc, e := saddleSquareArc(uv[p.Start], uv[p.End])
			if e != nil {
				return fail(e)
			}
			loop = append(loop, arc...)
		}
		regions[i] = graphEnvelopeRegion{loop: loop, first: (region.HeightSide > 0) == (op == parametric.Union)}
	}
	solid, history, e := assembleSplineEnvelope(ctx, base, a, b, regions, tolerance, exactGraphVertexEvaluator(ctx, proof))
	if e != nil {
		return fail(e)
	}
	if e = validateExactGraphPartitionEdges(ctx, solid, proof); e != nil {
		return fail(e)
	}
	for i, region := range arrangement.Regions {
		location, r, e := LocateRationalInFaceTrim(ctx, solid.faces[i], region.Witness, trimvalidate.Options{})
		if e != nil {
			return fail(e)
		}
		if r.Status != trimvalidate.Certified || location != trimvalidate.Inside {
			return fail(fmt.Errorf("trimmed: two polynomial node region %d: %s: %w", i, r.Reason, geom.ErrConvergence))
		}
	}
	if e = affineMaterialVertexLinks(ctx, solid); e != nil {
		return fail(e)
	}
	mapping, e := bindTwoPolynomialNodeEdges(solid, history.ContourEdges, arrangement, xyz, pieces)
	if e != nil {
		return fail(e)
	}
	history.TwoPolynomialNodes = mapping
	return solid, history, nil
}

func twoPolynomialNodePieces(arrangement geom.SplineGraphTwoPolynomialNodes, uv [6]geom.Vec2) (pieces [6][]openSectionPiece, err error) {
	for i, edge := range arrangement.Edges {
		h := make([][3]*big.Rat, len(edge.Controls))
		for j, p := range edge.Controls {
			h[j] = [3]*big.Rat{p[0], p[1], big.NewRat(1, 1)}
		}
		curve, e := NewExactBezierUV(h)
		if e != nil {
			return pieces, e
		}
		if curve.ends != ([2]geom.Vec2{uv[edge.Start], uv[edge.End]}) {
			return pieces, geom.ErrConvergence
		}
		if edge.Start == arrangement.Nodes[0].Vertex && edge.End == arrangement.Nodes[1].Vertex {
			// Both lens sides get distinct vertices at the same free station.
			// The native rim becomes a four-edge loop with unique end pairs.
			left, right, e := curve.Split(big.NewRat(1, 2))
			if e != nil {
				return pieces, e
			}
			pieces[i] = []openSectionPiece{
				{a: left.ends[0], b: left.ends[1], c: left, side: -1},
				{a: right.ends[0], b: right.ends[1], c: right, side: -1},
			}
		} else {
			pieces[i] = []openSectionPiece{{a: curve.ends[0], b: curve.ends[1], c: curve, side: -1}}
		}
	}
	return pieces, nil
}

func bindTwoPolynomialNodeEdges(solid Solid, ids []int, arrangement geom.SplineGraphTwoPolynomialNodes, xyz [6]geom.Vec3, pieces [6][]openSectionPiece) (*TwoPolynomialNodeHistory, error) {
	if len(ids) != 8 {
		return nil, geom.ErrConvergence
	}
	const missing = -1
	mapping := &TwoPolynomialNodeHistory{Arrangement: arrangement, Vertices: [6]int{missing, missing, missing, missing, missing, missing}}
	type directedEdge struct {
		id, start, end int
	}
	lookup := make(map[[2]geom.Vec2]directedEdge, 2*len(ids))
	for _, id := range ids {
		edge := solid.edges[id]
		model, ok := edge.Curve.(surfaceTrimCurve)
		if !ok {
			return nil, geom.ErrConvergence
		}
		if _, duplicate := lookup[[2]geom.Vec2{model.a, model.b}]; duplicate {
			return nil, geom.ErrConvergence
		}
		lookup[[2]geom.Vec2{model.a, model.b}] = directedEdge{id, edge.Start, edge.End}
		lookup[[2]geom.Vec2{model.b, model.a}] = directedEdge{id, edge.End, edge.Start}
	}
	for i, chain := range pieces {
		source := arrangement.Edges[i]
		for j, p := range chain {
			edge, ok := lookup[[2]geom.Vec2{p.a, p.b}]
			if !ok {
				return nil, geom.ErrConvergence
			}
			delete(lookup, [2]geom.Vec2{p.a, p.b})
			delete(lookup, [2]geom.Vec2{p.b, p.a})
			mapping.Edges[i] = append(mapping.Edges[i], edge.id)
			if j == 0 && len(chain) == 2 {
				mapping.Midpoints[source.Branch] = edge.end
			}
			if j == 1 && mapping.Midpoints[source.Branch] != edge.start {
				return nil, geom.ErrConvergence
			}
			for _, bind := range [2]struct {
				attached       bool
				source, native int
			}{
				{j == 0, source.Start, edge.start}, {j == len(chain)-1, source.End, edge.end},
			} {
				if !bind.attached {
					continue
				}
				old := mapping.Vertices[bind.source]
				if (old != missing && old != bind.native) || solid.vertices[bind.native] != xyz[bind.source] {
					return nil, geom.ErrConvergence
				}
				mapping.Vertices[bind.source] = bind.native
			}
		}
	}
	if len(lookup) != 0 || mapping.Midpoints[0] == mapping.Midpoints[1] {
		return nil, geom.ErrConvergence
	}
	return mapping, nil
}
