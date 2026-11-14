package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
	"plainsolid/intersect"
)

// RationalIsoShellPairProposal is guidance only. Faces use the original shell
// indices. Boxes are ordered source/target, with SourceOperand selecting source.
// Cuts partition the complete original source iso intersection, not a subset.
type RationalIsoShellPairProposal struct {
	Faces         [2]int
	SourceOperand int
	FixedAxis     int
	Fixed         float64
	Cuts          []*big.Rat
	Pieces        []geom.RationalIsoTubePiece
}
type MixedAlgebraicShellSectionOptions struct {
	Section  AlgebraicShellSectionOptions
	Rational geom.RationalIsoTubeChainOptions
	// MaxExactOperations bounds cumulative rational-chain construction.
	MaxExactOperations int
}

// MixedAlgebraicShellBranch retains exact restricted domains separately from
// legacy tubes. RestrictedSource identifies the original operand in the chain.
type MixedAlgebraicShellBranch struct {
	Faces            [2]int
	Segments         []intersect.TubeSegment
	Restricted       *geom.RationalIsoTubeChain
	RestrictedSource int
	Ends             [2]AlgebraicSplineTerminal
	EndEdges         [2]int
	EdgeParameters   [2]*geom.ExactAlgebraicRoot
	Closed           bool
}

// MixedAlgebraicShellSection is a complete section-wire certificate, not a
// material Boolean or a legacy unrestricted tube recipe. Its contents are owned.
type MixedAlgebraicShellSection struct {
	source    *algebraicShellSectionSource
	proposals []RationalIsoShellPairProposal
}

// MarshalJSON refuses accidental serialization without the versioned source recipe.
func (s *MixedAlgebraicShellSection) MarshalJSON() ([]byte, error) {
	return nil, geom.ErrUnrepresentable
}

func (s *MixedAlgebraicShellSection) Pairs() [][2]int {
	if s == nil || s.source == nil {
		return nil
	}
	return append([][2]int(nil), s.source.pairs...)
}
func (s *MixedAlgebraicShellSection) Wires() [][]AlgebraicShellSectionWireUse {
	if s == nil || s.source == nil {
		return nil
	}
	r := make([][]AlgebraicShellSectionWireUse, len(s.source.wires))
	for i, w := range s.source.wires {
		r[i] = append([]AlgebraicShellSectionWireUse(nil), w...)
	}
	return r
}
func (s *MixedAlgebraicShellSection) Branches() []MixedAlgebraicShellBranch {
	if s == nil || s.source == nil {
		return nil
	}
	r := make([]MixedAlgebraicShellBranch, len(s.source.branches))
	for i, b := range s.source.branches {
		r[i] = MixedAlgebraicShellBranch{Faces: b.Faces, Segments: append([]intersect.TubeSegment(nil), b.Segments...), Restricted: b.rational, RestrictedSource: b.rationalSource, Ends: b.Ends, EndEdges: b.EndEdges, EdgeParameters: b.EdgeParameters, Closed: b.Closed}
	}
	return r
}

type mixedShellSectionBuild struct {
	proposals       map[[2]int]*RationalIsoShellPairProposal
	options         geom.RationalIsoTubeChainOptions
	remaining, used int
}

