package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// WrappedAlgebraicEdgeOwner records indices in one original native open face.
// UV endpoints and vertex order are authored, before any pairing reversal.
type WrappedAlgebraicEdgeOwner struct {
	Loop, Edge, NativeEdge int
	Vertices               [2]int
	UV                     [2][2]*big.Rat
	FaceReversed           bool
}

// WrappedAlgebraicEdgePair proves one complete continuous native edge mate
// with opposite induced coedge orientations. It preserves two owner charts,
// including distinct original periodic occurrences. This local incidence proof
// does not prove the faces otherwise disjoint, a manifold fan, or a closed shell.
// Constructor results are immutable; getters copy rational coordinate data.
type WrappedAlgebraicEdgePair struct {
	owners       [2]WrappedAlgebraicEdgeOwner
	compositions [2]*geom.WrappedCurveCells
	points       [2][3]*big.Rat
	vertexPairs  [2][2]int
	reversed     bool
	operations   int
}

func (p *WrappedAlgebraicEdgePair) Proven() bool {
	return p != nil && p.compositions[0] != nil && p.compositions[1] != nil
}
func (p *WrappedAlgebraicEdgePair) ExactOperations() int {
	if p == nil {
		return 0
	}
	return p.operations
}
func (p *WrappedAlgebraicEdgePair) ReversedParameter() bool { return p != nil && p.reversed }
func (p *WrappedAlgebraicEdgePair) Owner(i int) (WrappedAlgebraicEdgeOwner, error) {
	if !p.Proven() || i < 0 || i > 1 {
		return WrappedAlgebraicEdgeOwner{}, geom.ErrInvalid
	}
	o := p.owners[i]
	for side := 0; side < 2; side++ {
		for axis := 0; axis < 2; axis++ {
			o.UV[side][axis] = new(big.Rat).Set(o.UV[side][axis])
		}
	}
	return o, nil
}
func (p *WrappedAlgebraicEdgePair) Composition(i int) (*geom.WrappedCurveCells, error) {
	if !p.Proven() || i < 0 || i > 1 {
		return nil, geom.ErrInvalid
	}
	return p.compositions[i], nil
}

// Endpoint returns the exact model point in A's authored endpoint order and
// the paired original native vertex indices [A,B]. No new vertex is introduced
// at an irrational seam or original support knot.
func (p *WrappedAlgebraicEdgePair) Endpoint(side int) ([3]*big.Rat, [2]int, error) {
	var z [3]*big.Rat
	if !p.Proven() || side < 0 || side > 1 {
		return z, [2]int{}, geom.ErrInvalid
	}
	for axis := range z {
		z[axis] = new(big.Rat).Set(p.points[side][axis])
	}
	return z, p.vertexPairs[side], nil
}

