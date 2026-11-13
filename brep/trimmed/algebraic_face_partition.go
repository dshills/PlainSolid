package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

type AlgebraicFaceFragmentUse struct {
	BoundaryEdge int // -1 is the new cut
	Reversed     bool
}

// AlgebraicFacePartition is a certified two-fragment arrangement on one
// original chart. The new seam is the segment of Line between the exact
// algebraic UV images at Vertices. The two fragments traverse it oppositely.
// No rounded UV endpoint determines incidence or intersection membership.
type AlgebraicFacePartition struct {
	Face          int
	Vertices      [2]int
	Line          [3]*big.Rat // A*u+B*v+C=0
	CutUV         [2]geom.AlgebraicCurveEndpointBounds
	Fragments     [2][]AlgebraicFaceFragmentUse
	FragmentSigns [2]int // strict signs of A*u+B*v+C in the fragment interiors
	HoleFragments [2][][]AlgebraicSubdivisionUse
	terminal      [2]*geom.ExactComposedPlaneEvent
	parameterAxis int
}

// Traversal reports original boundary uses in loop order. The final entry
// of each fragment is the shared generated cut; the returned slices are copies.
func (p AlgebraicFacePartition) Traversal() [2][]AlgebraicFaceFragmentUse {
	return [2][]AlgebraicFaceFragmentUse{append([]AlgebraicFaceFragmentUse(nil), p.Fragments[0]...), append([]AlgebraicFaceFragmentUse(nil), p.Fragments[1]...)}
}

