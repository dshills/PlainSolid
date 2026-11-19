//go:build research

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
	parts, e := s.NativeTubeLensComponents(ctx, tolerance, o)
	if e != nil {
		return Solid{}, e
	}
	if len(parts) != 1 {
		return Solid{}, geom.ErrConvergence
	}
	return parts[0], nil
}

// NativeTubeLensComponents converts each retained two-face tube lens into a
// native Solid. Each component must have one complete closed section, with
// no holes or original shell edges. Components remain separate, in original
// section branch order. Any unqualified component rejects the entire result.
func (s AlgebraicShellBoolean) NativeTubeLensComponents(ctx context.Context, tolerance float64, o edgecheck.Options) ([]Solid, error) {
	if ctx == nil || s.source == nil || tolerance <= 0 || !finite(tolerance) {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if len(s.faces) == 0 {
		return nil, nil
	}
	owners := make([][2]int, len(s.source.branches))
	for i := range owners {
		owners[i] = [2]int{-1, -1}
	}
	for i, f := range s.faces {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		if len(f.Outer) == 0 || len(f.Holes) != 0 {
			return nil, geom.ErrConvergence
		}
		edge := s.index[f.Outer[0].Edge]
		if edge.OriginalSubdivisionEdge >= 0 || edge.SectionPiece < 0 {
			return nil, geom.ErrConvergence
		}
		branch := s.parts[0].Section[edge.SectionPiece].Branch
		if owners[branch][f.Operand] >= 0 {
			return nil, geom.ErrConvergence
		}
		owners[branch][f.Operand] = i
	}
	var result []Solid
	for branch, faces := range owners {
		if faces == [2]int{-1, -1} {
			continue
		}
		if faces[0] < 0 || faces[1] < 0 {
			return nil, geom.ErrConvergence
		}
		part, e := s.nativeTubeLensComponent(ctx, branch, faces, tolerance, o)
		if e != nil {
			return nil, fmt.Errorf("trimmed: section branch %d: %w", branch, e)
		}
		result = append(result, part)
	}
	return result, nil
}

func (s AlgebraicShellBoolean) nativeTubeLensComponent(ctx context.Context, branch int, ids [2]int, tolerance float64, o edgecheck.Options) (Solid, error) {
	b := s.source.branches[branch]
	var f [2]Face
	var uses [2][]AlgebraicShellBooleanUse
	for i, id := range ids {
		src := s.faces[id]
		if src.Operand != i || src.Original != b.Faces[i] || len(src.Holes) != 0 {
			return Solid{}, geom.ErrConvergence
		}
		orig, e := s.OriginalFace(id)
		if e != nil {
			return Solid{}, e
		}
		f[i] = Face{Surface: orig.Surface, Reversed: src.Reversed}
		uses[i] = src.Outer
	}
	ring, e := s.nativeWholeTubeRing(ctx, branch, uses)
	if e != nil {
		return Solid{}, e
	}
	return assembleTubeRegionSolid(ctx, f, []tubeRegionSolidRing{ring}, tolerance, o)
}

// NativeTubeAnnulus converts a Boolean bounded by exactly two original faces
// with one common outer and one common inner complete section. It preserves
// original charts and paired implicit coedges and proves native incidence.
// The Boolean remains authoritative for the exact root coordinates.
func (s AlgebraicShellBoolean) NativeTubeAnnulus(ctx context.Context, tolerance float64, o edgecheck.Options) (Solid, error) {
	if ctx == nil || s.source == nil || tolerance <= 0 || !finite(tolerance) {
		return Solid{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return Solid{}, e
	}
	if len(s.faces) != 2 {
		return Solid{}, geom.ErrConvergence
	}
	var pair [2]int
	var faces [2]Face
	var loops [2][2][]AlgebraicShellBooleanUse // loop, operand
	for i, f := range s.faces {
		if f.Operand != i || len(f.Holes) != 1 {
			return Solid{}, geom.ErrConvergence
		}
		p, e := s.OriginalFace(i)
		if e != nil {
			return Solid{}, e
		}
		pair[i] = f.Original
		faces[i] = Face{Surface: p.Surface, Reversed: f.Reversed}
		loops[0][i], loops[1][i] = f.Outer, f.Holes[0]
	}
	var rings [2]tubeRegionSolidRing
	branches := [2]int{-1, -1}
	for i, loop := range loops {
		if len(loop[0]) < 3 {
			return Solid{}, geom.ErrConvergence
		}
		edge := s.index[loop[0][0].Edge]
		if edge.OriginalSubdivisionEdge >= 0 || edge.SectionPiece < 0 {
			return Solid{}, geom.ErrConvergence
		}
		branch := s.parts[0].Section[edge.SectionPiece].Branch
		if s.source.branches[branch].Faces != pair || (i == 1 && branch == branches[0]) {
			return Solid{}, geom.ErrConvergence
		}
		var e error
		rings[i], e = s.nativeWholeTubeRing(ctx, branch, loop)
		if e != nil {
			return Solid{}, e
		}
		branches[i] = branch
	}
	return assembleTubeRegionSolid(ctx, faces, rings[:], tolerance, o)
}

func (s AlgebraicShellBoolean) nativeWholeTubeRing(ctx context.Context, branch int, uses [2][]AlgebraicShellBooleanUse) (tubeRegionSolidRing, error) {
	return s.nativeWholeTubeRingBounded(ctx, branch, uses, 128)
}

func (s AlgebraicShellBoolean) nativeWholeTubeRingBounded(ctx context.Context, branch int, uses [2][]AlgebraicShellBooleanUse, maxPieces int) (tubeRegionSolidRing, error) {
	ring, e := s.nativeWholeTubeRingOrder(ctx, branch, uses, maxPieces)
	if e != nil {
		return tubeRegionSolidRing{}, e
	}
	b := s.source.branches[branch]
	for i, piece := range b.Segments {
		if e := ctx.Err(); e != nil {
			return tubeRegionSolidRing{}, e
		}
		_, limit, reverse, e := piece.ExactFreeDomain()
		if e != nil {
			return tubeRegionSolidRing{}, e
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
				return tubeRegionSolidRing{}, fmt.Errorf("trimmed: section segment %d endpoint %d: %w", i, j, e)
			}
		}
		ring.cut = append(ring.cut, x)
	}
	return ring, nil
}

// nativeWholeTubeRingOrder rechecks original closed-wire incidence without
// constructing endpoint representatives or native curves.
func (s AlgebraicShellBoolean) nativeWholeTubeRingOrder(ctx context.Context, branch int, uses [2][]AlgebraicShellBooleanUse, maxPieces int) (tubeRegionSolidRing, error) {
	if ctx == nil || s.source == nil || branch < 0 || branch >= len(s.source.branches) {
		return tubeRegionSolidRing{}, geom.ErrInvalid
	}
	if maxPieces < 3 || maxPieces > 256 {
		return tubeRegionSolidRing{}, geom.ErrInvalid
	}
	var ring tubeRegionSolidRing
	b := s.source.branches[branch]
	n := len(b.Segments)
	if !b.Closed || n < 3 || n > maxPieces || len(uses[0]) != n || len(uses[1]) != n {
		return ring, geom.ErrConvergence
	}
	for i, row := range uses {
		reverse := row[0].Reversed
		ring.reversed[i] = reverse
		first := -1
		for j, u := range row {
			if e := ctx.Err(); e != nil {
				return tubeRegionSolidRing{}, e
			}
			if u.Edge < 0 || u.Edge >= len(s.index) {
				return tubeRegionSolidRing{}, geom.ErrInvalid
			}
			edge := s.index[u.Edge]
			if edge.OriginalSubdivisionEdge >= 0 || edge.SectionPiece < 0 || edge.SectionPiece >= len(s.parts[0].Section) || u.Reversed != reverse {
				return tubeRegionSolidRing{}, geom.ErrConvergence
			}
			piece := s.parts[0].Section[edge.SectionPiece]
			if j == 0 {
				first = piece.Index
			}
			want := (first + j) % n
			if reverse {
				want = (first + n - j) % n
			}
			if piece.Branch != branch || piece.Reversed || piece.Index < 0 || piece.Index >= n || piece.Index != want {
				return tubeRegionSolidRing{}, geom.ErrConvergence
			}
		}
	}
	return ring, nil
}
