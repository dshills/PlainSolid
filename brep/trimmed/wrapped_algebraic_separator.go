package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// WrappedCoordinateSeparator proposes one common world coordinate plane and
// an exact affine coordinate formula on each ORIGINAL source chart.
// Model[Axis] = Scale[owner]*UV[ParameterAxes[owner]] + Offset[owner].
// The selected parameter axes must be unwrapped; surface maps may otherwise
// differ, including distinct domains, knot bases and original periodic lifts.
type WrappedCoordinateSeparator struct {
	Axis          int
	Level         *big.Rat
	ParameterAxes [2]int
	Scale, Offset [2]*big.Rat
}

// WrappedAlgebraicSeparatedPair proves the two individually embedded original
// faces intersect exactly along their declared whole mate and have disjoint
// interiors. The existing native indexed shell is an embedded open disk.
// It is not a material union or closed solid certificate.
type WrappedAlgebraicSeparatedPair struct {
	shell      *WrappedAlgebraicOpenShell
	separator  WrappedCoordinateSeparator
	sides      [2]int
	operations int
}

func (p *WrappedAlgebraicSeparatedPair) Shell() *WrappedAlgebraicOpenShell {
	if p == nil {
		return nil
	}
	return p.shell
}
func (p *WrappedAlgebraicSeparatedPair) Sides() [2]int {
	if p == nil {
		return [2]int{}
	}
	return p.sides
}
func (p *WrappedAlgebraicSeparatedPair) ExactOperations() int {
	if p == nil {
		return 0
	}
	return p.operations
}
func (p *WrappedAlgebraicSeparatedPair) Separator() WrappedCoordinateSeparator {
	if p == nil {
		return WrappedCoordinateSeparator{}
	}
	q := p.separator
	q.Level = new(big.Rat).Set(q.Level)
	for i := 0; i < 2; i++ {
		q.Scale[i] = new(big.Rat).Set(q.Scale[i])
		q.Offset[i] = new(big.Rat).Set(q.Offset[i])
	}
	return q
}

// CertifyWrappedAlgebraicSeparatedPair verifies homogeneous coefficient
// identities on both complete original surface charts and strict signed trim
// Bernstein controls. Each boundary is in its own closed halfspace, and only
// the declared mate (including its two endpoints) lies in the plane. Positivity
// on the other edges and the affine-coordinate maximum principle imply every
// face interior is strictly on that side. Opposite sides plus the already
// complete model mate prove global adjacency without chart-map equality.
// All arithmetic work across both surfaces and loops shares one allowance.
func CertifyWrappedAlgebraicSeparatedPair(ctx context.Context, s *WrappedAlgebraicOpenShell, q WrappedCoordinateSeparator, o geom.SurfaceAffineCoordinateOptions) (*WrappedAlgebraicSeparatedPair, error) {
	if ctx == nil || s == nil || s.pair == nil || q.Axis < 0 || q.Axis > 2 || q.Level == nil {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 16000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return nil, geom.ErrInvalid
	}
	if e := wrappedShellSourceBinding(ctx, s); e != nil {
		return nil, e
	}
	used := 0
	check := func(v *big.Rat) error {
		used++
		if v == nil || used > o.MaxOperations || v.Num().BitLen() > o.MaxCoefficientBits || v.Denom().BitLen() > o.MaxCoefficientBits {
			return geom.ErrConvergence
		}
		return ctx.Err()
	}
	if e := check(q.Level); e != nil {
		return nil, e
	}
	out := &WrappedAlgebraicSeparatedPair{shell: s, separator: q}
	for owner := 0; owner < 2; owner++ {
		axis := q.ParameterAxes[owner]
		if axis < 0 || axis > 1 || q.Scale[owner] == nil || q.Offset[owner] == nil || q.Scale[owner].Sign() == 0 {
			return nil, geom.ErrInvalid
		}
		for _, v := range []*big.Rat{q.Scale[owner], q.Offset[owner]} {
			if e := check(v); e != nil {
				return nil, e
			}
		}
		source := s.sources[owner]
		w, ok := source.NativeFace().Face.Surface.(*geom.WrappedSurfacePatch)
		if !ok || w.Periods()[axis] != nil {
			return nil, geom.ErrConvergence
		}
		if used >= o.MaxOperations {
			return nil, geom.ErrConvergence
		}
		opts := o
		opts.MaxOperations -= used
		proof, e := geom.CertifySurfaceAffineCoordinate(ctx, w.FundamentalPatch(), q.Axis, axis, q.Scale[owner], q.Offset[owner], opts)
		if e != nil {
			return nil, e
		}
		used += proof.ExactOperations
		if !proof.Proven {
			return nil, geom.ErrConvergence
		}
		mate, _ := s.pair.Owner(owner)
		levelOffset := new(big.Rat).Sub(q.Offset[owner], q.Level)
		if e := check(levelOffset); e != nil {
			return nil, e
		}
		for ei, curve := range source.face.loops[0] {
			h := curve.HomogeneousControls()
			var signs [3]int
			for i, row := range h {
				for _, v := range row {
					if e := check(v); e != nil {
						return nil, e
					}
				}
				r := new(big.Rat).Add(new(big.Rat).Mul(q.Scale[owner], row[axis]), new(big.Rat).Mul(levelOffset, row[2]))
				if e := check(r); e != nil {
					return nil, e
				}
				signs[i] = r.Sign()
			}
			if ei == mate.Edge {
				if signs != ([3]int{}) {
					return nil, geom.ErrDomain
				}
				continue
			}
			nonzero := false
			for _, sign := range signs {
				if sign == 0 {
					continue
				}
				nonzero = true
				if out.sides[owner] == 0 {
					out.sides[owner] = sign
				} else if out.sides[owner] != sign {
					return nil, geom.ErrConvergence
				}
			}
			if !nonzero {
				return nil, geom.ErrConvergence
			}
			edge := source.face.native.Edges[ei]
			for side, vertex := range []int{edge.Start, edge.End} {
				if signs[2*side] == 0 && vertex != mate.Vertices[0] && vertex != mate.Vertices[1] {
					return nil, geom.ErrConvergence
				}
			}
		}
		if out.sides[owner] == 0 {
			return nil, geom.ErrConvergence
		}
	}
	if out.sides[0] != -out.sides[1] {
		return nil, geom.ErrConvergence
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	out.separator = out.Separator()
	out.operations = used
	return out, nil
}
