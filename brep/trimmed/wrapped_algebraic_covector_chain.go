//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// WrappedStrictCovectorFaceSeparation proves two retained native faces lie strictly on
// opposite sides of an independently specified rational world covector plane. Margins
// bound their entire retained faces, including all trim boundaries.
type WrappedStrictCovectorFaceSeparation struct {
	source     [2]*WrappedAlgebraicQuadraticFace
	separator  WrappedCovectorSeparator
	sides      [2]int
	margins    [2]*big.Rat
	operations int
}

func (p *WrappedStrictCovectorFaceSeparation) Sides() [2]int {
	if p == nil {
		return [2]int{}
	}
	return p.sides
}
func (p *WrappedStrictCovectorFaceSeparation) Margins() [2]*big.Rat {
	if p == nil {
		return [2]*big.Rat{}
	}
	return [2]*big.Rat{new(big.Rat).Set(p.margins[0]), new(big.Rat).Set(p.margins[1])}
}
func (p *WrappedStrictCovectorFaceSeparation) Separator() WrappedCovectorSeparator {
	if p == nil {
		return WrappedCovectorSeparator{}
	}
	copy := WrappedAlgebraicCovectorSeparatedPair{separator: p.separator}
	return copy.Separator()
}
func (p *WrappedStrictCovectorFaceSeparation) Source(i int) (*WrappedAlgebraicQuadraticFace, error) {
	if p == nil || i < 0 || i > 1 {
		return nil, geom.ErrInvalid
	}
	return p.source[i], nil
}
func (p *WrappedStrictCovectorFaceSeparation) ExactOperations() int {
	if p == nil {
		return 0
	}
	return p.operations
}

