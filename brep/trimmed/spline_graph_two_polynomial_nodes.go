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
	loops, e := twoPolynomialNodeRegionLoops(arrangement, uv, pieces)
	if e != nil {
		return fail(e)
	}
	solid, history, e := assemblePolynomialNodeEnvelope(ctx, base, a, b, op, tolerance, arrangement.Regions[:], loops[:], proof)
	if e != nil {
		return fail(e)
	}
	mapping, e := bindTwoPolynomialNodeEdges(solid, history.ContourEdges, arrangement, xyz, pieces)
	if e != nil {
		return fail(e)
	}
	history.TwoPolynomialNodes = mapping
	return solid, history, nil
}

func assemblePolynomialNodeEnvelope(ctx context.Context, base, a, b *geom.BSplineSurface, op parametric.BooleanOp, tolerance float64, arrangement []geom.SplineGraphPolynomialRegion, loops [][]openSectionPiece, proof geom.GraphPointContactOptions) (Solid, SplineGraphBooleanHistory, error) {
	fail := func(e error) (Solid, SplineGraphBooleanHistory, error) {
		return Solid{}, SplineGraphBooleanHistory{TopA: -1, TopB: -1, Base: -1}, e
	}
	regions := make([]graphEnvelopeRegion, len(loops))
	for i, loop := range loops {
		regions[i] = graphEnvelopeRegion{loop: loop, first: (arrangement[i].HeightSide > 0) == (op == parametric.Union)}
	}
	solid, history, e := assembleSplineEnvelope(ctx, base, a, b, regions, tolerance, exactGraphVertexEvaluator(ctx, proof))
	if e != nil {
		return fail(e)
	}
	if e = validateExactGraphPartitionEdges(ctx, solid, proof); e != nil {
		return fail(e)
	}
	for i, region := range arrangement {
		location, r, e := LocateRationalInFaceTrim(ctx, solid.faces[i], region.Witness, trimvalidate.Options{})
		if e != nil {
			return fail(e)
		}
		if r.Status != trimvalidate.Certified || location != trimvalidate.Inside {
			return fail(fmt.Errorf("trimmed: polynomial node region %d: %s: %w", i, r.Reason, geom.ErrConvergence))
		}
	}
	if e = affineMaterialVertexLinks(ctx, solid); e != nil {
		return fail(e)
	}
	return solid, history, nil
}

func twoPolynomialNodeRegionLoops(arrangement geom.SplineGraphTwoPolynomialNodes, uv [6]geom.Vec2, pieces [6][]openSectionPiece) ([5][]openSectionPiece, error) {
	loops, e := polynomialNodeRegionLoops(arrangement.Regions[:], uv[:], pieces[:])
	if e != nil {
		return [5][]openSectionPiece{}, e
	}
	return [5][]openSectionPiece(loops), nil
}

func polynomialNodeRegionLoops(regions []geom.SplineGraphPolynomialRegion, uv []geom.Vec2, pieces [][]openSectionPiece) ([][]openSectionPiece, error) {
	return polynomialNodeRegionLoopsOnRectangle(regions, uv, pieces, geom.ParameterRectangle{UMax: 1, VMax: 1})
}

func polynomialNodeRegionLoopsOnRectangle(regions []geom.SplineGraphPolynomialRegion, uv []geom.Vec2, pieces [][]openSectionPiece,
	d geom.ParameterRectangle) ([][]openSectionPiece, error) {
	loops := make([][]openSectionPiece, len(regions))
	for i, region := range regions {
		for _, use := range region.Edges {
			chain := pieces[use.Edge]
			if use.Reversed {
				for j := len(chain) - 1; j >= 0; j-- {
					p := chain[j]
					loops[i] = append(loops[i], openSectionPiece{a: p.b, b: p.a, c: reversePCurve(p.c), side: -1})
				}
			} else {
				loops[i] = append(loops[i], chain...)
			}
		}
		if p := region.Perimeter; p != nil {
			if p.ParameterDirection != 1 {
				return loops, geom.ErrConvergence
			}
			arc, e := saddleRectangleArc(uv[p.Start], uv[p.End], d)
			if e != nil {
				return loops, e
			}
			loops[i] = append(loops[i], arc...)
		}
	}
	return loops, nil
}

func twoPolynomialNodePieces(arrangement geom.SplineGraphTwoPolynomialNodes, uv [6]geom.Vec2) ([6][]openSectionPiece, error) {
	pieces, e := polynomialNodePieces(arrangement.Edges[:], uv[:])
	if e != nil {
		return [6][]openSectionPiece{}, e
	}
	return [6][]openSectionPiece(pieces), nil
}

