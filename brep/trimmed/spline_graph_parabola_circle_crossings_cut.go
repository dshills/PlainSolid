package trimmed

import (
	"context"
	"fmt"
	"math/big"
	"slices"

	"plainsolid/geom"
)

// ParabolaCircleCrossingsCutHistory binds one component to its region and
// original roof and boundary pieces. Values of -1 denote absent source pieces
// or wall entries. All native IDs are local to the component.
type ParabolaCircleCrossingsCutHistory struct {
	Region        int
	RoofFaces     [2]int
	ParabolaEdges [3]int
	CircleEdges   [4]int
	WallFaces     [3]int
	DomainSides   [3]int
}

// ParabolaCircleCrossingsCutContact binds one original shared point to each
// separate shell's local vertex and two outgoing rays. Components are ordered
// as in Components, and Rays are counterclockwise around the positive sector.
type ParabolaCircleCrossingsCutContact struct {
	Parameters [2]*big.Rat
	Point      [3]*big.Rat
	Vertices   [2]int
	Rays       [2][2]int
}

// ParabolaCircleCrossingsCut retains an exterior layer and an interior cap
// with distinct vertex namespaces and exactly two shared closure points.
type ParabolaCircleCrossingsCut struct {
	sources    [3]*geom.BSplineSurface
	components [2]Solid
	history    [2]ParabolaCircleCrossingsCutHistory
	contacts   [2]ParabolaCircleCrossingsCutContact
	axis       int
	tolerance  float64
}

// Components returns the exterior layer and the disk cap in that order.
func (r *ParabolaCircleCrossingsCut) Components() [2]Solid {
	if r == nil {
		return [2]Solid{}
	}
	return r.components
}

func (r *ParabolaCircleCrossingsCut) History() [2]ParabolaCircleCrossingsCutHistory {
	if r == nil {
		return [2]ParabolaCircleCrossingsCutHistory{}
	}
	return r.history
}

// Contacts returns both original exact junctions in increasing free order.
// The exact point and parameter values are caller-owned copies.
func (r *ParabolaCircleCrossingsCut) Contacts() [2]ParabolaCircleCrossingsCutContact {
	if r == nil || r.sources[0] == nil {
		return [2]ParabolaCircleCrossingsCutContact{}
	}
	return cloneParabolaCrossingCutContacts(r.contacts)
}

func cloneParabolaCrossingCutContacts(contacts [2]ParabolaCircleCrossingsCutContact) [2]ParabolaCircleCrossingsCutContact {
	out := contacts
	for i, contact := range contacts {
		for j, q := range contact.Parameters {
			out[i].Parameters[j] = new(big.Rat).Set(q)
		}
		for j, q := range contact.Point {
			out[i].Point[j] = new(big.Rat).Set(q)
		}
	}
	return out
}

