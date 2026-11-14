package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// WrappedCylinderUnionTopology is an immutable native open annulus. It has two
// boundary cycles, not a planar exterior/hole pair, and is not a closed Solid.
// ExactComplex retains original owner UV and period lifts. No preexisting
// B-rep edge IDs are claimed by this rectangle-source constructor.
type WrappedCylinderUnionTopology struct {
	graph      *WrappedFaceRegionTopology
	exact      *geom.WrappedCylinderUnionComplex
	operations int
}

func (r *WrappedCylinderUnionTopology) Vertices() []geom.Vec3 {
	if r == nil {
		return nil
	}
	return r.graph.Vertices()
}
func (r *WrappedCylinderUnionTopology) Edges() []parametric.Edge {
	if r == nil {
		return nil
	}
	return r.graph.Edges()
}
func (r *WrappedCylinderUnionTopology) Faces() []Face {
	if r == nil {
		return nil
	}
	return r.graph.Faces()
}
func (r *WrappedCylinderUnionTopology) BoundaryCycles() [][]Use {
	if r == nil {
		return nil
	}
	return r.graph.BoundaryCycles()
}
func (r *WrappedCylinderUnionTopology) ExactComplex() *geom.WrappedCylinderUnionComplex {
	if r == nil {
		return nil
	}
	return r.exact
}

// ExactOperations includes conservative reserved work for continuous native
// edge-use proofs. All constructors and proofs share the caller's quota.
func (r *WrappedCylinderUnionTopology) ExactOperations() int {
	if r == nil {
		return 0
	}
	return r.operations
}

// NewWrappedCylinderUnionTopology re-proves the exact two-overlap annulus and
// converts only losslessly representable model vertices and face UVs. Each
// seam use retains its original canonical 0/period lift. Complete continuous
// model-edge agreement is proved after conversion, including reversed uses.
// Nonrepresentable cuts return ErrUnrepresentable without a partial result;
// the separate geom exact complex has no such representation restriction.
func NewWrappedCylinderUnionTopology(ctx context.Context, a, b *geom.WrappedSurfacePatch, rectA, rectB [2][2]*big.Rat, o WrappedCylinderIntersectionOptions) (*WrappedCylinderUnionTopology, error) {
	if ctx == nil {
		return nil, geom.ErrInvalid
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxPoles == 0 {
		o.MaxPoles = 4096
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxPoles < 4 {
		return nil, geom.ErrInvalid
	}
	relation, comparison, e := geom.CompareWrappedSurfaceCharts(ctx, a, b, o.ExactSurfaceComparisonOptions)
	if e != nil {
		return nil, e
	}
	if relation == nil || !comparison.Complete || !comparison.Equal {
		return nil, geom.ErrConvergence
	}
	used := comparison.ExactOperations
	if used >= o.MaxOperations {
		return nil, geom.ErrConvergence
	}
	exact, e := relation.CertifyCylinderUnionComplex(ctx, rectA, rectB, geom.WrappedCylinderTwoOverlapOptions{WrappedRectangleOverlapOptions: geom.WrappedRectangleOverlapOptions{MaxOperations: o.MaxOperations - used, MaxCoefficientBits: o.MaxCoefficientBits, MaxComponents: 2}, MaxPoles: o.MaxPoles})
	if e != nil {
		return nil, e
	}
	used += exact.ExactOperations()
	graph := &WrappedFaceRegionTopology{}
	for _, v := range exact.Vertices() {
		p, e := geom.RepresentedExactPoint(v.Point)
		if e != nil {
			return nil, e
		}
		graph.vertices = append(graph.vertices, p)
	}
	for _, edge := range exact.Edges() {
		p, e := saddleRepresentedUV(edge.UV[0])
		if e != nil {
			return nil, e
		}
		q, e := saddleRepresentedUV(edge.UV[1])
		if e != nil {
			return nil, e
		}
		graph.edges = append(graph.edges, parametric.Edge{Start: edge.Start, End: edge.End, Curve: surfaceTrimCurve{surface: a, a: p, b: q}, Range: parametric.Interval{Max: 1}})
	}
	nativeUse := func(u geom.CylinderQuotientUse) (Use, error) {
		p, e := saddleRepresentedUV(u.UV[0])
		if e != nil {
			return Use{}, e
		}
		q, e := saddleRepresentedUV(u.UV[1])
		if e != nil {
			return Use{}, e
		}
		return Use{Edge: u.Edge, Reversed: u.Reversed, UVStart: p, UVEnd: q}, nil
	}
	for _, f := range exact.Faces() {
		face := Face{Surface: a}
		for _, u := range f.Uses {
			use, e := nativeUse(u)
			if e != nil {
				return nil, e
			}
			face.Outer = append(face.Outer, use)
		}
		graph.faces = append(graph.faces, face)
	}
	for _, cycle := range exact.BoundaryCycles() {
		var native []Use
		for _, u := range cycle {
			use, e := nativeUse(u)
			if e != nil {
				return nil, e
			}
			native = append(native, use)
		}
		graph.boundaries = append(graph.boundaries, native)
	}
	if used >= o.MaxOperations {
		return nil, geom.ErrConvergence
	}
	self, p, e := geom.CompareWrappedSurfaceCharts(ctx, a, a, geom.ExactSurfaceComparisonOptions{MaxOperations: o.MaxOperations - used, MaxCoefficientBits: o.MaxCoefficientBits, MaxCells: o.MaxCells})
	if e != nil {
		return nil, e
	}
	if self == nil || !p.Complete || !p.Equal {
		return nil, geom.ErrConvergence
	}
	used += p.ExactOperations
	// Reserve a hard per-call quota rather than assuming an unreported work count.
	const lineQuota = 64
	for _, f := range graph.faces {
		for _, u := range f.Outer {
			if o.MaxOperations-used < lineQuota {
				return nil, geom.ErrConvergence
			}
			model, ok := analyticModelLine(graph.edges[u.Edge])
			if !ok {
				return nil, geom.ErrConvergence
			}
			use, ok := analyticUseLine(f.Surface, u)
			if !ok {
				return nil, geom.ErrConvergence
			}
			same, e := self.SameParameterLines(ctx, model.uv, use.uv, geom.SurfacePatchOptions{MaxOperations: lineQuota, MaxCoefficientBits: o.MaxCoefficientBits})
			if e != nil {
				return nil, e
			}
			if !same {
				return nil, geom.ErrConvergence
			}
			used += lineQuota
		}
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	return &WrappedCylinderUnionTopology{graph: graph, exact: exact, operations: used}, nil
}