// CertifyWrappedAlgebraicEdgePair proves B(1-t)=A(t) when reverseB, otherwise
// B(t)=A(t). Selectors are [original loop, original edge], with loop0 outer.
// Both private source faces already carry complete embedding/composition proofs;
// this method independently verifies continuous model identity and endpoint
// binding. Same-direction induced coedges reject even for identical curves.
// MaxWork is aggregate arithmetic, MaxIntervals bounds the common partition.
func CertifyWrappedAlgebraicEdgePair(ctx context.Context, a, b *WrappedAlgebraicQuadraticFace, edgeA, edgeB [2]int, reverseB bool, o edgecheck.Options) (*WrappedAlgebraicEdgePair, error) {
	if ctx == nil || a == nil || b == nil || a.face == nil || b.face == nil {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if o.MaxWork == 0 {
		o.MaxWork = 8000000
	}
	if o.MaxBits == 0 {
		o.MaxBits = 16384
	}
	if o.MaxIntervals == 0 {
		o.MaxIntervals = 4096
	}
	if o.MaxWork < 1 || o.MaxBits < 64 || o.MaxIntervals < 1 || o.MaxIntervals > 8192 {
		return nil, geom.ErrInvalid
	}
	out := &WrappedAlgebraicEdgePair{reversed: reverseB}
	used := 0
	options := func() geom.ExactCurvePlaneEventOptions {
		n := o.MaxWork - used
		if n <= 0 {
			n = -1
		}
		return geom.ExactCurvePlaneEventOptions{MaxOperations: n, MaxCoefficientBits: o.MaxBits}
	}
	var nativePoints [2][2][3]*big.Rat
	for owner, face := range []*WrappedAlgebraicQuadraticFace{a, b} {
		selector := edgeA
		if owner == 1 {
			selector = edgeB
		}
		li, ei := selector[0], selector[1]
		composition, e := face.Composition(li, ei)
		if e != nil {
			return nil, e
		}
		if composition.Len() < 1 || composition.Len() > o.MaxIntervals {
			return nil, geom.ErrConvergence
		}
		original := face.face
		uses := original.native.Face.Outer
		if li > 0 {
			uses = original.native.Face.Holes[li-1]
		}
		use := uses[ei]
		if use.Reversed || use.Edge < 0 || use.Edge >= len(original.native.Edges) {
			return nil, geom.ErrInvalid
		}
		edge := original.native.Edges[use.Edge]
		marker, ok := edge.Curve.(wrappedAlgebraicTrimCurve)
		if !ok || edge.Range != (parametric.Interval{Max: 1}) || marker.surface != original.native.Face.Surface || marker.proof != composition {
			return nil, geom.ErrInvalid
		}
		source := original.loops[li][ei]
		for _, pc := range []PCurve{marker.curve, use.Curve} {
			q, ok := pc.(ExactQuadraticUV)
			if !ok {
				return nil, geom.ErrInvalid
			}
			for i := 0; i < 3; i++ {
				for j := 0; j < 3; j++ {
					used++
					if used > o.MaxWork {
						return nil, geom.ErrConvergence
					}
					if q.control[i][j] == nil || q.control[i][j].Cmp(source.control[i][j]) != 0 {
						return nil, geom.ErrDomain
					}
				}
			}
		}
		uv, represented := source.Endpoints()
		if marker.a != represented[0] || marker.b != represented[1] || use.UVStart != represented[0] || use.UVEnd != represented[1] {
			return nil, geom.ErrDomain
		}
		descriptor := WrappedAlgebraicEdgeOwner{Loop: li, Edge: ei, NativeEdge: use.Edge, Vertices: [2]int{edge.Start, edge.End}, UV: uv, FaceReversed: original.native.Face.Reversed}
		for side, vertex := range descriptor.Vertices {
			if vertex < 0 || vertex >= len(original.native.Vertices) {
				return nil, geom.ErrInvalid
			}
			v := original.native.Vertices[vertex]
			xyz := [3]*big.Rat{ratInset(v.X), ratInset(v.Y), ratInset(v.Z)}
			index := 0
			if side == 1 {
				index = composition.Len() - 1
			}
			piece, e := composition.Piece(index)
			if e != nil {
				return nil, e
			}
			parameter := big.NewRat(int64(side), 1)
			model, e := piece.Model.EvaluateRational(ctx, parameter, options())
			used += model.ExactOperations
			if e != nil {
				return nil, e
			}
			chart, e := piece.UV.EvaluateRational(ctx, parameter, options())
			used += chart.ExactOperations
			if e != nil {
				return nil, e
			}
			for axis := 0; axis < 3; axis++ {
				used++
				if used > o.MaxWork || xyz[axis].Num().BitLen() > o.MaxBits || xyz[axis].Denom().BitLen() > o.MaxBits {
					return nil, geom.ErrConvergence
				}
				if model.Point[axis].Cmp(xyz[axis]) != 0 {
					return nil, geom.ErrDomain
				}
			}
			for axis := 0; axis < 2; axis++ {
				if chart.Point[axis].Cmp(uv[side][axis]) != 0 {
					return nil, geom.ErrDomain
				}
			}
			nativePoints[owner][side] = xyz
		}
		out.owners[owner] = descriptor
		out.compositions[owner] = composition
	}
	comparison, e := out.compositions[0].CompareModelBounded(ctx, out.compositions[1], reverseB, o.MaxIntervals, options())
	used += comparison.ExactOperations
	if e != nil {
		return nil, e
	}
	if !comparison.Proven || !comparison.Equivalent {
		return nil, geom.ErrDomain
	}
	// In A's parameter direction B's oriented tangent has sign
	// (faceB orientation)*(parameter map sign). It must oppose A's coedge.
	if (out.owners[0].FaceReversed == out.owners[1].FaceReversed) != reverseB {
		return nil, geom.ErrDomain
	}
	for side := 0; side < 2; side++ {
		other := side
		if reverseB {
			other = 1 - side
		}
		for axis := 0; axis < 3; axis++ {
			if nativePoints[0][side][axis].Cmp(nativePoints[1][other][axis]) != 0 {
				return nil, geom.ErrDomain
			}
			out.points[side][axis] = new(big.Rat).Set(nativePoints[0][side][axis])
		}
		out.vertexPairs[side] = [2]int{out.owners[0].Vertices[side], out.owners[1].Vertices[other]}
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	out.operations = used
	return out, nil
}