// CutParabolaCircleCrossingsSplineGraphs subtracts the original graph stocks
// into two individually embedded positive shells. The source signs select
// opposite sectors at both junctions. Their chart closures intersect only
// where both factors vanish, and each shared vertical fiber is one point.
func CutParabolaCircleCrossingsSplineGraphs(ctx context.Context, base, a, b *geom.BSplineSurface, axis int,
	tolerance float64, o ParabolaCircleGraphOptions) (*ParabolaCircleCrossingsCut, error) {
	sources := [3]*geom.BSplineSurface{base, a, b}
	p, err := proveParabolaCircleCrossingsPartition(ctx, sources, axis, tolerance, o)
	if err != nil {
		return nil, err
	}
	out := &ParabolaCircleCrossingsCut{sources: sources, axis: axis, tolerance: tolerance}
	var regions []int
	for i, sign := range p.proof.RegionSides {
		if sign > 0 {
			regions = append(regions, i)
		}
	}
	if !slices.Equal(regions, []int{0, 3}) && !slices.Equal(regions, []int{1, 2}) {
		return nil, geom.ErrConvergence
	}
	evaluate := exactGraphVertexEvaluator(ctx, o.Proof)
	var walls [2]OpenSectionHistory
	for part, region := range regions {
		body, assembly, err := assembleOpenSplineGraphPairOnCharts(ctx, a, b, 0, true, tolerance, p.regions[region].loop,
			[2]graphEnvelopeChart{}, nil, evaluate)
		if err != nil {
			return nil, err
		}
		history, err := bindParabolaCircleCrossingsCutComponent(ctx, body, assembly, p, region, [2]*geom.BSplineSurface{a, b}, o.Proof)
		if err != nil {
			return nil, fmt.Errorf("trimmed: parabola-circle crossings Cut region %d: %w", region, err)
		}
		out.components[part], out.history[part], walls[part] = body, history, assembly
	}
	out.contacts, err = bindParabolaCrossingCutContacts(p, out.components[:], out.history, walls)
	if err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func bindParabolaCrossingCutContacts(p parabolaCircleCrossingsPartition, parts []Solid,
	histories [2]ParabolaCircleCrossingsCutHistory, walls [2]OpenSectionHistory) (contacts [2]ParabolaCircleCrossingsCutContact, err error) {
	material := polynomialNodeCutMaterial{components: parts}
	for i, node := range p.proof.Nodes {
		contact := ParabolaCircleCrossingsCutContact{Parameters: node.Parameters, Point: node.Point}
		sourceRays := p.rayPieces(i)
		var sectors [2]int
		for part, history := range histories {
			region := history.Region
			sector := slices.Index(node.SectorRegions[:], region)
			if sector < 0 {
				return contacts, geom.ErrConvergence
			}
			sectors[part] = sector
			body := parts[part]
			v, err := exactParabolaCircleCrossingVertex(body, node.Point)
			if err != nil {
				return contacts, err
			}
			index, err := polynomialCutContactVertex(body, walls[part], body.vertices[v])
			if err != nil || index != v {
				return contacts, geom.ErrConvergence
			}
			contact.Vertices[part] = v
			native := [7]int{history.ParabolaEdges[0], history.ParabolaEdges[1], history.ParabolaEdges[2],
				history.CircleEdges[0], history.CircleEdges[1], history.CircleEdges[2], history.CircleEdges[3]}
			for j, ray := range [2]int{sector, (sector + 1) % 4} {
				id := native[sourceRays[ray]]
				if id < 0 || !matchesParabolaCircleCrossingRay(body, id, v, node.Rays[ray]) {
					return contacts, geom.ErrConvergence
				}
				contact.Rays[part][j] = id
			}
		}
		if (sectors[0]+2)%4 != sectors[1] || (i > 0 &&
			(contact.Vertices[0] == contacts[0].Vertices[0] || contact.Vertices[1] == contacts[0].Vertices[1])) {
			return contacts, geom.ErrConvergence
		}
		contacts[i] = contact
		material.contacts = append(material.contacts, TwoPolynomialNodeComponentContact{
			Parameters: node.Parameters, Point: node.Point, Components: [2]int{0, 1}, Vertices: contact.Vertices,
		})
	}
	if err := material.validateContactIdentities(); err != nil {
		return contacts, err
	}
	return contacts, nil
}

func bindParabolaCircleCrossingsCutComponent(ctx context.Context, part Solid, assembly OpenSectionHistory,
	p parabolaCircleCrossingsPartition, region int, roofs [2]*geom.BSplineSurface, opts geom.GraphPointContactOptions) (h ParabolaCircleCrossingsCutHistory, err error) {
	h = ParabolaCircleCrossingsCutHistory{
		Region: region, RoofFaces: [2]int{assembly.SplineFace, assembly.PlaneFace}, ParabolaEdges: [3]int{-1, -1, -1},
		CircleEdges: [4]int{-1, -1, -1, -1}, WallFaces: [3]int{-1, -1, -1}, DomainSides: [3]int{-1, -1, -1},
	}
	topo := part.Topology()
	reg := p.regions[region]
	holes := len(reg.holes)
	want, wallCount := [3]int{3, 3, 2}, 0
	if region < 2 {
		want, wallCount = [3]int{9, 12, 5}, 3
	}
	if holes > 1 || (holes == 1 && (region >= 2 || len(reg.holes[0]) != 4)) {
		return h, geom.ErrConvergence
	}
	want[0] += 4 * holes
	want[1] += 4 * holes
	sourceEdges := [4][]int{{0, 2, 5, 6}, {0, 2, 3, 4}, {1, 5, 6}, {1, 3, 4}}[region]
	if ([3]int{topo.Vertices, topo.Edges, topo.Faces}) != want || topo.InnerWires != 2*holes ||
		len(assembly.WallFaces) != wallCount || len(assembly.DomainSides) != wallCount || len(assembly.ContourEdges) != len(sourceEdges)+4*holes {
		return h, geom.ErrConvergence
	}
	loop := p.regions[region].loop
	for owner, id := range h.RoofFaces {
		if err := bindParabolaCirclesRegionFace(ctx, part.faces[id], roofs[owner], owner == 1, reg, p.proof.RegionWitnesses[region]); err != nil {
			return h, err
		}
	}
	pieces := make([]openSectionPiece, 0, len(sourceEdges))
	for _, source := range sourceEdges {
		pieces = append(pieces, p.pieces[source])
	}
	edges, err := bindParabolaCircleCrossingEdges(part, assembly.ContourEdges, pieces)
	if err != nil {
		return h, err
	}
	for i, source := range sourceEdges {
		id := edges[i]
		trim := part.edges[id].Curve.(surfaceTrimCurve)
		if trim.surface != roofs[0] {
			return h, geom.ErrConvergence
		}
		for _, roofFace := range h.RoofFaces {
			if !slices.ContainsFunc(part.faces[roofFace].Outer, func(use Use) bool {
				return use.Edge == id && !use.Reversed && use.UVStart == trim.a && use.UVEnd == trim.b &&
					sameParabolaCircleCrossingTrim(use.Curve, trim.curve)
			}) {
				return h, geom.ErrConvergence
			}
		}
		if source < 3 {
			h.ParabolaEdges[source] = id
		} else {
			h.CircleEdges[source-3] = id
		}
	}
	copy(h.WallFaces[:], assembly.WallFaces)
	copy(h.DomainSides[:], assembly.DomainSides)
	for i, piece := range loop[len(loop)-wallCount:] {
		id := h.WallFaces[i]
		if piece.side != h.DomainSides[i] {
			return h, geom.ErrConvergence
		}
		var seen [2]bool
		for _, use := range part.faces[id].Outer {
			for owner, roof := range roofs {
				trim, ok := graphWallSourceTrim(use.Curve, owner == 0, false)
				if ok && trim.surface == roof && trim.a == piece.a && trim.b == piece.b && use.Reversed == (owner == 0) {
					if seen[owner] {
						return h, geom.ErrConvergence
					}
					seen[owner] = true
				}
			}
		}
		if seen != ([2]bool{true, true}) {
			return h, geom.ErrConvergence
		}
	}
	if holes == 0 {
		return h, validatePolynomialCutBoundary(ctx, part, assembly, opts)
	}
	if len(topo.Shells) != 1 || topo.EulerCharacteristic != 0 || topo.CollapsedEdges != 0 {
		return h, geom.ErrConvergence
	}
	if err := materialVertexLinks(ctx, part, true); err != nil {
		return h, err
	}
	return h, validateExactGraphPartitionEdges(ctx, part, opts)
}
