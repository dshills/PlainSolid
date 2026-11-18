//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// WrappedAlgebraicSeparatedSeries is an embedded native open disk on two to
// eight original wrapped charts. All adjacent mates are explicit. Strictly
// ordered parallel separator planes prove separation of every nonadjacent
// face pair; neither a common surface map nor inferred contact is used.
type WrappedAlgebraicSeparatedSeries struct {
	adjacent   []*WrappedAlgebraicSeparatedPair
	graph      *WrappedAlgebraicFaceChainShell
	operations int
}

func (s *WrappedAlgebraicSeparatedSeries) FaceCount() int {
	if s == nil || len(s.adjacent) == 0 {
		return 0
	}
	return len(s.adjacent) + 1
}
func (s *WrappedAlgebraicSeparatedSeries) Adjacent(i int) (*WrappedAlgebraicSeparatedPair, error) {
	if s == nil || i < 0 || i >= len(s.adjacent) {
		return nil, geom.ErrInvalid
	}
	return s.adjacent[i], nil
}

// NonadjacentGap returns the exact positive world-coordinate distance between
// the two separating support planes for original owners i and j (either order).
// Adjacent/equal owners reject: their contact is described by Adjacent instead.
func (s *WrappedAlgebraicSeparatedSeries) NonadjacentGap(i, j int) (*big.Rat, error) {
	if s == nil || i < 0 || j < 0 || i >= s.FaceCount() || j >= s.FaceCount() {
		return nil, geom.ErrInvalid
	}
	if i > j {
		i, j = j, i
	}
	if j-i < 2 {
		return nil, geom.ErrInvalid
	}
	return new(big.Rat).Abs(new(big.Rat).Sub(s.adjacent[j-1].separator.Level, s.adjacent[i].separator.Level)), nil
}
func (s *WrappedAlgebraicSeparatedSeries) Vertices() []geom.Vec3 {
	if s == nil {
		return nil
	}
	return s.graph.Vertices()
}
func (s *WrappedAlgebraicSeparatedSeries) Edges() []parametric.Edge {
	if s == nil {
		return nil
	}
	return s.graph.Edges()
}
func (s *WrappedAlgebraicSeparatedSeries) Faces() []Face {
	if s == nil {
		return nil
	}
	return s.graph.Faces()
}
func (s *WrappedAlgebraicSeparatedSeries) Boundary() []WrappedOpenBoundaryUse {
	if s == nil {
		return nil
	}
	return s.graph.Boundary()
}
func (s *WrappedAlgebraicSeparatedSeries) VertexSources(i int) ([]WrappedOpenVertexSource, error) {
	if s == nil {
		return nil, geom.ErrInvalid
	}
	return s.graph.VertexSources(i)
}
func (s *WrappedAlgebraicSeparatedSeries) EdgeSources(i int) ([]WrappedOpenEdgeSource, error) {
	if s == nil {
		return nil, geom.ErrInvalid
	}
	return s.graph.EdgeSources(i)
}
func (s *WrappedAlgebraicSeparatedSeries) ExactOperations() int {
	if s == nil {
		return 0
	}
	return s.operations
}

// NewWrappedAlgebraicSeparatedSeries takes an ordered chain of one to seven
// pair shells and one separator proposal per pair. Consecutive pairs share
// their identical immutable original middle face; original face reuse rejects.
// Both sides of every mate are re-proved. All separator axes and inside/outside
// orientations agree, with strictly monotone levels. Thus face i and j>=i+2
// are separated by the positive difference of levels i and j-1. Every indexed
// native use and vertex fan is checked under the remaining cumulative budget.
// The original three-face JSON format is intentionally not widened.
func NewWrappedAlgebraicSeparatedSeries(ctx context.Context, pairs []*WrappedAlgebraicOpenShell, separators []WrappedCoordinateSeparator, o geom.SurfaceAffineCoordinateOptions) (*WrappedAlgebraicSeparatedSeries, error) {
	if ctx == nil || len(pairs) < 1 || len(pairs) > 7 || len(separators) != len(pairs) {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	seen := map[*WrappedAlgebraicQuadraticFace]bool{}
	for i, pair := range pairs {
		if pair == nil || pair.sources[0] == nil || pair.sources[1] == nil {
			return nil, geom.ErrInvalid
		}
		if i == 0 {
			seen[pair.sources[0]] = true
		} else if pairs[i-1].sources[1] != pair.sources[0] {
			return nil, geom.ErrInvalid
		}
		if seen[pair.sources[1]] {
			return nil, geom.ErrDomain
		}
		seen[pair.sources[1]] = true
		if separators[i].Axis != separators[0].Axis {
			return nil, geom.ErrInvalid
		}
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
	out := &WrappedAlgebraicSeparatedSeries{adjacent: make([]*WrappedAlgebraicSeparatedPair, len(pairs))}
	for i, pair := range pairs {
		if out.operations >= o.MaxOperations {
			return nil, geom.ErrConvergence
		}
		opts := o
		opts.MaxOperations -= out.operations
		p, e := CertifyWrappedAlgebraicSeparatedPair(ctx, pair, separators[i], opts)
		if e != nil {
			return nil, e
		}
		out.operations += p.ExactOperations()
		out.adjacent[i] = p
		if i > 0 {
			previous := out.adjacent[i-1]
			difference := new(big.Rat).Sub(p.separator.Level, previous.separator.Level)
			out.operations++
			if out.operations > o.MaxOperations || difference.Num().BitLen() > o.MaxCoefficientBits || difference.Denom().BitLen() > o.MaxCoefficientBits {
				return nil, geom.ErrConvergence
			}
			if p.sides[0] != previous.sides[0] || difference.Sign() != -previous.sides[0] {
				return nil, geom.ErrConvergence
			}
			a, _ := pairs[i-1].pair.Owner(1)
			b, _ := pair.pair.Owner(0)
			for _, x := range a.Vertices {
				for _, y := range b.Vertices {
					if x == y {
						return nil, geom.ErrDomain
					}
				}
			}
		}
	}
	// Bound the exact non-neighbor gaps exposed by the public accessor too.
	for i := 0; i < len(out.adjacent); i++ {
		for j := i + 2; j < len(out.adjacent); j++ {
			gap := new(big.Rat).Sub(out.adjacent[j].separator.Level, out.adjacent[i].separator.Level)
			out.operations++
			if out.operations > o.MaxOperations || gap.Num().BitLen() > o.MaxCoefficientBits || gap.Denom().BitLen() > o.MaxCoefficientBits {
				return nil, geom.ErrConvergence
			}
		}
	}
	if out.operations >= o.MaxOperations {
		return nil, geom.ErrConvergence
	}
	graph, e := materializeWrappedPairChain(ctx, pairs, nil, edgecheck.Options{MaxWork: o.MaxOperations - out.operations, MaxBits: o.MaxCoefficientBits, MaxIntervals: 8192})
	if e != nil {
		return nil, e
	}
	out.operations += graph.ExactOperations()
	out.graph = graph
	return out, nil
}
