package trimmed

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// PolynomialTangencyEnvelopeOptions bounds the positive-base and original
// polynomial tangency proofs for a shared-base Common or Fuse.
type PolynomialTangencyEnvelopeOptions struct {
	Section intersect.SplinePlaneOptions
	Proof   geom.GraphPointContactOptions
}

// PolynomialTangencyEnvelopeHistory maps each original contact restriction,
// vertex and sector to the native boundary. Vertex 4 is the tangency.
// RoofOwners identifies the selected original roof (0=A, 1=B) in sector order.
type PolynomialTangencyEnvelopeHistory struct {
	Edges      [4]int
	Vertices   [5]int
	RoofFaces  [4]int
	RoofOwners [4]int
	Base       int
}

// PolynomialTangencyEnvelope is a source-bound Common or Fuse with one exact
// tangency vertex and four original roof sectors. Its source recipe re-proves
// the exact owner coedges, one-sided sector order and closed vertex link.
type PolynomialTangencyEnvelope struct {
	solid      Solid
	history    PolynomialTangencyEnvelopeHistory
	sources    [3]*geom.BSplineSurface
	parameters [2]*big.Rat
	point      [3]*big.Rat
	axis       int
	op         parametric.BooleanOp
	tolerance  float64
}

// Boundary returns the native manifold boundary.
func (r *PolynomialTangencyEnvelope) Boundary() Solid {
	if r == nil {
		return Solid{}
	}
	return r.solid
}

// History returns the original-to-native identity ledger.
func (r *PolynomialTangencyEnvelope) History() PolynomialTangencyEnvelopeHistory {
	if r == nil {
		return PolynomialTangencyEnvelopeHistory{}
	}
	return r.history
}

// Contact returns caller-owned rational UV and model coordinates.
func (r *PolynomialTangencyEnvelope) Contact() (uv [2]*big.Rat, xyz [3]*big.Rat) {
	if r == nil || r.sources[0] == nil {
		return uv, xyz
	}
	for i, x := range r.parameters {
		uv[i] = new(big.Rat).Set(x)
	}
	for i, x := range r.point {
		xyz[i] = new(big.Rat).Set(x)
	}
	return uv, xyz
}

// CombineSplineGraphPolynomialTangency computes a bounded Common or Fuse over
// the strictly positive shared base. It uses the complete source arrangement
// for the two tangential sector boundaries and verifies every native coedge
// continuously on its original owner. Native UV and XYZ vertices must be exact.
func CombineSplineGraphPolynomialTangency(ctx context.Context, base, a, b *geom.BSplineSurface, axis int, op parametric.BooleanOp, tolerance float64, o PolynomialTangencyEnvelopeOptions) (*PolynomialTangencyEnvelope, error) {
	if ctx == nil || base == nil || a == nil || b == nil || axis < 0 || axis > 1 ||
		op != parametric.Intersection && op != parametric.Union || !finite(tolerance) || tolerance <= 0 ||
		o.Section.Domain != (geom.ParameterRectangle{}) {
		return nil, geom.ErrInvalid
	}
	patches, e := prepareSharedBaseGraphContact(ctx, base, a, b, o.Section)
	if e != nil {
		return nil, e
	}
	proof, e := geom.DiscoverSplineGraphPolynomialTangency(ctx, patches[0], patches[1], axis, o.Proof)
	if e != nil {
		return nil, e
	}
	if !proof.Proven || proof.Domain != (geom.ParameterRectangle{UMax: 1, VMax: 1}) || proof.Graph.ProjectionOrientation != 1 {
		return nil, geom.ErrConvergence
	}
	var uv [5]geom.Vec2
	var xyz [5]geom.Vec3
	for i, vertex := range proof.Vertices {
		uv[i], e = exactGraphPartitionUV(vertex.Parameters)
		if e != nil {
			return nil, e
		}
		xyz[i], e = geom.RepresentedExactPoint(vertex.Point)
		if e != nil {
			return nil, e
		}
	}
	pieces, e := tangencyEnvelopePieces(proof, uv)
	if e != nil {
		return nil, e
	}
	var regions [4]graphEnvelopeRegion
	for i, region := range proof.Regions {
		for _, use := range region.Edges {
			p := pieces[use.Edge]
			if use.Reversed {
				p.a, p.b, p.c = p.b, p.a, reversePCurve(p.c)
			}
			regions[i].loop = append(regions[i].loop, p)
		}
		p := region.Perimeter
		if p == nil || p.ParameterDirection != 1 {
			return nil, geom.ErrConvergence
		}
		arc, e := saddleSquareArc(uv[p.Start], uv[p.End])
		if e != nil {
			return nil, e
		}
		regions[i].loop = append(regions[i].loop, arc...)
		regions[i].first = (region.HeightSide > 0) == (op == parametric.Union)
	}
	solid, envelope, e := assembleSplineEnvelope(ctx, base, a, b, regions[:], tolerance, exactGraphVertexEvaluator(ctx, o.Proof))
	if e != nil {
		return nil, e
	}
	if e = validateExactGraphPartitionEdges(ctx, solid, o.Proof); e != nil {
		return nil, e
	}
	history, e := bindPolynomialTangencyEnvelope(ctx, solid, envelope, proof, [2]*geom.BSplineSurface{a, b}, regions, pieces, xyz)
	if e != nil {
		return nil, e
	}
	return &PolynomialTangencyEnvelope{
		solid: solid, history: history, sources: [3]*geom.BSplineSurface{base, a, b},
		parameters: proof.Vertices[4].Parameters, point: proof.Vertices[4].Point, axis: axis, op: op, tolerance: tolerance,
	}, nil
}

