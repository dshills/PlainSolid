package trimmed

import (
	"context"
	"fmt"
	"slices"

	"plainsolid/geom"
)

// ParabolaCircleFourCrossingsCutHistory records the original owner faces
// and contour on one positive shell. Edges follow the envelope order, with
// -1 for original pieces outside this component.
type ParabolaCircleFourCrossingsCutHistory struct {
	Region        int
	RoofFaces     [2]int
	ParabolaEdges [fourCrossingParabolaPieces]int
	CircleEdges   []int
	WallFaces     []int
	DomainSides   []int
}

// ParabolaCircleFourCrossingsCut holds the separate original difference
// shells in source region order. One difference has two shells that meet
// at all four junctions. The other has four shells connected by four
// indexed point contacts.
type ParabolaCircleFourCrossingsCut struct {
	components []Solid
	history    []ParabolaCircleFourCrossingsCutHistory
	contacts   [4]ParabolaTwoCrossingCirclesCutContact
	sources    [3]*geom.BSplineSurface
	axis       int
	tolerance  float64
}

func (r *ParabolaCircleFourCrossingsCut) Components() []Solid {
	if r == nil {
		return nil
	}
	return slices.Clone(r.components)
}

func (r *ParabolaCircleFourCrossingsCut) History() []ParabolaCircleFourCrossingsCutHistory {
	if r == nil {
		return nil
	}
	history := slices.Clone(r.history)
	for i := range history {
		history[i].CircleEdges = slices.Clone(history[i].CircleEdges)
		history[i].WallFaces = slices.Clone(history[i].WallFaces)
		history[i].DomainSides = slices.Clone(history[i].DomainSides)
	}
	return history
}

// Contacts returns the exact original junctions in increasing free
// parameter. Component indices are in ascending order at each contact.
// Ray pairs retain the local order bounding that component's sector.
func (r *ParabolaCircleFourCrossingsCut) Contacts() (contacts [4]ParabolaTwoCrossingCirclesCutContact) {
	if r == nil {
		return contacts
	}
	for i, contact := range r.contacts {
		contacts[i] = cloneCrossingCircleCutContact(contact)
	}
	return contacts
}

// CutParabolaCircleFourCrossingsSplineGraphs constructs the original ordered
// difference. Strict source signs prove every selected layer positive. The
// original partition and its vertex fans prove which distinct layers can
// touch, and both native occurrences are authenticated at each junction.
func CutParabolaCircleFourCrossingsSplineGraphs(ctx context.Context, base, a, b *geom.BSplineSurface, axis int,
	tolerance float64, o ParabolaCircleGraphOptions) (*ParabolaCircleFourCrossingsCut, error) {
	src := [3]*geom.BSplineSurface{base, a, b}
	p, err := proveParabolaCircleFourCrossingPartition(ctx, src, axis, tolerance, o)
	if err != nil {
		return nil, err
	}
	out := &ParabolaCircleFourCrossingsCut{sources: src, axis: axis, tolerance: tolerance}
	evaluate := exactGraphVertexEvaluator(ctx, o.Proof)
	roofs := [2]*geom.BSplineSurface{a, b}
	regionComponent := [6]int{-1, -1, -1, -1, -1, -1}
	var binding crossingCircleCutBinding
	for region, reg := range p.regions {
		if reg.sign < 0 {
			continue
		}
		if reg.sign != 1 {
			return nil, geom.ErrConvergence
		}
		id := len(out.components)
		s, assembly, err := assembleOpenSplineGraphPairOnCharts(ctx, a, b, 0, true, tolerance, reg.loop,
			[2]graphEnvelopeChart{}, nil, evaluate)
		if err != nil {
			return nil, fmt.Errorf("trimmed: four crossings of one circle Cut region %d: %w", region, err)
		}
		edges, err := bindCrossingCircleCutComponent(ctx, s, assembly, p.pieces, region, reg, p.proof.RegionWitnesses[region], roofs, o.Proof)
		if err != nil {
			return nil, fmt.Errorf("trimmed: four crossings of one circle Cut region %d source binding: %w", region, err)
		}
		h := ParabolaCircleFourCrossingsCutHistory{Region: region, RoofFaces: [2]int{assembly.SplineFace, assembly.PlaneFace},
			ParabolaEdges: [fourCrossingParabolaPieces]int(edges[:fourCrossingParabolaPieces]), CircleEdges: edges[fourCrossingParabolaPieces:],
			WallFaces: assembly.WallFaces, DomainSides: assembly.DomainSides}
		regionComponent[region] = id
		out.components = append(out.components, s)
		out.history = append(out.history, h)
		binding.regions = append(binding.regions, region)
		binding.assembly = append(binding.assembly, assembly)
		binding.edges = append(binding.edges, edges)
	}
	if len(out.components) != 2 && len(out.components) != 4 {
		return nil, geom.ErrConvergence
	}
	binding.material.components = out.components
	out.contacts, err = bindParabolaCircleFourCrossingCutContacts(p, binding, regionComponent[:])
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func bindParabolaCircleFourCrossingCutContacts(p parabolaCircleFourCrossingPartition, binding crossingCircleCutBinding,
	regionComponent []int) (contacts [4]ParabolaTwoCrossingCirclesCutContact, err error) {
	binding.used = make([][]int, len(binding.material.components))
	for i, node := range p.proof.Nodes {
		pair := [2]int{-1, -1}
		count := 0
		for _, region := range node.SectorRegions {
			id := regionComponent[region]
			if id < 0 {
				continue
			}
			if count >= len(pair) {
				return contacts, geom.ErrConvergence
			}
			pair[count] = id
			count++
		}
		if count != len(pair) || pair[0] == pair[1] {
			return contacts, geom.ErrConvergence
		}
		if pair[0] > pair[1] {
			pair[0], pair[1] = pair[1], pair[0]
		}
		contacts[i], err = binding.contact(node, p.rays[i], pair)
		if err != nil {
			return contacts, fmt.Errorf("trimmed: four crossings of one circle Cut contact %d: %w", i, err)
		}
	}
	return contacts, binding.material.validateContactIdentities()
}
