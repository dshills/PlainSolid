package trimmed

import (
	"context"
	"fmt"
	"math/big"
	"slices"

	"plainsolid/geom"
)

// ParabolaTwoCrossingCirclesCutHistory records both original roof uses of
// a positive component. The parabola and circle edge ledgers use the
// envelope piece order, with absent edges set to -1.
type ParabolaTwoCrossingCirclesCutHistory struct {
	Region        int
	RoofFaces     [2]int
	ParabolaEdges [twoCrossingCirclesParabolaPieces]int
	CircleEdges   [2][]int
	WallFaces     []int
	DomainSides   []int
}

// ParabolaTwoCrossingCirclesCutContact binds one original point to the
// exterior and the appropriate cap component, with each component's two
// outgoing boundary edges in physical counterclockwise sector order.
type ParabolaTwoCrossingCirclesCutContact struct {
	Parameters [2]*big.Rat
	Point      [3]*big.Rat
	Components [2]int
	Vertices   [2]int
	Rays       [2][2]int
}

// ParabolaTwoCrossingCirclesCut holds three individually embedded positive
// shells. The exterior component meets each cap at its two exact junctions.
type ParabolaTwoCrossingCirclesCut struct {
	components [3]Solid
	history    [3]ParabolaTwoCrossingCirclesCutHistory
	contacts   [2][2]ParabolaTwoCrossingCirclesCutContact
	sources    [3]*geom.BSplineSurface
	axis       int
	tolerance  float64
}

func (r *ParabolaTwoCrossingCirclesCut) Components() [3]Solid {
	if r == nil {
		return [3]Solid{}
	}
	return r.components
}

func (r *ParabolaTwoCrossingCirclesCut) History() (h [3]ParabolaTwoCrossingCirclesCutHistory) {
	if r == nil {
		return h
	}
	h = r.history
	for i := range h {
		h[i] = cloneTwoCrossingCircleCutHistory(h[i])
	}
	return h
}

func cloneTwoCrossingCircleCutHistory(h ParabolaTwoCrossingCirclesCutHistory) ParabolaTwoCrossingCirclesCutHistory {
	h.WallFaces = slices.Clone(h.WallFaces)
	h.DomainSides = slices.Clone(h.DomainSides)
	for circle, edges := range h.CircleEdges {
		h.CircleEdges[circle] = slices.Clone(edges)
	}
	return h
}

func (r *ParabolaTwoCrossingCirclesCut) Contacts() [2][2]ParabolaTwoCrossingCirclesCutContact {
	if r == nil {
		return [2][2]ParabolaTwoCrossingCirclesCutContact{}
	}
	return cloneTwoCrossingCircleCutContacts(r.contacts)
}

func cloneTwoCrossingCircleCutContacts(out [2][2]ParabolaTwoCrossingCirclesCutContact) [2][2]ParabolaTwoCrossingCirclesCutContact {
	for i := range out {
		for j := range out[i] {
			for k, q := range out[i][j].Parameters {
				out[i][j].Parameters[k] = new(big.Rat).Set(q)
			}
			for k, q := range out[i][j].Point {
				out[i][j].Point[k] = new(big.Rat).Set(q)
			}
		}
	}
	return out
}