// CertifyWrappedStrictCovectorFaceSeparation requires positive strict homogeneous trim
// control margins, and exact full-chart affine coordinate identities. The
// affine function on the bounded retained UV region lies between its boundary
// extrema; positive weights bound each boundary curve by its control values.
// This sufficient predicate deliberately rejects a plane touching any control
// hull, even if a more involved proof could establish strict separation.
func CertifyWrappedStrictCovectorFaceSeparation(ctx context.Context, a, b *WrappedAlgebraicQuadraticFace, q WrappedCovectorSeparator, o geom.SurfaceAffineCoordinateOptions) (*WrappedStrictCovectorFaceSeparation, error) {
	if ctx == nil || a == nil || b == nil || a.face == nil || b.face == nil || q.Level == nil {
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
	nonzeroNormal := false
	for _, v := range q.Normal {
		if v == nil {
			return nil, geom.ErrInvalid
		}
		if e := check(v); e != nil {
			return nil, e
		}
		nonzeroNormal = nonzeroNormal || v.Sign() != 0
	}
	if !nonzeroNormal {
		return nil, geom.ErrInvalid
	}
	out := &WrappedStrictCovectorFaceSeparation{source: [2]*WrappedAlgebraicQuadraticFace{a, b}, separator: q}
	for owner, f := range out.source {
		axis := q.ParameterAxes[owner]
		if axis < 0 || axis > 1 || q.Scale[owner] == nil || q.Offset[owner] == nil || q.Scale[owner].Sign() == 0 {
			return nil, geom.ErrInvalid
		}
		for _, v := range []*big.Rat{q.Scale[owner], q.Offset[owner]} {
			if e := check(v); e != nil {
				return nil, e
			}
		}
		w, ok := f.NativeFace().Face.Surface.(*geom.WrappedSurfacePatch)
		if !ok || w.Periods()[axis] != nil {
			return nil, geom.ErrConvergence
		}
		if used >= o.MaxOperations {
			return nil, geom.ErrConvergence
		}
		opts := o
		opts.MaxOperations -= used
		proof, e := geom.CertifySurfaceAffineCovector(ctx, w.FundamentalPatch(), q.Normal, axis, q.Scale[owner], q.Offset[owner], opts)
		if e != nil {
			return nil, e
		}
		used += proof.ExactOperations
		if !proof.Proven {
			return nil, geom.ErrConvergence
		}
		offset := new(big.Rat).Sub(q.Offset[owner], q.Level)
		if e := check(offset); e != nil {
			return nil, e
		}
		for _, loop := range f.face.loops {
			for _, curve := range loop {
				for _, h := range curve.HomogeneousControls() {
					for _, v := range h {
						if e := check(v); e != nil {
							return nil, e
						}
					}
					value := new(big.Rat).Add(new(big.Rat).Mul(q.Scale[owner], h[axis]), new(big.Rat).Mul(offset, h[2]))
					if e := check(value); e != nil {
						return nil, e
					}
					sign := value.Sign()
					if sign == 0 {
						return nil, geom.ErrConvergence
					}
					if out.sides[owner] == 0 {
						out.sides[owner] = sign
					} else if out.sides[owner] != sign {
						return nil, geom.ErrConvergence
					}
					margin := new(big.Rat).Abs(new(big.Rat).Quo(value, h[2]))
					if e := check(margin); e != nil {
						return nil, e
					}
					if out.margins[owner] == nil || margin.Cmp(out.margins[owner]) < 0 {
						out.margins[owner] = margin
					}
				}
			}
		}
	}
	if out.sides[0] != -out.sides[1] || out.sides[0] == 0 {
		return nil, geom.ErrConvergence
	}
	out.separator = out.Separator()
	out.operations = used
	return out, nil
}

// WrappedAlgebraicCovectorChain is an embedded indexed three-face open
// disk with independently proved rational covector mate separators. Its
// nonadjacent faces have an independent strict retained-face separator.
// Covectors remain unnormalized; margins are in their original scalar units.
type WrappedAlgebraicCovectorChain struct {
	adjacent   [2]*WrappedAlgebraicCovectorSeparatedPair
	outer      *WrappedStrictCovectorFaceSeparation
	graph      *WrappedAlgebraicFaceChainShell
	operations int
}

func (s *WrappedAlgebraicCovectorChain) Adjacent(i int) (*WrappedAlgebraicCovectorSeparatedPair, error) {
	if s == nil || i < 0 || i > 1 {
		return nil, geom.ErrInvalid
	}
	return s.adjacent[i], nil
}
func (s *WrappedAlgebraicCovectorChain) OuterSeparation() *WrappedStrictCovectorFaceSeparation {
	if s == nil {
		return nil
	}
	return s.outer
}
func (s *WrappedAlgebraicCovectorChain) Vertices() []geom.Vec3 {
	if s == nil {
		return nil
	}
	return s.graph.Vertices()
}
func (s *WrappedAlgebraicCovectorChain) Edges() []parametric.Edge {
	if s == nil {
		return nil
	}
	return s.graph.Edges()
}
func (s *WrappedAlgebraicCovectorChain) Faces() []Face {
	if s == nil {
		return nil
	}
	return s.graph.Faces()
}
func (s *WrappedAlgebraicCovectorChain) Boundary() []WrappedOpenBoundaryUse {
	if s == nil {
		return nil
	}
	return s.graph.Boundary()
}
func (s *WrappedAlgebraicCovectorChain) VertexSources(i int) ([]WrappedOpenVertexSource, error) {
	if s == nil {
		return nil, geom.ErrInvalid
	}
	return s.graph.VertexSources(i)
}
func (s *WrappedAlgebraicCovectorChain) EdgeSources(i int) ([]WrappedOpenEdgeSource, error) {
	if s == nil {
		return nil, geom.ErrInvalid
	}
	return s.graph.EdgeSources(i)
}
func (s *WrappedAlgebraicCovectorChain) ExactOperations() int {
	if s == nil {
		return 0
	}
	return s.operations
}

// NewWrappedAlgebraicCovectorChain proves both adjacent pairs and the
// independent strict A/C separator before publishing native topology. The
// original middle source must match, its mates have disjoint endpoints, and
// all three original source objects are distinct. One arithmetic budget covers
// all three geometric proofs and every exported native edge/use recheck.
func NewWrappedAlgebraicCovectorChain(ctx context.Context, ab, bc *WrappedAlgebraicOpenShell, adjacent [2]WrappedCovectorSeparator, outer WrappedCovectorSeparator, o geom.SurfaceAffineCoordinateOptions) (*WrappedAlgebraicCovectorChain, error) {
	if ctx == nil || ab == nil || bc == nil || ab.sources[1] == nil || ab.sources[1] != bc.sources[0] || ab.sources[0] == bc.sources[1] {
		return nil, geom.ErrInvalid
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 64000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return nil, geom.ErrInvalid
	}
	out := &WrappedAlgebraicCovectorChain{}
	for i, pair := range []*WrappedAlgebraicOpenShell{ab, bc} {
		if out.operations >= o.MaxOperations {
			return nil, geom.ErrConvergence
		}
		opts := o
		opts.MaxOperations -= out.operations
		p, e := CertifyWrappedAlgebraicCovectorSeparatedPair(ctx, pair, adjacent[i], opts)
		if e != nil {
			return nil, e
		}
		out.operations += p.ExactOperations()
		out.adjacent[i] = p
	}
	a, _ := ab.pair.Owner(1)
	b, _ := bc.pair.Owner(0)
	for _, x := range a.Vertices {
		for _, y := range b.Vertices {
			if x == y {
				return nil, geom.ErrDomain
			}
		}
	}
	if out.operations >= o.MaxOperations {
		return nil, geom.ErrConvergence
	}
	opts := o
	opts.MaxOperations -= out.operations
	strict, e := CertifyWrappedStrictCovectorFaceSeparation(ctx, ab.sources[0], bc.sources[1], outer, opts)
	if e != nil {
		return nil, e
	}
	out.operations += strict.ExactOperations()
	out.outer = strict
	if out.operations >= o.MaxOperations {
		return nil, geom.ErrConvergence
	}
	graph, e := materializeWrappedPairChain(ctx, []*WrappedAlgebraicOpenShell{ab, bc}, nil, edgecheck.Options{MaxWork: o.MaxOperations - out.operations, MaxBits: o.MaxCoefficientBits, MaxIntervals: 8192})
	if e != nil {
		return nil, e
	}
	out.operations += graph.ExactOperations()
	out.graph = graph
	return out, nil
}
