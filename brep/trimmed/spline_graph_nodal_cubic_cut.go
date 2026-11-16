package trimmed

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// NodalCubicCutOptions bounds the original chart, clearance and section proofs.
type NodalCubicCutOptions = NodalCubicCommonOptions

// NodalCubicCutContact gives the shared original point, both source UVs,
// and each separate shell's vertex and two incident section rays.
type NodalCubicCutContact struct {
	UV       [2][2]*big.Rat
	Point    [3]*big.Rat
	Vertices [2]int
	Rays     [2][2]int
}

// NodalCubicCut retains the exact positive loop and arm layers with one
// point contact between their independently embedded closed boundaries.
type NodalCubicCut struct {
	sources    [3]*geom.BSplineSurface
	axis       int
	tolerance  float64
	components [2]Solid
	boundaries [2]OpenSectionHistory
	contact    NodalCubicCutContact
	volumes    [2]*big.Rat
}

// Components returns the loop component followed by the between-arms component.
func (r *NodalCubicCut) Components() []Solid {
	if r == nil || r.sources[0] == nil {
		return nil
	}
	return append([]Solid(nil), r.components[:]...)
}

// Contact returns caller-owned copies of the exact source coordinates.
func (r *NodalCubicCut) Contact() NodalCubicCutContact {
	if r == nil || r.sources[0] == nil {
		return NodalCubicCutContact{}
	}
	c := r.contact
	for i := range c.UV {
		for j, q := range c.UV[i] {
			c.UV[i][j] = new(big.Rat).Set(q)
		}
	}
	for i, q := range c.Point {
		c.Point[i] = new(big.Rat).Set(q)
	}
	return c
}

// ExactComponentVolumes returns the independent original source integrals
// in component order, followed by their sum.
func (r *NodalCubicCut) ExactComponentVolumes() (volumes [2]*big.Rat, total *big.Rat) {
	if r == nil || r.sources[0] == nil {
		return volumes, nil
	}
	for i, v := range r.volumes {
		volumes[i] = new(big.Rat).Set(v)
	}
	return volumes, new(big.Rat).Add(volumes[0], volumes[1])
}

