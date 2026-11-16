package trimmed

import (
	"context"
	"fmt"
	"slices"

	"plainsolid/geom"
)

// ParabolaThreeCrossingCirclesCutHistory records the original roofs, walls,
// and directed contour uses of one positive component. The parabola and
// circle edge ledgers follow the envelope order and use -1 for absent edges.
type ParabolaThreeCrossingCirclesCutHistory struct {
	Region        int
	RoofFaces     [2]int
	ParabolaEdges [threeCrossingCirclesParabolaPieces]int
	CircleEdges   [3][]int
	WallFaces     []int
	DomainSides   []int
}

// ParabolaThreeCrossingCirclesCut holds four embedded positive native shells.
// The exterior component meets each of the three caps at exactly its two
// original junctions. Every contact refers to separate component vertices.
type ParabolaThreeCrossingCirclesCut struct {
	components [4]Solid
	history    [4]ParabolaThreeCrossingCirclesCutHistory
	contacts   [3][2]ParabolaTwoCrossingCirclesCutContact
	sources    [3]*geom.BSplineSurface
	axis       int
	tolerance  float64
}

func (r *ParabolaThreeCrossingCirclesCut) Components() [4]Solid {
	if r == nil {
		return [4]Solid{}
	}
	return r.components
}

func (r *ParabolaThreeCrossingCirclesCut) History() (h [4]ParabolaThreeCrossingCirclesCutHistory) {
	if r == nil {
		return h
	}
	h = r.history
	for i := range h {
		h[i].WallFaces = slices.Clone(h[i].WallFaces)
		h[i].DomainSides = slices.Clone(h[i].DomainSides)
		for circle, edges := range h[i].CircleEdges {
			h[i].CircleEdges[circle] = slices.Clone(edges)
		}
	}
	return h
}

func (r *ParabolaThreeCrossingCirclesCut) Contacts() (contacts [3][2]ParabolaTwoCrossingCirclesCutContact) {
	if r == nil {
		return contacts
	}
	contacts = r.contacts
	cloneCrossingCircleCutContacts(contacts[:])
	return contacts
}

// CutParabolaThreeCrossingCirclesSplineGraphs returns the original ordered
// difference as separate components. Each positive cap is bounded by its
// original parabola and circle pieces. The exterior touches that cap only
// at the endpoints where the material height vanishes.
func CutParabolaThreeCrossingCirclesSplineGraphs(ctx context.Context, base, a, b *geom.BSplineSurface, axis int,
	tolerance float64, o ParabolaCircleGraphOptions) (*ParabolaThreeCrossingCirclesCut, error) {
	src := [3]*geom.BSplineSurface{base, a, b}
	part, proof, err := proveParabolaThreeCrossingCirclesPartition(ctx, src, axis, tolerance, o)
	if err != nil {
		return nil, err
	}
	exterior := 0
	if proof.RegionSides[1] > 0 {
		exterior = 1
	}
	regions := [4]int{exterior, 3 - exterior, 5 - exterior, 7 - exterior}
	out := &ParabolaThreeCrossingCirclesCut{sources: src, axis: axis, tolerance: tolerance}
	var assembly [4]OpenSectionHistory
	var edges [4][]int
	evaluate := exactGraphVertexEvaluator(ctx, o.Proof)
	for i, region := range regions {
		reg := part.regions[region]
		if reg.sign <= 0 {
			return nil, geom.ErrConvergence
		}
		s, history, err := assembleOpenSplineGraphPairOnCharts(ctx, a, b, 0, true, tolerance, reg.loop,
			[2]graphEnvelopeChart{}, nil, evaluate)
		if err != nil {
			return nil, fmt.Errorf("trimmed: three crossing circles Cut region %d: %w", region, err)
		}
		mapped, err := bindCrossingCircleCutComponent(ctx, s, history, part.pieces, region, reg, proof.RegionWitnesses[region],
			[2]*geom.BSplineSurface{a, b}, o.Proof)
		if err != nil {
			return nil, fmt.Errorf("trimmed: three crossing circles Cut region %d source binding: %w", region, err)
		}
		h := ParabolaThreeCrossingCirclesCutHistory{Region: region, RoofFaces: [2]int{history.SplineFace, history.PlaneFace},
			ParabolaEdges: [threeCrossingCirclesParabolaPieces]int(mapped[:threeCrossingCirclesParabolaPieces]),
			WallFaces:     history.WallFaces, DomainSides: history.DomainSides}
		bindCrossingCircleCutRings(h.CircleEdges[:], part.circles, mapped)
		out.components[i], out.history[i], assembly[i], edges[i] = s, h, history, mapped
	}
	if err := bindCrossingCircleCutContacts(proof.Nodes[:], part.rays, out.components[:], regions[:], assembly[:], edges[:], out.contacts[:]); err != nil {
		return nil, fmt.Errorf("trimmed: three crossing circles Cut contacts: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
