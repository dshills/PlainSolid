package trimmed

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// PolynomialMixedEnvelopeOptions bounds the positive-base and original graph
// proofs for a shared-base Common or Fuse with mixed contacts.
type PolynomialMixedEnvelopeOptions = PolynomialTangencyEnvelopeOptions

// PolynomialMixedEnvelopeHistory maps the original branch pieces, vertices and
// signed roof regions to native topology. Edges are in increasing free order,
// with -1 in the second slot when a branch piece needs only one native edge.
// Midpoints indexes the two subdivided sides of the interior lens by branch.
type PolynomialMixedEnvelopeHistory struct {
	Edges      [6][2]int
	Vertices   [6]int
	Midpoints  [2]int
	RoofFaces  [5]int
	RoofOwners [5]int // 0 for A, 1 for B
	Base       int
}

// PolynomialMixedEnvelopeContact binds the exact contact and its order to the
// native vertex link. Order is one at the crossing and two at the tangency.
// DerivativeGap is the branch difference derivative of that order. Rays are
// native edge IDs, with Sectors giving the roof face between successive rays.
type PolynomialMixedEnvelopeContact struct {
	Parameters    [2]*big.Rat
	Point         [3]*big.Rat
	Order         int
	DerivativeGap *big.Rat
	Vertex        int
	Rays, Sectors [4]int
}

// PolynomialMixedEnvelope is a source-backed Common or Fuse with one
// transverse and one tangent native junction on the two original roofs.
type PolynomialMixedEnvelope struct {
	solid     Solid
	history   PolynomialMixedEnvelopeHistory
	contacts  [2]PolynomialMixedEnvelopeContact
	sources   [3]*geom.BSplineSurface
	axis      int
	op        parametric.BooleanOp
	tolerance float64
}

// Boundary returns the native manifold boundary.
func (r *PolynomialMixedEnvelope) Boundary() Solid {
	if r == nil {
		return Solid{}
	}
	return r.solid
}

// History returns the original-to-native topology ledger.
func (r *PolynomialMixedEnvelope) History() PolynomialMixedEnvelopeHistory {
	if r == nil {
		return PolynomialMixedEnvelopeHistory{}
	}
	return r.history
}

// Contacts returns both native junctions in increasing free-coordinate order.
// The exact rational coordinates and derivative gaps are caller-owned copies.
func (r *PolynomialMixedEnvelope) Contacts() [2]PolynomialMixedEnvelopeContact {
	if r == nil || r.sources[0] == nil {
		return [2]PolynomialMixedEnvelopeContact{}
	}
	out := r.contacts
	for i := range out {
		out[i] = out[i].copy()
	}
	return out
}

func (c PolynomialMixedEnvelopeContact) copy() PolynomialMixedEnvelopeContact {
	for i, x := range c.Parameters {
		c.Parameters[i] = new(big.Rat).Set(x)
	}
	for i, x := range c.Point {
		c.Point[i] = new(big.Rat).Set(x)
	}
	c.DerivativeGap = new(big.Rat).Set(c.DerivativeGap)
	return c
}

// CombineSplineGraphPolynomialMixedContact constructs a bounded Common or Fuse
// from the exact source sectors, checking both original owner coedges and the
// distinct transverse and tangent vertex links. The original charts must be
// identity XY and every native corner and subdivision point must be exact.
func CombineSplineGraphPolynomialMixedContact(ctx context.Context, base, a, b *geom.BSplineSurface, axis int, op parametric.BooleanOp, tolerance float64, o PolynomialMixedEnvelopeOptions) (*PolynomialMixedEnvelope, error) {
	if ctx == nil || base == nil || a == nil || b == nil || axis < 0 || axis > 1 ||
		(op != parametric.Intersection && op != parametric.Union) || !finite(tolerance) || tolerance <= 0 ||
		o.Section.Domain != (geom.ParameterRectangle{}) {
		return nil, geom.ErrInvalid
	}
	patches, e := prepareSharedBaseGraphContact(ctx, base, a, b, o.Section)
	if e != nil {
		return nil, e
	}
	proof, e := geom.DiscoverSplineGraphPolynomialMixedContact(ctx, patches[0], patches[1], axis, o.Proof)
	if e != nil {
		return nil, e
	}
	if !proof.Proven || proof.Domain != (geom.ParameterRectangle{UMax: 1, VMax: 1}) || proof.Graph.ProjectionOrientation != 1 {
		return nil, geom.ErrConvergence
	}
	uv, xyz, e := proof.RepresentedVertices()
	if e != nil {
		return nil, e
	}
	pieces, e := polynomialNodePieces(proof.Edges[:], uv[:])
	if e != nil {
		return nil, e
	}
	loops, e := polynomialNodeRegionLoops(proof.Regions[:], uv[:], pieces)
	if e != nil {
		return nil, e
	}
	regions := make([]graphEnvelopeRegion, len(loops))
	for i, loop := range loops {
		regions[i] = graphEnvelopeRegion{loop: loop, first: (proof.Regions[i].HeightSide > 0) == (op == parametric.Union)}
	}
	solid, envelope, e := assembleSplineEnvelope(ctx, base, a, b, regions, tolerance, exactGraphVertexEvaluator(ctx, o.Proof))
	if e != nil {
		return nil, e
	}
	if e = validateExactGraphPartitionEdges(ctx, solid, o.Proof); e != nil {
		return nil, e
	}
	if e = affineMaterialVertexLinks(ctx, solid); e != nil {
		return nil, e
	}
	result := &PolynomialMixedEnvelope{
		solid: solid, sources: [3]*geom.BSplineSurface{base, a, b}, axis: axis, op: op, tolerance: tolerance,
	}
	if e = result.bind(proof, xyz, pieces, loops, envelope); e != nil {
		return nil, e
	}
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	return result, nil
}

