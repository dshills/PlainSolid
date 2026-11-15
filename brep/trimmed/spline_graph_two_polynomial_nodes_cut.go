package trimmed

import (
	"context"
	"fmt"
	"math/big"
	"slices"

	"plainsolid/geom"
)

// TwoPolynomialNodeComponentContact binds one exact original junction to the
// local vertex IDs of its two incident components. Components are ordered by
// their indices in the Cut result.
type TwoPolynomialNodeComponentContact struct {
	Parameters [2]*big.Rat
	Point      [3]*big.Rat
	Components [2]int
	Vertices   [2]int
}

// TwoPolynomialNodeCut retains the original source recipe and the independently
// embedded, outward Cut components. Its contact ledger lists the only points
// shared by the closures of the component materials.
type TwoPolynomialNodeCut struct {
	sources    [3]*geom.BSplineSurface
	components []Solid
	regions    []int
	contacts   [2]TwoPolynomialNodeComponentContact
	axis       int
	tolerance  float64
}

// Components returns the embedded shells in original arrangement region order.
func (r *TwoPolynomialNodeCut) Components() []Solid {
	if r == nil {
		return nil
	}
	return append([]Solid(nil), r.components...)
}

// Regions returns the corresponding indices into the certified arrangement.
func (r *TwoPolynomialNodeCut) Regions() []int {
	if r == nil {
		return nil
	}
	return append([]int(nil), r.regions...)
}

// Contacts returns both exact point contacts in increasing free-coordinate
// order, with caller-owned rational coordinates.
func (r *TwoPolynomialNodeCut) Contacts() [2]TwoPolynomialNodeComponentContact {
	if r == nil || r.sources[0] == nil {
		return [2]TwoPolynomialNodeComponentContact{}
	}
	out := r.contacts
	for i, c := range out {
		out[i] = c.copy()
	}
	return out
}

func (c TwoPolynomialNodeComponentContact) copy() TwoPolynomialNodeComponentContact {
	for i, x := range c.Parameters {
		c.Parameters[i] = new(big.Rat).Set(x)
	}
	for i, x := range c.Point {
		c.Point[i] = new(big.Rat).Set(x)
	}
	return c
}

// CutSplineGraphTwoPolynomialNodeComponents cuts A by B across a complete
// two-node arrangement on their original shared-base identity XY charts.
// It proves two exterior components sharing both nodes, or three between-
// branch components with two successive point contacts. Each vertex identity
// belongs to one component, even when its position matches another component.
func CutSplineGraphTwoPolynomialNodeComponents(ctx context.Context, base, a, b *geom.BSplineSurface, axis int, tolerance float64, o PolynomialNodeCutOptions) (*TwoPolynomialNodeCut, error) {
	if ctx == nil || base == nil || a == nil || b == nil || axis < 0 || axis > 1 ||
		!finite(tolerance) || tolerance <= 0 || o.Section.Domain != (geom.ParameterRectangle{}) {
		return nil, geom.ErrInvalid
	}
	patches, e := prepareSharedBaseGraphContact(ctx, base, a, b, o.Section)
	if e != nil {
		return nil, e
	}
	arrangement, e := geom.DiscoverSplineGraphTwoPolynomialNodes(ctx, patches[0], patches[1], axis, o.Proof)
	if e != nil {
		return nil, e
	}
	if !arrangement.Proven || arrangement.Domain != (geom.ParameterRectangle{UMax: 1, VMax: 1}) || arrangement.Graph.ProjectionOrientation != 1 {
		return nil, geom.ErrConvergence
	}
	uv, xyz, e := arrangement.RepresentedVertices()
	if e != nil {
		return nil, e
	}
	pieces, e := twoPolynomialNodePieces(arrangement, uv)
	if e != nil {
		return nil, e
	}
	loops, e := twoPolynomialNodeRegionLoops(arrangement, uv, pieces)
	if e != nil {
		return nil, e
	}
	material, e := assemblePolynomialNodeCutComponents(ctx, a, b, tolerance, o.Proof, arrangement.Regions[:], arrangement.Nodes[:], arrangement.Vertices[:], xyz[:], loops[:])
	if e != nil {
		return nil, e
	}
	return &TwoPolynomialNodeCut{
		sources: [3]*geom.BSplineSurface{base, a, b}, axis: axis, tolerance: tolerance,
		components: material.components, regions: material.regions, contacts: [2]TwoPolynomialNodeComponentContact(material.contacts),
	}, nil
}

type polynomialNodeCutMaterial struct {
	components []Solid
	regions    []int
	contacts   []TwoPolynomialNodeComponentContact
}

