package trimmed

import (
	"context"
	"fmt"
	"slices"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// ParabolaThreeCirclesCutHistory records the original region, roofs, walls,
// and curve edges of one positive layer. The separated ring belongs to
// either an exterior hole or an independent disk. Absent ring IDs are -1.
type ParabolaThreeCirclesCutHistory struct {
	ParabolaTwoCrossingCirclesCutHistory
	SeparatedEdges    [4]int
	SeparatedVertices [4]int
}

// ParabolaThreeCirclesCutArrangement maps the two contact groups and the
// isolated rim to the original free/dependent center order. SeparatedRegion
// is the exterior region enclosing the separated circle.
type ParabolaThreeCirclesCutArrangement struct {
	CrossingCircles [2]int
	SeparatedCircle int
	SeparatedRegion int
}

// ParabolaThreeCirclesCut holds the exterior, both crossing caps and,
// when positive, the independent disk. Each is a separate native shell.
type ParabolaThreeCirclesCut struct {
	components  []Solid
	history     []ParabolaThreeCirclesCutHistory
	contacts    [2][2]ParabolaTwoCrossingCirclesCutContact
	arrangement ParabolaThreeCirclesCutArrangement
	sources     [3]*geom.BSplineSurface
	axis        int
	tolerance   float64
}

func (r *ParabolaThreeCirclesCut) Components() []Solid {
	if r == nil {
		return nil
	}
	return slices.Clone(r.components)
}

func (r *ParabolaThreeCirclesCut) History() []ParabolaThreeCirclesCutHistory {
	if r == nil {
		return nil
	}
	history := slices.Clone(r.history)
	for i := range history {
		h := &history[i].ParabolaTwoCrossingCirclesCutHistory
		*h = cloneTwoCrossingCircleCutHistory(*h)
	}
	return history
}

func (r *ParabolaThreeCirclesCut) Contacts() [2][2]ParabolaTwoCrossingCirclesCutContact {
	if r == nil {
		return [2][2]ParabolaTwoCrossingCirclesCutContact{}
	}
	return cloneTwoCrossingCircleCutContacts(r.contacts)
}

func (r *ParabolaThreeCirclesCut) Arrangement() ParabolaThreeCirclesCutArrangement {
	if r == nil {
		return ParabolaThreeCirclesCutArrangement{}
	}
	return r.arrangement
}

// CutParabolaThreeCirclesSplineGraphs forms A minus B on the original
// stock. Only the positive source regions materialize. The four transverse
// nodes give the only separate component contacts; the separated rim is
// either a hole of one positive layer or bounds an independent layer.
func CutParabolaThreeCirclesSplineGraphs(ctx context.Context, base, a, b *geom.BSplineSurface, axis int,
	tolerance float64, o ParabolaCircleGraphOptions) (*ParabolaThreeCirclesCut, error) {
	sources := [3]*geom.BSplineSurface{base, a, b}
	p, err := proveParabolaThreeCirclesPartition(ctx, sources, axis, tolerance, o)
	if err != nil {
		return nil, fmt.Errorf("trimmed: three-circle Cut partition: %w", err)
	}
	out := &ParabolaThreeCirclesCut{sources: sources, axis: axis, tolerance: tolerance,
		arrangement: ParabolaThreeCirclesCutArrangement{CrossingCircles: p.proof.CrossingCircles,
			SeparatedCircle: p.proof.SeparatedCircle, SeparatedRegion: p.proof.SeparatedRegion}}
	exterior := 0
	if p.proof.RegionSides[1] > 0 {
		exterior = 1
	}
	crossingRegions := [3]int{exterior, 3 - exterior, 5 - exterior}
	regions := slices.Clone(crossingRegions[:])
	if p.proof.RegionSides[6] > 0 {
		regions = append(regions, 6)
	}
	if (len(regions) == 3) != (len(p.regions[exterior].holes) == 1) {
		return nil, geom.ErrConvergence
	}
	evaluate := exactGraphVertexEvaluator(ctx, o.Proof)
	buildHole := func(vertices []geom.Vec3, edges []parametric.Edge, faces []Face, tolerance float64) (Solid, error) {
		return newMixedCirclesHoledCutSolid(ctx, vertices, edges, faces, tolerance)
	}
	var walls [3]OpenSectionHistory
	var edges [3][]int
	roofs := [2]*geom.BSplineSurface{a, b}
	for i, region := range regions {
		if p.regions[region].sign <= 0 {
			return nil, geom.ErrConvergence
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		part, assembly, err := assembleGraphRegionDifferenceWithHoleBuilder(a, b, p.regions[region], tolerance, buildHole, evaluate)
		if err != nil {
			return nil, fmt.Errorf("trimmed: three-circle Cut region %d: %w", region, err)
		}
		h, contour, err := bindParabolaThreeCirclesCutComponent(ctx, part, assembly, p, region, roofs, o.Proof)
		if err != nil {
			return nil, fmt.Errorf("trimmed: three-circle Cut region %d source binding: %w", region, err)
		}
		if i < len(walls) {
			walls[i], edges[i] = assembly, contour
		}
		out.components, out.history = append(out.components, part), append(out.history, h)
	}
	out.contacts, err = bindTwoCrossingCircleCutContacts(p.crossing, out.components, crossingRegions, walls, edges)
	if err != nil {
		return nil, fmt.Errorf("trimmed: three-circle Cut contacts: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func bindParabolaThreeCirclesCutComponent(ctx context.Context, part Solid, assembly OpenSectionHistory,
	p parabolaThreeCirclesPartition, region int, roofs [2]*geom.BSplineSurface,
	opts geom.GraphPointContactOptions) (h ParabolaThreeCirclesCutHistory, mapped []int, err error) {
	h.SeparatedEdges = [4]int{-1, -1, -1, -1}
	h.SeparatedVertices = [4]int{-1, -1, -1, -1}
	if region == 6 {
		h.ParabolaTwoCrossingCirclesCutHistory, err = bindParabolaThreeCirclesCutDisk(ctx, part, assembly, p, roofs, opts)
	} else {
		h.ParabolaTwoCrossingCirclesCutHistory, mapped, err = bindTwoCrossingCirclesCutComponent(ctx, part, assembly, p.crossing, region, roofs, opts)
	}
	if err != nil || (region < 6 && len(p.regions[region].holes) == 0) {
		return h, mapped, err
	}

	disk := p.regions[6].loop
	rim, err := bindParabolaCircleCrossingEdges(part, assembly.ContourEdges, disk)
	if err != nil {
		return h, nil, err
	}
	h.SeparatedEdges = [4]int(rim)
	h.SeparatedVertices, err = bindOriginalSeparatedCircleRing(part, rim, disk, p.proof.SeparatedExtrema)
	if err != nil {
		return h, nil, err
	}
	for _, id := range rim {
		model, ok := part.edges[id].Curve.(surfaceTrimCurve)
		if !ok || model.surface != roofs[0] {
			return h, nil, geom.ErrConvergence
		}
		for _, faceID := range h.RoofFaces {
			face := part.faces[faceID]
			loop := face.Outer
			if region < 6 {
				loop = face.Holes[0]
			}
			if !slices.ContainsFunc(loop, func(use Use) bool {
				return use.Edge == id && !use.Reversed && use.UVStart == model.a && use.UVEnd == model.b &&
					sameParabolaCircleCrossingTrim(use.Curve, model.curve)
			}) {
				return h, nil, geom.ErrConvergence
			}
		}
	}
	return h, mapped, nil
}

func bindParabolaThreeCirclesCutDisk(ctx context.Context, part Solid, assembly OpenSectionHistory,
	p parabolaThreeCirclesPartition, roofs [2]*geom.BSplineSurface,
	opts geom.GraphPointContactOptions) (h ParabolaTwoCrossingCirclesCutHistory, err error) {
	h.Region = 6
	h.RoofFaces = [2]int{assembly.SplineFace, assembly.PlaneFace}
	for i := range h.ParabolaEdges {
		h.ParabolaEdges[i] = -1
	}
	for i, ring := range p.crossing.circles {
		h.CircleEdges[i] = make([]int, len(ring))
		for j := range h.CircleEdges[i] {
			h.CircleEdges[i][j] = -1
		}
	}
	topo := part.Topology()
	if topo.Faces != 2 || topo.Edges != 4 || topo.Vertices != 4 || topo.InnerWires != 0 ||
		len(assembly.WallFaces) != 0 || len(assembly.DomainSides) != 0 || len(assembly.ContourEdges) != 4 {
		return h, geom.ErrConvergence
	}
	for owner, id := range h.RoofFaces {
		if err := bindParabolaCirclesRegionFace(ctx, part.faces[id], roofs[owner], owner == 1,
			p.regions[6], p.proof.RegionWitnesses[6]); err != nil {
			return h, err
		}
	}
	return h, validatePolynomialCutBoundary(ctx, part, assembly, opts)
}