// PartitionFaceByAlgebraicUVLine proves a complete straight chart cut between
// two existing algebraic subdivision vertices. The source outer loop must
// have certified simple topology and straight or positive rational quadratic
// sides. Exactly two simple interior outer-boundary contacts are required.
// Holes strictly separated from the line remain intact in their fragment.
// A hole contact, tangency, corner or coincident edge remains unresolved.
func (s AlgebraicSubdivision) PartitionFaceByAlgebraicUVLine(ctx context.Context, face int, line [3]*big.Rat,
	o AlgebraicSeamOptions) (AlgebraicFacePartition, error) {
	var empty AlgebraicFacePartition
	if ctx == nil || face < 0 || face >= len(s.faces) || line[0] == nil || line[1] == nil || line[2] == nil || line[0].Sign() == 0 && line[1].Sign() == 0 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	f := s.source.faces[face]
	if len(f.Holes) > 16 || len(f.Outer) > 128 {
		return empty, geom.ErrConvergence
	}
	proof, e := ValidateFaceTrimLoops(ctx, f, trimvalidate.Options{})
	if e != nil {
		return empty, e
	}
	if proof.Status != trimvalidate.Certified {
		return empty, geom.ErrConvergence
	}
	lim := o.Root.ExactCurvePlaneEventOptions
	if lim.MaxOperations == 0 {
		lim.MaxOperations = 8000000
	}
	if lim.MaxSubdivisions == 0 {
		lim.MaxSubdivisions = 8192
	}
	if lim.MaxOperations < 1 || lim.MaxSubdivisions < 1 {
		return empty, geom.ErrInvalid
	}
	out := AlgebraicFacePartition{Face: face}
	for i, p := range line {
		out.Line[i] = new(big.Rat).Set(p)
	}
	type terminal struct {
		vertex int
		uv     geom.AlgebraicCurveEndpointBounds
		event  *geom.ExactComposedPlaneEvent
	}
	var cuts []terminal
	for _, u := range f.Outer {
		if e := ctx.Err(); e != nil {
			return empty, e
		}
		h, e := partitionQuadraticControls(u)
		if e != nil {
			return empty, e
		}
		if u.Reversed {
			h[0], h[2] = h[2], h[0]
		}
		uv, e := exactQuadraticTrimParameterCurve(ctx, h, o.Composition.SurfacePatchOptions)
		if e != nil {
			return empty, e
		}
		cell, e := geom.NewExactComposedBezierCell(ctx, uv, o.Composition.SurfacePatchOptions)
		if e != nil {
			return empty, e
		}
		if lim.MaxOperations < 1 || lim.MaxSubdivisions < 1 {
			return empty, geom.ErrConvergence
		}
		r, e := cell.IntersectRawPlane(ctx, [4]*big.Rat{line[0], line[1], nil, line[2]}, lim)
		lim.MaxOperations -= r.Operations
		lim.MaxSubdivisions -= r.Subdivisions
		if e != nil {
			return empty, e
		}
		if !r.Complete || r.Coincident {
			return empty, geom.ErrConvergence
		}
		for _, ev := range r.Events {
			if len(cuts) == 2 || ev.EndpointMask() != 0 || ev.Multiplicity() != 1 {
				return empty, geom.ErrConvergence
			}
			vertex := -1
			for _, sub := range s.faces[face].Outer {
				piece := s.edges[sub.Edge]
				if piece.Original != u.Edge || piece.Upper == nil {
					continue
				}
				if lim.MaxOperations < 1 || lim.MaxSubdivisions < 1 {
					return empty, geom.ErrConvergence
				}
				cmp, e := ev.Root().Compare(ctx, piece.Upper, lim)
				lim.MaxOperations -= cmp.Operations
				lim.MaxSubdivisions -= cmp.Subdivisions
				if e != nil {
					return empty, e
				}
				if !cmp.Proven {
					return empty, geom.ErrConvergence
				}
				if cmp.Order == 0 {
					if vertex >= 0 && vertex != piece.End {
						return empty, geom.ErrConvergence
					}
					vertex = piece.End
				}
			}
			if vertex < 0 {
				return empty, geom.ErrConvergence
			}
			if lim.MaxOperations < 1 || lim.MaxSubdivisions < 1 {
				return empty, geom.ErrConvergence
			}
			options := o.Enclosure
			options.ExactCurvePlaneEventOptions = lim
			b, e := ev.Enclose(ctx, options)
			lim.MaxOperations -= b.ExactOperations
			lim.MaxSubdivisions -= b.Subdivisions
			if e != nil {
				return empty, e
			}
			if !b.Converged {
				return empty, geom.ErrConvergence
			}
			cuts = append(cuts, terminal{vertex, b, ev})
		}
	}
	if len(cuts) != 2 || cuts[0].vertex == cuts[1].vertex {
		return empty, geom.ErrConvergence
	}
	axis := 0
	if line[1].Sign() == 0 {
		axis = 1
	}
	out.parameterAxis = axis
	a, b := cuts[0].uv.Coordinates[axis], cuts[1].uv.Coordinates[axis]
	if b.Upper.Cmp(a.Lower) < 0 {
		cuts[0], cuts[1] = cuts[1], cuts[0]
	} else if a.Upper.Cmp(b.Lower) >= 0 {
		return empty, geom.ErrConvergence
	}
	ring := s.faces[face].Outer
	idx := [2]int{-1, -1}
	for i, u := range ring {
		x := s.edges[u.Edge]
		start := x.Start
		if u.Reversed {
			start = x.End
		}
		for k, t := range cuts {
			if start == t.vertex {
				idx[k] = i
			}
		}
	}
	if idx[0] < 0 || idx[1] < 0 {
		return empty, geom.ErrConvergence
	}
	for i, c := range cuts {
		out.Vertices[i], out.CutUV[i] = c.vertex, c.uv
		out.terminal[i] = c.event
	}
	for k := range 2 {
		sign := 0
		for i := idx[k]; i != idx[k^1]; i = (i + 1) % len(ring) {
			u := ring[i]
			out.Fragments[k] = append(out.Fragments[k], AlgebraicFaceFragmentUse{u.Edge, u.Reversed})
			x := s.edges[u.Edge]
			start := x.Start
			if u.Reversed {
				start = x.End
			}
			if s.vertices[start].Original < 0 {
				continue
			}
			original := f.Outer[u.OriginalUse]
			h, e := partitionQuadraticControls(original)
			if e != nil {
				return empty, e
			}
			value := new(big.Rat).Add(new(big.Rat).Mul(line[0], h[0][0]), new(big.Rat).Mul(line[1], h[0][1]))
			value.Add(value, new(big.Rat).Mul(line[2], h[0][2]))
			if value.Sign() == 0 || sign != 0 && sign != value.Sign() {
				return empty, geom.ErrConvergence
			}
			sign = value.Sign()
		}
		if len(out.Fragments[k]) < 2 || sign == 0 {
			return empty, geom.ErrConvergence
		}
		out.FragmentSigns[k] = sign
		out.Fragments[k] = append(out.Fragments[k], AlgebraicFaceFragmentUse{-1, k == 0})
	}
	if out.FragmentSigns[0] == out.FragmentSigns[1] {
		return empty, geom.ErrConvergence
	}
	for holeIndex, hole := range f.Holes {
		if e := ctx.Err(); e != nil {
			return empty, e
		}
		if len(hole) > 128 {
			return empty, geom.ErrConvergence
		}
		sign := 0
		for _, u := range hole {
			h, e := partitionQuadraticControls(u)
			if e != nil {
				return empty, e
			}
			uv, e := exactQuadraticTrimParameterCurve(ctx, h, o.Composition.SurfacePatchOptions)
			if e != nil {
				return empty, e
			}
			cell, e := geom.NewExactComposedBezierCell(ctx, uv, o.Composition.SurfacePatchOptions)
			if e != nil {
				return empty, e
			}
			if lim.MaxOperations < 1 || lim.MaxSubdivisions < 1 {
				return empty, geom.ErrConvergence
			}
			r, e := cell.IntersectRawPlane(ctx, [4]*big.Rat{line[0], line[1], nil, line[2]}, lim)
			lim.MaxOperations -= r.Operations
			lim.MaxSubdivisions -= r.Subdivisions
			if e != nil {
				return empty, e
			}
			if !r.Complete || r.Coincident || len(r.Events) > 0 {
				return empty, geom.ErrConvergence
			}
			v := new(big.Rat).Add(new(big.Rat).Mul(line[0], h[0][0]), new(big.Rat).Mul(line[1], h[0][1]))
			v.Add(v, new(big.Rat).Mul(line[2], h[0][2]))
			if v.Sign() == 0 || sign != 0 && sign != v.Sign() {
				return empty, geom.ErrConvergence
			}
			sign = v.Sign()
		}
		owner := 0
		if sign == out.FragmentSigns[1] {
			owner = 1
		} else if sign != out.FragmentSigns[0] {
			return empty, geom.ErrConvergence
		}
		out.HoleFragments[owner] = append(out.HoleFragments[owner], append([]AlgebraicSubdivisionUse(nil), s.faces[face].Holes[holeIndex]...))
	}
	return out, nil
}

func partitionQuadraticControls(u Use) ([3][3]*big.Rat, error) {
	switch q := u.Curve.(type) {
	case ExactQuadraticUV:
		return q.HomogeneousControls(), nil
	case SplineUV:
		return quadraticTrimSourceControls(q)
	case nil, QuadraticUV:
		a, b := u.UVStart, u.UVEnd
		m := [2]*big.Rat{new(big.Rat).Quo(new(big.Rat).Add(ratInset(a.X), ratInset(b.X)), big.NewRat(2, 1)),
			new(big.Rat).Quo(new(big.Rat).Add(ratInset(a.Y), ratInset(b.Y)), big.NewRat(2, 1))}
		if q, ok := q.(QuadraticUV); ok {
			m = [2]*big.Rat{ratInset(q.Control.X), ratInset(q.Control.Y)}
		}
		return [3][3]*big.Rat{{ratInset(a.X), ratInset(a.Y), big.NewRat(1, 1)}, {m[0], m[1], big.NewRat(1, 1)},
			{ratInset(b.X), ratInset(b.Y), big.NewRat(1, 1)}}, nil
	default:
		return [3][3]*big.Rat{}, geom.ErrConvergence
	}
}