func assemblePolynomialNodeCutComponents(ctx context.Context, a, b *geom.BSplineSurface, tolerance float64, proof geom.GraphPointContactOptions,
	regions []geom.SplineGraphPolynomialRegion, nodes []geom.SplineGraphPolynomialJunction, vertices []geom.SplineGraphPolynomialVertex, xyz []geom.Vec3,
	loops [][]openSectionPiece) (polynomialNodeCutMaterial, error) {
	var material polynomialNodeCutMaterial
	componentByRegion := make([]int, len(regions))
	for i, region := range regions {
		componentByRegion[i] = -1
		if region.HeightSide > 0 {
			componentByRegion[i] = len(material.regions)
			material.regions = append(material.regions, i)
		}
	}

	// The exterior regions have disjoint projections except where the two
	// branch values agree. The between-branch regions occupy consecutive
	// open free-coordinate slabs whose neighboring closures meet only at
	// their common node. At every such junction the shared vertical fiber
	// collapses exactly to the original model point.
	if len(regions) != len(nodes)+3 {
		return polynomialNodeCutMaterial{}, geom.ErrConvergence
	}
	exterior := slices.Equal(material.regions, []int{0, len(regions) - 1})
	if !exterior {
		if len(material.regions) != len(nodes)+1 {
			return polynomialNodeCutMaterial{}, geom.ErrConvergence
		}
		for i, region := range material.regions {
			if region != i+1 {
				return polynomialNodeCutMaterial{}, geom.ErrConvergence
			}
		}
	}

	histories := make([]OpenSectionHistory, 0, len(material.regions))
	evaluate := exactGraphVertexEvaluator(ctx, proof)
	for _, index := range material.regions {
		loop := loops[index]
		if len(loop) == 3 {
			// The only perimeter side of this end wedge has a zero-height
			// upright at each end. Split it to retain a nonzero wall upright.
			p, e := splitGraphSideAtMidpoint(loop[2])
			if e != nil {
				return polynomialNodeCutMaterial{}, e
			}
			loop = []openSectionPiece{loop[0], loop[1], p[0], p[1]}
		}
		part, history, e := assembleOpenSplineGraphPair(a, b, 0, true, tolerance, loop, evaluate)
		if e != nil {
			return polynomialNodeCutMaterial{}, e
		}
		if e = validatePolynomialCutLayer(ctx, part, history, regions[index].Witness, proof); e != nil {
			return polynomialNodeCutMaterial{}, fmt.Errorf("trimmed: polynomial node Cut region %d: %w", index, e)
		}
		material.components = append(material.components, part)
		histories = append(histories, history)
	}

	for i, node := range nodes {
		var components [2]int
		count, firstSector := 0, -1
		for sector, region := range node.Regions {
			component := componentByRegion[region]
			if component < 0 {
				continue
			}
			if count == 0 {
				firstSector = sector
			} else if count != 1 || sector-firstSector != 2 {
				return polynomialNodeCutMaterial{}, geom.ErrConvergence
			}
			components[count] = component
			count++
		}
		if count != 2 {
			return polynomialNodeCutMaterial{}, geom.ErrConvergence
		}
		if components[0] > components[1] {
			components[0], components[1] = components[1], components[0]
		}
		wantPair := [2]int{0, 1}
		if !exterior {
			wantPair = [2]int{i, i + 1}
		}
		if components != wantPair {
			return polynomialNodeCutMaterial{}, geom.ErrConvergence
		}
		p := vertices[node.Vertex]
		contact := TwoPolynomialNodeComponentContact{Parameters: p.Parameters, Point: p.Point, Components: components}
		for j, component := range components {
			vertex, e := polynomialCutContactVertex(material.components[component], histories[component], xyz[node.Vertex])
			if e != nil {
				return polynomialNodeCutMaterial{}, e
			}
			contact.Vertices[j] = vertex
		}
		material.contacts = append(material.contacts, contact)
	}
	if e := material.validateContactIdentities(); e != nil {
		return polynomialNodeCutMaterial{}, e
	}
	return material, nil
}

func (r polynomialNodeCutMaterial) validateContactIdentities() error {
	for i, a := range r.components {
		for j := i + 1; j < len(r.components); j++ {
			for v, p := range a.vertices {
				for w, q := range r.components[j].vertices {
					if p != q {
						continue
					}
					bound := false
					for _, contact := range r.contacts {
						bound = bound || contact.Components == [2]int{i, j} && contact.Vertices == [2]int{v, w}
					}
					if !bound {
						return geom.ErrConvergence
					}
				}
			}
		}
	}
	return nil
}