// CutParabolaTwoCrossingCirclesSplineGraphs builds the original ordered
// difference. The positive regions alternate at each crossing. Their chart
// closures meet only at the four source nodes, where the difference layer
// has zero height. Each component remains a distinct closed shell.
func CutParabolaTwoCrossingCirclesSplineGraphs(ctx context.Context, base, a, b *geom.BSplineSurface, axis int,
	tolerance float64, o ParabolaCircleGraphOptions) (*ParabolaTwoCrossingCirclesCut, error) {
	sources := [3]*geom.BSplineSurface{base, a, b}
	p, err := proveParabolaTwoCrossingCirclesPartition(ctx, sources, axis, tolerance, o)
	if err != nil {
		return nil, fmt.Errorf("trimmed: two crossing circles Cut partition: %w", err)
	}
	out := &ParabolaTwoCrossingCirclesCut{sources: sources, axis: axis, tolerance: tolerance}
	exterior := 0
	if p.proof.RegionSides[1] > 0 {
		exterior = 1
	}
	regions := [3]int{exterior, 3 - exterior, 5 - exterior}
	var assembly [3]OpenSectionHistory
	var edges [3][]int
	evaluate := exactGraphVertexEvaluator(ctx, o.Proof)
	for i, region := range regions {
		if p.regions[region].sign <= 0 {
			return nil, geom.ErrConvergence
		}
		solid, h, err := assembleOpenSplineGraphPairOnCharts(ctx, a, b, 0, true, tolerance, p.regions[region].loop,
			[2]graphEnvelopeChart{}, nil, evaluate)
		if err != nil {
			return nil, fmt.Errorf("trimmed: two crossing circles Cut region %d: %w", region, err)
		}
		bound, contour, err := bindTwoCrossingCirclesCutComponent(ctx, solid, h, p, region, [2]*geom.BSplineSurface{a, b}, o.Proof)
		if err != nil {
			return nil, fmt.Errorf("trimmed: two crossing circles Cut region %d source binding: %w", region, err)
		}
		out.components[i], out.history[i], assembly[i], edges[i] = solid, bound, h, contour
	}
	out.contacts, err = bindTwoCrossingCircleCutContacts(p, out.components[:], regions, assembly, edges)
	if err != nil {
		return nil, fmt.Errorf("trimmed: two crossing circles Cut contacts: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func bindTwoCrossingCirclesCutComponent(ctx context.Context, part Solid, assembly OpenSectionHistory,
	p parabolaTwoCrossingCirclesPartition, region int, roofs [2]*geom.BSplineSurface,
	opts geom.GraphPointContactOptions) (h ParabolaTwoCrossingCirclesCutHistory, mapped []int, err error) {
	h.Region = region
	h.RoofFaces = [2]int{assembly.SplineFace, assembly.PlaneFace}
	h.WallFaces, h.DomainSides = assembly.WallFaces, assembly.DomainSides
	reg := p.regions[region]
	wallCount, additionalVertices, additionalEdges := 0, 0, 0
	if region < 2 {
		wallCount, additionalVertices, additionalEdges = 3, 5, 8
	}
	count := len(reg.loop) - wallCount
	holes := len(reg.holes)
	if holes > 1 || (holes == 1 && (region >= 2 || len(reg.holes[0]) != 4)) {
		return h, nil, geom.ErrConvergence
	}
	topo := part.Topology()
	if len(h.WallFaces) != wallCount || len(h.DomainSides) != wallCount || len(assembly.ContourEdges) != count+4*holes ||
		topo.InnerWires != 2*holes || topo.Faces != 2+wallCount ||
		topo.Vertices != count+additionalVertices+4*holes || topo.Edges != count+additionalEdges+4*holes {
		return h, nil, geom.ErrConvergence
	}
	for owner, id := range h.RoofFaces {
		if err := bindParabolaCirclesRegionFace(ctx, part.faces[id], roofs[owner], owner == 1, reg, p.proof.RegionWitnesses[region]); err != nil {
			return h, nil, err
		}
	}
	var holeEdges []int
	if holes == 1 {
		holeEdges, err = bindParabolaCircleCrossingEdges(part, assembly.ContourEdges, reg.holes[0])
		if err != nil {
			return h, nil, err
		}
	}
	mapped = make([]int, len(p.pieces))
	for i := range mapped {
		mapped[i] = -1
	}
	for _, id := range assembly.ContourEdges {
		if slices.Contains(holeEdges, id) {
			continue
		}
		model, ok := part.edges[id].Curve.(surfaceTrimCurve)
		if !ok || model.surface != roofs[0] {
			return h, nil, geom.ErrConvergence
		}
		source := slices.IndexFunc(p.pieces, func(piece openSectionPiece) bool {
			reversed := reverseParabolaCircleCrossingPiece(piece)
			return (piece.a == model.a && piece.b == model.b && sameParabolaCircleCrossingTrim(model.curve, piece.c)) ||
				(reversed.a == model.a && reversed.b == model.b && sameParabolaCircleCrossingTrim(model.curve, reversed.c))
		})
		if source < 0 || mapped[source] >= 0 {
			return h, nil, geom.ErrConvergence
		}
		mapped[source] = id
		for _, face := range h.RoofFaces {
			if !slices.ContainsFunc(part.faces[face].Outer, func(use Use) bool {
				return use.Edge == id && !use.Reversed && use.UVStart == model.a && use.UVEnd == model.b &&
					sameParabolaCircleCrossingTrim(use.Curve, model.curve)
			}) {
				return h, nil, geom.ErrConvergence
			}
		}
	}
	h.ParabolaEdges = [twoCrossingCirclesParabolaPieces]int(mapped[:twoCrossingCirclesParabolaPieces])
	for i, ring := range p.circles {
		for _, piece := range ring {
			h.CircleEdges[i] = append(h.CircleEdges[i], mapped[piece])
		}
	}
	for i, piece := range reg.loop[count:] {
		id := h.WallFaces[i]
		if piece.side != h.DomainSides[i] {
			return h, nil, geom.ErrConvergence
		}
		var seen [2]bool
		for _, use := range part.faces[id].Outer {
			for owner, roof := range roofs {
				trim, ok := graphWallSourceTrim(use.Curve, owner == 0, false)
				if ok && trim.surface == roof && trim.a == piece.a && trim.b == piece.b && use.Reversed == (owner == 0) {
					if seen[owner] {
						return h, nil, geom.ErrConvergence
					}
					seen[owner] = true
				}
			}
		}
		if seen != ([2]bool{true, true}) {
			return h, nil, geom.ErrConvergence
		}
	}
	if holes == 0 {
		return h, mapped, validatePolynomialCutBoundary(ctx, part, assembly, opts)
	}
	if len(topo.Shells) != 1 || topo.EulerCharacteristic != 0 || topo.CollapsedEdges != 0 {
		return h, nil, geom.ErrConvergence
	}
	if err := materialVertexLinks(ctx, part, true); err != nil {
		return h, nil, err
	}
	return h, mapped, validateExactGraphPartitionEdges(ctx, part, opts)
}

func bindTwoCrossingCircleCutContacts(p parabolaTwoCrossingCirclesPartition, components []Solid, regions [3]int,
	assembly [3]OpenSectionHistory, edges [3][]int) (out [2][2]ParabolaTwoCrossingCirclesCutContact, err error) {
	material := polynomialNodeCutMaterial{components: components}
	var used [3][]int
	for i, nodes := range p.proof.Nodes {
		for j, node := range nodes {
			contact := ParabolaTwoCrossingCirclesCutContact{Parameters: node.Parameters, Point: node.Point, Components: [2]int{0, i + 1}}
			var sectors [2]int
			for k, part := range contact.Components {
				region := regions[part]
				sector := slices.Index(node.SectorRegions[:], region)
				if sector < 0 {
					return out, geom.ErrConvergence
				}
				sectors[k] = sector
				body := components[part]
				v, err := exactParabolaCircleCrossingVertex(body, node.Point)
				if err != nil {
					return out, err
				}
				index, err := polynomialCutContactVertex(body, assembly[part], body.vertices[v])
				if err != nil || index != v || slices.Contains(used[part], v) {
					return out, geom.ErrConvergence
				}
				used[part] = append(used[part], v)
				contact.Vertices[k] = v
				for l, ray := range [2]int{sector, (sector + 1) % 4} {
					id := edges[part][p.rays[i][j][ray]]
					if id < 0 || !matchesParabolaCircleCrossingRay(body, id, v, node.Rays[ray]) {
						return out, geom.ErrConvergence
					}
					contact.Rays[k][l] = id
				}
			}
			if (sectors[0]+2)%4 != sectors[1] {
				return out, geom.ErrConvergence
			}
			out[i][j] = contact
			material.contacts = append(material.contacts, TwoPolynomialNodeComponentContact{Parameters: node.Parameters, Point: node.Point,
				Components: contact.Components, Vertices: contact.Vertices})
		}
	}
	return out, material.validateContactIdentities()
}