// CutNodalCubicSplineGraphs subtracts B from A on their original shared-base
// rectangular footprint. It admits the order retaining both positive
// interior layers with polynomial heights and exactly represented vertices.
// The exterior order pinches at the node and returns geom.ErrDegenerate.
func CutNodalCubicSplineGraphs(ctx context.Context, base, a, b *geom.BSplineSurface, freeAxis int,
	tolerance float64, o NodalCubicCutOptions) (*NodalCubicCut, error) {
	sources := [3]*geom.BSplineSurface{base, a, b}
	section, err := prepareNodalCubicNativeSection(ctx, sources, freeAxis, tolerance, o)
	if err != nil {
		return nil, err
	}
	proof := section.proof
	if proof.Regions[0].HeightSide > 0 {
		// Its two opposite positive sectors are connected around the
		// turn. They are two links of the same pinched component.
		return nil, fmt.Errorf("trimmed: nodal cubic exterior Cut pinches at its original node: %w", geom.ErrDegenerate)
	}
	if proof.Regions[0].HeightSide != -1 || proof.Regions[1].HeightSide != 1 || proof.Regions[2].HeightSide != 1 ||
		proof.Sectors != ([4]int{0, 1, 0, 2}) {
		return nil, geom.ErrConvergence
	}
	volumes, err := nodalCubicCutVolumes(ctx, [2]*geom.BSplineSurface{a, b}, section, o)
	if err != nil {
		return nil, err
	}

	arms := section.regions[3].loop
	if len(arms) != 3 || arms[1].side < 0 {
		return nil, geom.ErrConvergence
	}
	// One positive upright is needed between the two zero-thickness
	// terminal fibers on the far stock side.
	wall, err := splitGraphSideAtMidpoint(arms[1])
	if err != nil {
		return nil, err
	}
	loops := [2][]openSectionPiece{section.regions[2].loop, {arms[0], wall[0], wall[1], arms[2]}}
	node := proof.Vertices[0]
	out := &NodalCubicCut{
		sources: sources, axis: freeAxis, tolerance: tolerance, volumes: volumes,
		contact: NodalCubicCutContact{UV: node.UV, Point: node.Point},
	}
	evaluate := exactGraphVertexEvaluator(ctx, o.Proof)
	roofs := [2]*geom.BSplineSurface{a, b}
	for i, loop := range loops {
		part, h, err := assembleOpenSplineGraphPairOnCharts(ctx, a, b, 0, true, tolerance, loop,
			section.charts, nodalCubicReferenceXY, evaluate)
		if err != nil {
			return nil, err
		}
		expected, err := nodalCubicCutOwnerPieces(section.charts, loop)
		if err != nil {
			return nil, err
		}
		if err := bindAlignedPolynomialCutLayer(ctx, part, h, roofs, section.charts, loop, expected, o.Proof); err != nil {
			return nil, fmt.Errorf("trimmed: nodal cubic Cut region %d: %w", i+1, err)
		}
		for owner, index := range [2]int{h.SplineFace, h.PlaneFace} {
			where, loopProof, err := LocateRationalInFaceTrim(ctx, part.faces[index], proof.Regions[i+1].UV[owner], trimvalidate.Options{})
			if err != nil {
				return nil, err
			}
			if loopProof.Status != trimvalidate.Certified || where != trimvalidate.Inside {
				return nil, geom.ErrConvergence
			}
		}
		vertex, err := polynomialCutContactVertex(part, h, section.vertices[0])
		if err != nil {
			return nil, err
		}
		rays := ([2][2]int{{1, 2}, {3, 0}})[i]
		ids, err := bindNodalCubicCutRays(part, h, proof, vertex, rays)
		if err != nil {
			return nil, err
		}
		out.components[i], out.boundaries[i] = part, h
		out.contact.Vertices[i], out.contact.Rays[i] = vertex, ids
	}
	// The exact loop has x<0 and the exact arm region has x>0. Each
	// closure meets x=0 only at the original node, where both roofs
	// agree. Check the separately indexed native realization as well.
	material := polynomialNodeCutMaterial{components: out.components[:],
		contacts: []TwoPolynomialNodeComponentContact{{
			Parameters: node.UV[0], Point: node.Point, Components: [2]int{0, 1}, Vertices: out.contact.Vertices,
		}},
	}
	if err := material.validateContactIdentities(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func nodalCubicCutOwnerPieces(charts [2]graphEnvelopeChart, loop []openSectionPiece) ([2]map[[2]geom.Vec2]openSectionPiece, error) {
	var out [2]map[[2]geom.Vec2]openSectionPiece
	for owner, chart := range charts {
		out[owner] = make(map[[2]geom.Vec2]openSectionPiece)
		for _, p := range loop {
			if p.side < 0 {
				q, err := chart.piece(p)
				if err != nil {
					return out, err
				}
				out[owner][[2]geom.Vec2{p.a, p.b}] = q
			}
		}
	}
	return out, nil
}

func bindNodalCubicCutRays(part Solid, h OpenSectionHistory, p *geom.NodalCubicSection, vertex int, rays [2]int) ([2]int, error) {
	ids := [2]int{-1, -1}
	for _, id := range h.ContourEdges {
		edge := part.edges[id]
		if edge.Start != vertex && edge.End != vertex {
			continue
		}
		curve := edge.Curve.(surfaceTrimCurve).curve.(ExactBezierUV)
		for i, ray := range rays {
			if nodalCubicRayMatches(curve, edge.End == vertex, p.Rays[ray].UV[0]) {
				if ids[i] >= 0 {
					return [2]int{}, geom.ErrConvergence
				}
				ids[i] = id
			}
		}
	}
	if ids[0] < 0 || ids[1] < 0 || ids[0] == ids[1] {
		return [2]int{}, geom.ErrConvergence
	}
	return ids, nil
}