// polynomialNodePieces keeps each already-proved graph restriction at its original degree.
func polynomialNodePieces(edges []geom.SplineGraphPolynomialPiece, uv []geom.Vec2) ([][]openSectionPiece, error) {
	pieces := make([][]openSectionPiece, len(edges))
	for i, edge := range edges {
		h := make([][3]*big.Rat, len(edge.Controls))
		for j, p := range edge.Controls {
			h[j] = [3]*big.Rat{p[0], p[1], big.NewRat(1, 1)}
		}
		curve, e := newExactBezierUV(h, maxSourceGraphBezierControls)
		if e != nil {
			return pieces, e
		}
		if curve.ends != ([2]geom.Vec2{uv[edge.Start], uv[edge.End]}) {
			return pieces, geom.ErrConvergence
		}
		// The four terminal vertices precede all interior nodes.
		const firstNode = 4
		if edge.Start >= firstNode && edge.End >= firstNode {
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
	binding, e := bindPolynomialNodeEdges(solid, ids, arrangement.Edges[:], xyz[:], pieces[:])
	if e != nil {
		return nil, e
	}
	return &TwoPolynomialNodeHistory{
		Arrangement: arrangement,
		Edges:       [6][]int(binding.edges),
		Vertices:    [6]int(binding.vertices),
		Midpoints:   [2]int{binding.midpoints[1], binding.midpoints[4]},
	}, nil
}

type polynomialNodeBinding struct {
	edges     [][]int
	vertices  []int
	midpoints []int // indexed by source edge, -1 for an unsplit edge
}

func bindPolynomialNodeEdges(solid Solid, ids []int, sources []geom.SplineGraphPolynomialPiece, xyz []geom.Vec3, pieces [][]openSectionPiece) (polynomialNodeBinding, error) {
	return bindPolynomialNodeEdgesOnChart(solid, ids, sources, xyz, pieces, false)
}

func bindPolynomialNodeEdgesOnChart(solid Solid, ids []int, sources []geom.SplineGraphPolynomialPiece, xyz []geom.Vec3,
	pieces [][]openSectionPiece, modelXY bool) (polynomialNodeBinding, error) {
	const missing = -1
	mapping := polynomialNodeBinding{
		edges:     make([][]int, len(sources)),
		vertices:  make([]int, len(xyz)),
		midpoints: make([]int, len(sources)),
	}
	for i := range mapping.vertices {
		mapping.vertices[i] = missing
	}
	for i := range mapping.midpoints {
		mapping.midpoints[i] = missing
	}
	type directedEdge struct {
		id, start, end int
	}
	lookup := make(map[[2]geom.Vec2]directedEdge, 2*len(ids))
	for _, id := range ids {
		edge := solid.edges[id]
		model, ok := edge.Curve.(surfaceTrimCurve)
		if !ok {
			return polynomialNodeBinding{}, geom.ErrConvergence
		}
		a, b := model.a, model.b
		if modelXY {
			start, end := solid.vertices[edge.Start], solid.vertices[edge.End]
			a, b = geom.Vec2{X: start.X, Y: start.Y}, geom.Vec2{X: end.X, Y: end.Y}
		}
		if _, duplicate := lookup[[2]geom.Vec2{a, b}]; duplicate {
			return polynomialNodeBinding{}, geom.ErrConvergence
		}
		lookup[[2]geom.Vec2{a, b}] = directedEdge{id, edge.Start, edge.End}
		lookup[[2]geom.Vec2{b, a}] = directedEdge{id, edge.End, edge.Start}
	}
	for i, chain := range pieces {
		source := sources[i]
		for j, p := range chain {
			edge, ok := lookup[[2]geom.Vec2{p.a, p.b}]
			if !ok {
				return polynomialNodeBinding{}, geom.ErrConvergence
			}
			delete(lookup, [2]geom.Vec2{p.a, p.b})
			delete(lookup, [2]geom.Vec2{p.b, p.a})
			mapping.edges[i] = append(mapping.edges[i], edge.id)
			if j == 0 && len(chain) == 2 {
				mapping.midpoints[i] = edge.end
			}
			if j == 1 && mapping.midpoints[i] != edge.start {
				return polynomialNodeBinding{}, geom.ErrConvergence
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
				old := mapping.vertices[bind.source]
				if (old != missing && old != bind.native) || solid.vertices[bind.native] != xyz[bind.source] {
					return polynomialNodeBinding{}, geom.ErrConvergence
				}
				mapping.vertices[bind.source] = bind.native
			}
		}
	}
	if len(lookup) != 0 {
		return polynomialNodeBinding{}, geom.ErrConvergence
	}
	used := make(map[int]bool, len(xyz)+len(sources))
	for _, id := range mapping.vertices {
		if id == missing || used[id] {
			return polynomialNodeBinding{}, geom.ErrConvergence
		}
		used[id] = true
	}
	for _, id := range mapping.midpoints {
		if id == missing {
			continue
		}
		if used[id] {
			return polynomialNodeBinding{}, geom.ErrConvergence
		}
		used[id] = true
	}
	return mapping, nil
}
