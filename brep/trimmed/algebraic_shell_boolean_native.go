package trimmed

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
)

// NativeTubeLens converts a Boolean whose entire boundary is a two-face
// lens formed by one complete closed tube. Both original face supports and
// both implicit edge trims are retained. The Solid uses represented vertex
// witnesses and the receiver remains authoritative for the exact junctions.
// Use the Boolean JSON codec to persist the original construction and proof.
// Results with source edges, holes, or several section branches are unresolved.
func (s AlgebraicShellBoolean) NativeTubeLens(ctx context.Context, tolerance float64, o edgecheck.Options) (Solid, error) {
	if ctx == nil || s.source == nil || tolerance <= 0 || !finite(tolerance) {
		return Solid{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, e
	}
	if len(s.faces) != 2 || len(s.source.branches) != 1 {
		return Solid{}, geom.ErrConvergence
	}
	b := s.source.branches[0]
	n := len(b.Segments)
	if !b.Closed || n < 3 || n > 128 || len(s.index) != n || len(s.vertices) != n {
		return Solid{}, geom.ErrConvergence
	}
	var f [2]Face
	var ring tubeRegionSolidRing
	for i, src := range s.faces {
		if src.Operand != i || src.Original != b.Faces[i] || len(src.Outer) != n || len(src.Holes) != 0 {
			return Solid{}, geom.ErrConvergence
		}
		orig, e := s.OriginalFace(i)
		if e != nil {
			return Solid{}, e
		}
		f[i] = Face{Surface: orig.Surface, Reversed: src.Reversed}
		reverse := src.Outer[0].Reversed
		ring.reversed[i] = reverse
		first := -1
		for j, u := range src.Outer {
			edge := s.index[u.Edge]
			if edge.OriginalSubdivisionEdge >= 0 || edge.SectionPiece < 0 || u.Reversed != reverse {
				return Solid{}, geom.ErrConvergence
			}
			piece := s.parts[0].Section[edge.SectionPiece]
			if j == 0 {
				first = piece.Index
			}
			want := (first + j) % n
			if reverse {
				want = (first + n - j) % n
			}
			if piece.Branch != 0 || piece.Reversed || piece.Index != want {
				return Solid{}, geom.ErrConvergence
			}
		}
	}
	for i, piece := range b.Segments {
		if e := ctx.Err(); e != nil {
			return Solid{}, e
		}
		_, limit, reverse, e := piece.ExactFreeDomain()
		if e != nil {
			return Solid{}, e
		}
		// The entire canonical section edge is retained. Bind exact roots
		// at its original endpoints before invoking the shared lens builder.
		endpoints := [2]*big.Rat{limit.Lower, limit.Upper}
		if reverse {
			endpoints[0], endpoints[1] = endpoints[1], endpoints[0]
		}
		x := AlgebraicTubeFaceCutPiece{Segment: piece}
		roots := [2]**geom.ExactAlgebraicRoot{&x.FreeStart, &x.FreeEnd}
		for j, end := range endpoints {
			*roots[j], _, e = geom.NewExactRationalRoot(ctx, end, geom.ExactCurvePlaneEventOptions{})
			if e != nil {
				return Solid{}, fmt.Errorf("trimmed: section segment %d endpoint %d: %w", i, j, e)
			}
		}
		ring.cut = append(ring.cut, x)
	}
	return assembleTubeRegionSolid(ctx, f, []tubeRegionSolidRing{ring}, tolerance, o)
}
