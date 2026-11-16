package trimmed

import (
	"context"
	"fmt"
	"slices"

	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// ParabolaCircleCutComponentHistory identifies one strict positive region
// and its original roof and wall boundaries. Region is 0 below the
// parabola, 1 above it, or 2 inside the circle. Indices are component-local.
type ParabolaCircleCutComponentHistory struct {
	Region       int
	RoofFaces    [2]int // original minuend and subtrahend
	ParabolaEdge int    // -1 for the interior lens
	CircleEdges  []int
	WallFaces    []int
	DomainSides  []int
}

// ParabolaCircleGraphCut retains each positive source layer as its own
// individually embedded native solid.
type ParabolaCircleGraphCut struct {
	sources    [3]*geom.BSplineSurface
	axis       int
	tolerance  float64
	components []Solid
	history    []ParabolaCircleCutComponentHistory
}

// Components returns the closed positive components in source region order.
func (r *ParabolaCircleGraphCut) Components() []Solid {
	if r == nil {
		return nil
	}
	return slices.Clone(r.components)
}

// History returns the original edge and wall owners in component order.
func (r *ParabolaCircleGraphCut) History() []ParabolaCircleCutComponentHistory {
	if r == nil {
		return nil
	}
	out := slices.Clone(r.history)
	for i := range out {
		out[i].CircleEdges = slices.Clone(out[i].CircleEdges)
		out[i].WallFaces = slices.Clone(out[i].WallFaces)
		out[i].DomainSides = slices.Clone(out[i].DomainSides)
	}
	return out
}

// CutParabolaCircleSplineGraphs subtracts B from A on the original shared
// positive stock with a complete open parabola and disjoint interior circle.
// The positive layers are one connected component with a through hole or
// two disjoint components consisting of a wall cap and an interior lens.
func CutParabolaCircleSplineGraphs(ctx context.Context, base, a, b *geom.BSplineSurface, freeAxis int,
	tolerance float64, o ParabolaCircleGraphOptions) (*ParabolaCircleGraphCut, error) {
	if ctx == nil || base == nil || a == nil || b == nil || freeAxis < 0 || freeAxis > 1 ||
		tolerance <= 0 || !finite(tolerance) || o.Stock.Domain != (geom.ParameterRectangle{}) {
		return nil, geom.ErrInvalid
	}
	patches, err := prepareSharedBaseGraphContact(ctx, base, a, b, o.Stock)
	if err != nil {
		return nil, err
	}
	section, err := geom.DiscoverSplineGraphParabolaCircle(ctx, patches[0], patches[1], freeAxis, o.Proof)
	if err != nil {
		return nil, err
	}
	regions, err := parabolaCircleGraphRegions(section)
	if err != nil {
		return nil, err
	}
	walls := [2]*geom.BSplineSurface{a, b}
	evaluate := exactGraphVertexEvaluator(ctx, o.Proof)
	result := &ParabolaCircleGraphCut{sources: [3]*geom.BSplineSurface{base, a, b}, axis: freeAxis, tolerance: tolerance}
	for i, region := range regions {
		if region.sign < 0 {
			continue
		}
		part, assembly, err := assembleGraphRegionDifference(a, b, region, tolerance, evaluate)
		if err != nil {
			return nil, fmt.Errorf("trimmed: parabola-circle Cut region %d: %w", i, err)
		}
		if err := validateExactGraphPartitionEdges(ctx, part, o.Proof); err != nil {
			return nil, err
		}
		h, err := bindParabolaCircleCutComponent(ctx, part, assembly, walls, i, len(region.holes) == 1)
		if err != nil {
			return nil, err
		}
		result.components = append(result.components, part)
		result.history = append(result.history, h)
	}
	// The proved strict gap in the free/dependent chart separates a
	// retained disk from the opposite wall cap through their closures.
	// If the disk is negative, the positive wall layer surrounds its hole.
	wantComponents := 1
	if section.RegionSides[2] > 0 {
		wantComponents = 2
	}
	if len(result.components) != wantComponents {
		return nil, geom.ErrConvergence
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func bindParabolaCircleCutComponent(ctx context.Context, part Solid, base OpenSectionHistory,
	roofs [2]*geom.BSplineSurface, region int, hole bool) (ParabolaCircleCutComponentHistory, error) {
	var h ParabolaCircleCutComponentHistory
	topo := part.Topology()
	// Vertices, edges, faces, inner wires, Euler characteristic.
	want, sectionEdges, wallCount := [5]int{6, 9, 5, 0, 2}, 1, 3
	if region == 2 {
		want, sectionEdges, wallCount = [5]int{4, 4, 2, 0, 2}, 4, 0
	} else if hole {
		want, sectionEdges = [5]int{10, 13, 5, 2, 0}, 5
	}
	actual := [5]int{topo.Vertices, topo.Edges, topo.Faces, topo.InnerWires, topo.EulerCharacteristic}
	if len(topo.Shells) != 1 || topo.CollapsedEdges != 0 || actual != want ||
		len(base.ContourEdges) != sectionEdges || len(base.WallFaces) != wallCount || len(base.DomainSides) != wallCount {
		return h, fmt.Errorf("trimmed: parabola-circle Cut region %d topology %v: %w", region, actual, geom.ErrConvergence)
	}
	h = ParabolaCircleCutComponentHistory{Region: region, RoofFaces: [2]int{base.SplineFace, base.PlaneFace},
		ParabolaEdge: -1, WallFaces: base.WallFaces, DomainSides: base.DomainSides}
	circle := base.ContourEdges
	if region != 2 {
		h.ParabolaEdge = circle[0]
		circle = circle[1:]
	}
	h.CircleEdges = circle
	check := func(loop []Use, edge int) bool {
		return slices.ContainsFunc(loop, func(u Use) bool { return u.Edge == edge && !u.Reversed })
	}
	for owner, id := range h.RoofFaces {
		f := part.faces[id]
		if f.Surface != roofs[owner] || f.Reversed != (owner == 1) || len(f.Holes) != topo.InnerWires/2 {
			return h, geom.ErrConvergence
		}
		if h.ParabolaEdge >= 0 && !check(f.Outer, h.ParabolaEdge) {
			return h, geom.ErrConvergence
		}
		ring := f.Outer
		if hole {
			ring = f.Holes[0]
		}
		for _, edge := range h.CircleEdges {
			if !check(ring, edge) {
				return h, geom.ErrConvergence
			}
		}
		t, err := ValidateFaceTrimLoops(ctx, f, trimvalidate.Options{})
		if err != nil {
			return h, err
		}
		if t.Status != trimvalidate.Certified {
			return h, geom.ErrConvergence
		}
	}
	for _, face := range h.WallFaces {
		var seen [2]bool
		for _, u := range part.faces[face].Outer {
			for owner, roof := range roofs {
				trim, ok := graphWallSourceTrim(u.Curve, owner == 0, false)
				if ok && trim.surface == roof {
					seen[owner] = true
				}
			}
		}
		if seen != ([2]bool{true, true}) {
			return h, geom.ErrConvergence
		}
	}
	return h, nil
}
