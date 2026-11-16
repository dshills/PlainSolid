package trimmed

import (
	"context"
	"fmt"
	"slices"

	"plainsolid/geom"
)

// CuspidalCubicCutOptions bounds the stock, chart and section proofs.
type CuspidalCubicCutOptions = CuspidalCubicCommonOptions

// CuspidalCubicCutHistory binds the retained source region to its original
// roof faces, exact cusp germs and stock walls.
type CuspidalCubicCutHistory struct {
	Region    int    // 0 exterior, 1 between the germs
	RoofFaces [2]int // minuend, subtrahend
	Vertices  [3]int // cusp, negative exit, positive exit
	Germs     [2]int // negative t, positive t
	WallFaces []int
}

// CuspidalCubicCut retains the positive material between the original roofs
// on the single source region where the first roof is higher.
type CuspidalCubicCut struct {
	solid     Solid
	history   CuspidalCubicCutHistory
	section   *geom.CuspidalCubicSection
	sources   [3]*geom.BSplineSurface
	freeAxis  int
	tolerance float64
}

// Boundary returns the exact native shell. The generic tessellator can reject
// a display of the exterior region when its sampled boundary fails closure.
func (r *CuspidalCubicCut) Boundary() Solid {
	if r == nil {
		return Solid{}
	}
	return r.solid
}

// History returns the native incidence of the original cusp and wall faces.
func (r *CuspidalCubicCut) History() CuspidalCubicCutHistory {
	if r == nil {
		return CuspidalCubicCutHistory{}
	}
	h := r.history
	h.WallFaces = slices.Clone(h.WallFaces)
	return h
}

// CutCuspidalCubicSplineGraphs subtracts the second stock from the first over
// a shared positive rectangular base. The certified cusp opens in positive
// model X, with its two exit vertices on the far side of the stock. The
// positive region is a disk, and the layer collapses only along its one
// continuous cusp arc. Both roof charts and their singular jets are retained.
func CutCuspidalCubicSplineGraphs(ctx context.Context, base, a, c *geom.BSplineSurface, freeAxis int,
	tolerance float64, o CuspidalCubicCutOptions) (*CuspidalCubicCut, error) {
	prepared, err := prepareCuspidalCubicStock(ctx, base, a, c, freeAxis, tolerance, o)
	if err != nil {
		return nil, err
	}
	section := prepared.section
	region := 0
	if section.Regions[0].HeightSide < 0 {
		region = 1
	}
	if section.Regions[region].HeightSide != 1 || section.Regions[1-region].HeightSide != -1 {
		return nil, geom.ErrConvergence
	}
	loop := prepared.regions[region].loop
	if region == 1 {
		// This disk has one stock-side interval, with zero gap at both
		// exits. An exact midpoint supplies its positive upright.
		if len(loop) != 3 || loop[1].side < 0 {
			return nil, geom.ErrConvergence
		}
		sides, err := splitGraphSideAtMidpoint(loop[1])
		if err != nil {
			return nil, err
		}
		loop = []openSectionPiece{loop[0], sides[0], sides[1], loop[2]}
	}

	var expected [2]map[[2]geom.Vec2]openSectionPiece
	for owner, chart := range prepared.charts {
		expected[owner] = make(map[[2]geom.Vec2]openSectionPiece, 2)
		for _, piece := range loop {
			if piece.side >= 0 {
				continue
			}
			owned, err := chart.piece(piece)
			if err != nil {
				return nil, err
			}
			expected[owner][[2]geom.Vec2{piece.a, piece.b}] = owned
		}
	}
	solid, layer, err := assembleOpenSplineGraphPairOnCharts(ctx, a, c, 0, true, tolerance, loop, prepared.charts,
		nodalCubicReferenceXY, exactGraphVertexEvaluator(ctx, o.Proof))
	if err != nil {
		return nil, err
	}
	roofs := [2]*geom.BSplineSurface{a, c}
	if err := bindAlignedPolynomialCutLayer(ctx, solid, layer, roofs, prepared.charts, loop, expected, o.Proof); err != nil {
		return nil, err
	}
	seam, err := bindCuspidalCubicCutSeam(solid, layer, section, prepared.points)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	history := CuspidalCubicCutHistory{Region: region, RoofFaces: seam.RoofFaces, Vertices: seam.Vertices,
		Germs: seam.Germs, WallFaces: slices.Clone(layer.WallFaces)}
	return &CuspidalCubicCut{solid: solid, history: history, section: section,
		sources: [3]*geom.BSplineSurface{base, a, c}, freeAxis: freeAxis, tolerance: tolerance}, nil
}

func bindCuspidalCubicCutSeam(s Solid, layer OpenSectionHistory, section *geom.CuspidalCubicSection,
	points [3]geom.Vec3) (CuspidalCubicCommonHistory, error) {
	var h CuspidalCubicCommonHistory
	cusp, err := polynomialCutContactVertex(s, layer, points[0])
	if err != nil {
		return h, err
	}
	h.RoofFaces = [2]int{layer.SplineFace, layer.PlaneFace}
	h.RoofOwners = [2]int{0, 1}
	h.Vertices = [3]int{cusp, -1, -1}
	h.Germs = [2]int{-1, -1}
	for _, id := range layer.ContourEdges {
		edge := s.edges[id]
		station, exit := edge.Range.Min, edge.End
		if edge.End == cusp {
			station, exit = edge.Range.Max, edge.Start
		} else if edge.Start != cusp {
			return h, geom.ErrConvergence
		}
		derivative, err := edge.Curve.Derivative(station)
		if err != nil {
			return h, err
		}
		if derivative != (geom.Vec3{}) {
			return h, fmt.Errorf("trimmed: original Cut cusp germ changed its model derivative: %w", geom.ErrConvergence)
		}
		germ := 0
		if s.vertices[exit] == points[2] {
			germ = 1
		}
		if s.vertices[exit] != points[germ+1] || h.Germs[germ] >= 0 {
			return h, geom.ErrConvergence
		}
		h.Germs[germ] = id
		h.Vertices[germ+1] = exit
	}
	for germ, id := range h.Germs {
		if id < 0 {
			return h, geom.ErrConvergence
		}
		if err := bindCuspidalCubicGermCoedges(s, h, section, germ); err != nil {
			return h, err
		}
	}
	return h, nil
}