func (r *PolynomialMixedEnvelope) bind(proof geom.SplineGraphPolynomialMixedContact, xyz [6]geom.Vec3, pieces, loops [][]openSectionPiece, envelope SplineGraphBooleanHistory) error {
	binding, e := bindPolynomialEnvelopeSources(r.solid, r.sources, r.op, proof.Edges[:], proof.Regions[:], xyz[:], pieces, loops, envelope)
	if e != nil {
		return e
	}
	r.history = PolynomialMixedEnvelopeHistory{
		Base: envelope.Base, Vertices: [6]int(binding.vertices), Edges: [6][2]int(binding.chains),
		Midpoints: [2]int{binding.midpoints[1], binding.midpoints[4]},
		RoofFaces: [5]int(binding.roofFaces), RoofOwners: [5]int(binding.roofOwners),
	}
	for i, node := range proof.Nodes {
		c, e := bindPolynomialEnvelopeJunction(r.solid, binding, proof.Edges[:], node, proof.Vertices[node.Vertex], xyz[node.Vertex])
		if e != nil {
			return e
		}
		c.Order, c.DerivativeGap = proof.ContactOrder[i], proof.DerivativeGap[i]
		r.contacts[i] = c
	}
	return nil
}

type polynomialEnvelopeBinding struct {
	polynomialNodeBinding
	chains     [][2]int
	roofFaces  []int
	roofOwners []int
}

func bindPolynomialEnvelopeSources(s Solid, sources [3]*geom.BSplineSurface, op parametric.BooleanOp, edges []geom.SplineGraphPolynomialPiece, regions []geom.SplineGraphPolynomialRegion,
	xyz []geom.Vec3, pieces, loops [][]openSectionPiece, envelope SplineGraphBooleanHistory) (polynomialEnvelopeBinding, error) {
	var out polynomialEnvelopeBinding
	topology := s.Topology()
	if len(topology.Shells) != 1 || topology.EulerCharacteristic != 2 || topology.CollapsedEdges != 0 ||
		envelope.Base != len(regions) || len(s.faces) <= envelope.Base || s.faces[envelope.Base].Surface != sources[0] {
		return out, geom.ErrConvergence
	}
	binding, e := bindPolynomialNodeEdges(s, envelope.ContourEdges, edges, xyz, pieces)
	if e != nil {
		return out, e
	}
	out.polynomialNodeBinding = binding
	for i, chain := range binding.edges {
		encoded := [2]int{chain[0], -1}
		if len(chain) == 2 {
			encoded[1] = chain[1]
		}
		out.chains = append(out.chains, encoded)
		for j, id := range chain {
			model := s.edges[id].Curve.(surfaceTrimCurve)
			piece := pieces[i][j]
			expected := piece.c
			if model.a == piece.b {
				expected = reversePCurve(expected)
			}
			if (model.surface != sources[1] && model.surface != sources[2]) || !matchesTangencyTrim(model.curve, expected) {
				return polynomialEnvelopeBinding{}, geom.ErrConvergence
			}
		}
	}

	// Bind both directed coedges of each original branch restriction
	// to their exact source sector loops and selected roofs.
	for i, region := range regions {
		owner := 1
		if (region.HeightSide > 0) == (op == parametric.Union) {
			owner = 0
		}
		out.roofFaces = append(out.roofFaces, i)
		out.roofOwners = append(out.roofOwners, owner)
		face := s.faces[i]
		if e := bindPolynomialTangencyFace(face, sources[1+owner], false, loops[i]); e != nil {
			return polynomialEnvelopeBinding{}, fmt.Errorf("trimmed: polynomial roof region %d: %w", i, e)
		}
		offset := 0
		for _, source := range region.Edges {
			chain := binding.edges[source.Edge]
			for j := range chain {
				index := j
				if source.Reversed {
					index = len(chain) - 1 - j
				}
				if face.Outer[offset+j].Edge != chain[index] {
					return polynomialEnvelopeBinding{}, geom.ErrConvergence
				}
			}
			offset += len(chain)
		}
	}
	return out, nil
}

func bindPolynomialEnvelopeJunction(s Solid, binding polynomialEnvelopeBinding, edges []geom.SplineGraphPolynomialPiece, node geom.SplineGraphPolynomialJunction,
	point geom.SplineGraphPolynomialVertex, xyz geom.Vec3) (PolynomialMixedEnvelopeContact, error) {
	c := PolynomialMixedEnvelopeContact{Parameters: point.Parameters, Point: point.Point, Vertex: binding.vertices[node.Vertex]}
	seen := make(map[int]bool, len(node.RayCycle))
	for j, source := range node.RayCycle {
		chain := binding.edges[source]
		id := chain[0]
		if edges[source].End == node.Vertex {
			id = chain[len(chain)-1]
		}
		edge := s.edges[id]
		if seen[id] || (edge.Start == c.Vertex) == (edge.End == c.Vertex) {
			return PolynomialMixedEnvelopeContact{}, geom.ErrConvergence
		}
		seen[id] = true
		c.Rays[j], c.Sectors[j] = id, binding.roofFaces[node.Regions[j]]
	}
	for id, edge := range s.edges {
		if (edge.Start == c.Vertex || edge.End == c.Vertex) && !seen[id] {
			return PolynomialMixedEnvelopeContact{}, geom.ErrConvergence
		}
	}
	for id, point := range s.vertices {
		if point == xyz && id != c.Vertex {
			return PolynomialMixedEnvelopeContact{}, geom.ErrConvergence
		}
	}
	return c, nil
}