func tangencyEnvelopePieces(proof geom.SplineGraphPolynomialTangency, uv [5]geom.Vec2) ([4]openSectionPiece, error) {
	var pieces [4]openSectionPiece
	for i, edge := range proof.Edges {
		h := make([][3]*big.Rat, len(edge.Controls))
		for j, p := range edge.Controls {
			h[j] = [3]*big.Rat{p[0], p[1], big.NewRat(1, 1)}
		}
		curve, e := NewExactBezierUV(h)
		if e != nil {
			return pieces, e
		}
		if curve.ends != ([2]geom.Vec2{uv[edge.Start], uv[edge.End]}) {
			return pieces, geom.ErrConvergence
		}
		pieces[i] = openSectionPiece{a: curve.ends[0], b: curve.ends[1], c: curve, side: -1}
	}
	return pieces, nil
}

func matchesTangencyTrim(got, want PCurve) bool {
	if want == nil {
		return got == nil
	}
	a, ok := got.(ExactBezierUV)
	c, expected := want.(ExactBezierUV)
	if !ok || !expected || len(a.control) != len(c.control) {
		return false
	}
	for i, p := range a.control {
		for j, x := range p {
			if x.Cmp(c.control[i][j]) != 0 {
				return false
			}
		}
	}
	return true
}

func bindPolynomialTangencyEnvelope(ctx context.Context, solid Solid, envelope SplineGraphBooleanHistory, proof geom.SplineGraphPolynomialTangency, roofs [2]*geom.BSplineSurface, regions [4]graphEnvelopeRegion, pieces [4]openSectionPiece, xyz [5]geom.Vec3) (PolynomialTangencyEnvelopeHistory, error) {
	var h PolynomialTangencyEnvelopeHistory
	topology := solid.Topology()
	if len(topology.Shells) != 1 || topology.EulerCharacteristic != 2 || topology.CollapsedEdges != 0 || len(envelope.ContourEdges) != 4 {
		return h, geom.ErrConvergence
	}
	if e := affineMaterialVertexLinks(ctx, solid); e != nil {
		return h, e
	}
	h.Base = envelope.Base
	// Each source region already has proved complete one-sided order and a
	// simple boundary. Bind every native directed coedge and its exact
	// perimeter arc so the proof describes the published UV region itself.
	for i, region := range regions {
		owner := 1
		if region.first {
			owner = 0
		}
		f := solid.faces[i]
		if f.Surface != roofs[owner] || f.Reversed || len(f.Holes) != 0 || len(f.Outer) != len(region.loop) {
			return h, geom.ErrConvergence
		}
		h.RoofFaces[i], h.RoofOwners[i] = i, owner
		for j, p := range region.loop {
			u := f.Outer[j]
			if u.UVStart != p.a || u.UVEnd != p.b || !matchesTangencyTrim(u.Curve, p.c) {
				return h, fmt.Errorf("trimmed: tangent sector %d coedge %d changed: %w", i, j, geom.ErrConvergence)
			}
		}
	}
	for i := range h.Vertices {
		h.Vertices[i] = -1
	}
	var used [4]bool
	for _, id := range envelope.ContourEdges {
		edge := solid.edges[id]
		m, ok := edge.Curve.(surfaceTrimCurve)
		if !ok || (m.surface != roofs[0] && m.surface != roofs[1]) {
			return h, geom.ErrConvergence
		}
		matched := false
		for i, p := range pieces {
			expected := p.c
			start, end := edge.Start, edge.End
			if m.a == p.b && m.b == p.a {
				expected = reversePCurve(p.c)
				start, end = end, start
			} else if m.a != p.a || m.b != p.b {
				continue
			}
			if used[i] || !matchesTangencyTrim(m.curve, expected) {
				return h, geom.ErrConvergence
			}
			for j, v := range [2]int{proof.Edges[i].Start, proof.Edges[i].End} {
				vertex := [2]int{start, end}[j]
				if solid.vertices[vertex] != xyz[v] || (h.Vertices[v] >= 0 && h.Vertices[v] != vertex) {
					return h, geom.ErrConvergence
				}
				h.Vertices[v] = vertex
			}
			h.Edges[i], used[i], matched = id, true, true
			break
		}
		if !matched {
			return h, geom.ErrConvergence
		}
	}
	for i, region := range proof.Regions {
		for j, coedge := range region.Edges {
			if solid.faces[h.RoofFaces[i]].Outer[j].Edge != h.Edges[coedge.Edge] {
				return h, geom.ErrConvergence
			}
		}
	}
	for i, vertex := range solid.vertices {
		if vertex == xyz[4] && i != h.Vertices[4] {
			return h, geom.ErrConvergence
		}
	}
	degree := 0
	for _, e := range solid.edges {
		if e.Start == h.Vertices[4] || e.End == h.Vertices[4] {
			degree++
		}
	}
	if degree != 4 {
		return h, geom.ErrConvergence
	}
	return h, ctx.Err()
}
