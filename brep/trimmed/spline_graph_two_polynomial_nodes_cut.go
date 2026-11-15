package trimmed

import (
	"context"
	"fmt"
	"math/big"

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
	for i := range out {
		for j, x := range out[i].Parameters {
			out[i].Parameters[j] = new(big.Rat).Set(x)
		}
		for j, x := range out[i].Point {
			out[i].Point[j] = new(big.Rat).Set(x)
		}
	}
	return out
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
	r := &TwoPolynomialNodeCut{sources: [3]*geom.BSplineSurface{base, a, b}, axis: axis, tolerance: tolerance}
	componentByRegion := [5]int{-1, -1, -1, -1, -1}
	for i, region := range arrangement.Regions {
		if region.HeightSide > 0 {
			componentByRegion[i] = len(r.regions)
			r.regions = append(r.regions, i)
		}
	}
	// The exterior regions can share material only where both branch values
	// agree. Between-branch regions lie in disjoint open free-coordinate slabs.
	// Neighboring slabs share a single node fiber on their common boundary,
	// and the first and third slabs have disjoint closures. The height gap
	// vanishes exactly on each node fiber of the retained closed regions.
	exterior := len(r.regions) == 2 && [2]int(r.regions) == ([2]int{0, 4})
	between := len(r.regions) == 3 && [3]int(r.regions) == ([3]int{1, 2, 3})
	if !exterior && !between {
		return nil, geom.ErrConvergence
	}
	var histories []OpenSectionHistory
	evaluate := exactGraphVertexEvaluator(ctx, o.Proof)
	for _, index := range r.regions {
		loop := loops[index]
		if len(loop) == 3 {
			// One perimeter side joins two collapsed endpoints of a wedge.
			// An exact interior upright splits it into two native wall loops.
			p, e := splitGraphSideAtMidpoint(loop[2])
			if e != nil {
				return nil, e
			}
			loop = []openSectionPiece{loop[0], loop[1], p[0], p[1]}
		}
		part, history, e := assembleOpenSplineGraphPair(a, b, 0, true, tolerance, loop, evaluate)
		if e != nil {
			return nil, e
		}
		if e = validatePolynomialCutLayer(ctx, part, history, arrangement.Regions[index].Witness, o.Proof); e != nil {
			return nil, fmt.Errorf("trimmed: two polynomial node Cut region %d: %w", index, e)
		}
		r.components = append(r.components, part)
		histories = append(histories, history)
	}
	for i, node := range arrangement.Nodes {
		var participants []int
		for _, region := range node.Regions {
			if component := componentByRegion[region]; component >= 0 {
				participants = append(participants, component)
			}
		}
		if len(participants) != 2 || participants[0] == participants[1] {
			return nil, geom.ErrConvergence
		}
		p := arrangement.Vertices[node.Vertex]
		contact := TwoPolynomialNodeComponentContact{Parameters: p.Parameters, Point: p.Point,
			Components: [2]int{min(participants[0], participants[1]), max(participants[0], participants[1])}}
		wantPair := [2]int{0, 1}
		if between {
			wantPair = [2]int{i, i + 1}
		}
		if contact.Components != wantPair {
			return nil, geom.ErrConvergence
		}
		for j, component := range contact.Components {
			contact.Vertices[j], e = polynomialCutContactVertex(r.components[component], histories[component], xyz[node.Vertex])
			if e != nil {
				return nil, e
			}
		}
		r.contacts[i] = contact
	}
	if e = r.validateContactIdentities(); e != nil {
		return nil, e
	}
	return r, nil
}

func (r *TwoPolynomialNodeCut) validateContactIdentities() error {
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