// SectionSplineShellsMixedAlgebraic exhaustively checks every original face
// pair. Proposals replace only discovery; complete iso coverage, original tube
// proofs, terminal ownership and closed global wire joins are independently proved.
// Any failure returns nil, never a partially complete wire ledger.
func SectionSplineShellsMixedAlgebraic(ctx context.Context, a, b Solid, proposals []RationalIsoShellPairProposal, o MixedAlgebraicShellSectionOptions) (*MixedAlgebraicShellSection, error) {
	if ctx == nil || len(proposals) == 0 || len(proposals) > 64 || o.MaxExactOperations < 0 || o.MaxExactOperations > 256000000 {
		return nil, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if o.MaxExactOperations == 0 {
		o.MaxExactOperations = 16000000
	}
	m := &mixedShellSectionBuild{proposals: map[[2]int]*RationalIsoShellPairProposal{}, options: o.Rational, remaining: o.MaxExactOperations}
	owned := make([]RationalIsoShellPairProposal, len(proposals))
	for i, p := range proposals {
		if p.Faces[0] < 0 || p.Faces[0] >= len(a.faces) || p.Faces[1] < 0 || p.Faces[1] >= len(b.faces) || p.SourceOperand < 0 || p.SourceOperand > 1 || m.proposals[p.Faces] != nil || len(p.Pieces) == 0 || len(p.Pieces) > 64 || len(p.Cuts) != len(p.Pieces)+1 {
			return nil, geom.ErrInvalid
		}
		owned[i] = p
		owned[i].Pieces = append([]geom.RationalIsoTubePiece(nil), p.Pieces...)
		owned[i].Cuts = make([]*big.Rat, len(p.Cuts))
		for j, r := range p.Cuts {
			if r == nil {
				return nil, geom.ErrInvalid
			}
			if r.Num().BitLen() > 16384 || r.Denom().BitLen() > 16384 {
				return nil, geom.ErrConvergence
			}
			owned[i].Cuts[j] = new(big.Rat).Set(r)
		}
		m.proposals[p.Faces] = &owned[i]
	}
	r, err := sectionSplineShellsAlgebraic(ctx, a, b, o.Section, m)
	if err != nil {
		return nil, err
	}
	if !r.Complete || m.used != len(proposals) {
		return nil, geom.ErrConvergence
	}
	return &MixedAlgebraicShellSection{source: r.source, proposals: owned}, nil
}
func (m *mixedShellSectionBuild) branch(ctx context.Context, ids [2]int, patches [2]*geom.SurfacePatch, charts [2]intersect.Patch, shells [2]Solid, faces [2][]splineEmbeddingFace, o AlgebraicShellSectionOptions) (AlgebraicShellSectionBranch, error) {
	var z AlgebraicShellSectionBranch
	p := m.proposals[ids]
	source := p.SourceOperand
	if m.remaining <= 0 {
		return z, geom.ErrConvergence
	}
	opts := m.options
	if opts.MaxOperations == 0 || opts.MaxOperations > m.remaining {
		opts.MaxOperations = m.remaining
	}
	c, err := geom.CertifyRationalIsoTubeChain(ctx, patches[source], patches[1-source], p.FixedAxis, p.Fixed, p.Cuts, p.Pieces, opts)
	if err != nil {
		return z, err
	}
	m.remaining -= c.ExactOperations()
	m.used++
	z.Faces = ids
	z.rational = c
	z.rationalSource = source
	first, _ := c.Piece(0)
	last, _ := c.Piece(c.PieceCount() - 1)
	for end, q := range [2]geom.TubeAlgebraicGraphEndpointCertificate{first.Endpoints[0], last.Endpoints[1]} {
		if source == 1 {
			q.Parameters = [4]*geom.ExactAlgebraicRoot{q.Parameters[2], q.Parameters[3], q.Parameters[0], q.Parameters[1]}
			q.ParameterBoundsA, q.ParameterBoundsB = q.ParameterBoundsB, q.ParameterBoundsA
		}
		owner, side, err := locateAlgebraicTerminalBoundary(q, charts[0], charts[1])
		if err != nil {
			return z, err
		}
		t, err := verifyAlgebraicTerminalBoundary(ctx, q, charts[0], charts[1], owner, side, o.Endpoints)
		if err != nil {
			return z, err
		}
		edge, root, err := algebraicShellEdgeTerminal(ctx, shells[owner], ids[owner], faces[owner][ids[owner]], t, o.Endpoints)
		if err != nil {
			return z, err
		}
		z.Ends[end] = t
		z.EndEdges[end] = edge
		z.EdgeParameters[end] = root
	}
	return z, nil
}
