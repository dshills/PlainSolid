package trimmed

import (
	"context"
	"errors"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

// MixedAlgebraicSectionPiece identifies exactly one tube or a retained rational
// piece. Restricted and Segment must not be interchanged: Index selects the
// restricted chain's exact domain; its traversal is increasing source parameter.
type MixedAlgebraicSectionPiece struct {
	Faces                [2]int
	Segment              *intersect.TubeSegment
	Restricted           *geom.RationalIsoTubeChain
	Index, SourceOperand int
}

func mixedSectionPiece(source *algebraicShellSectionSource, p AlgebraicShellImprintPiece) (MixedAlgebraicSectionPiece, error) {
	if source == nil || p.Branch < 0 || p.Branch >= len(source.branches) {
		return MixedAlgebraicSectionPiece{}, geom.ErrInvalid
	}
	b := source.branches[p.Branch]
	if p.Index < 0 || p.Index >= shellSectionPieceCount(b) {
		return MixedAlgebraicSectionPiece{}, geom.ErrInvalid
	}
	r := MixedAlgebraicSectionPiece{Faces: b.Faces, Index: p.Index, Restricted: b.rational, SourceOperand: b.rationalSource}
	if b.rational == nil {
		v := b.Segments[p.Index]
		r.Segment = &v
	}
	return r, nil
}

// MixedAlgebraicShellImprint retains both original shells with exact cuts,
// including restricted rational chart pieces. It is not a float-only Solid.
type MixedAlgebraicShellImprint struct {
	section *MixedAlgebraicShellSection
	parts   [2]AlgebraicShellImprint
}

func (s *MixedAlgebraicShellSection) ImprintOriginalShells(ctx context.Context, o AlgebraicShellImprintOptions) (*MixedAlgebraicShellImprint, error) {
	if s == nil || s.source == nil {
		return nil, geom.ErrInvalid
	}
	parts, e := imprintAlgebraicShells(ctx, AlgebraicShellSectionResult{source: s.source}, o, true)
	if e != nil {
		return nil, e
	}
	return &MixedAlgebraicShellImprint{section: s, parts: parts}, nil
}
func (s *MixedAlgebraicShellImprint) MarshalJSON() ([]byte, error) {
	return nil, geom.ErrUnrepresentable
}

// Faces returns owned fragment cycles; section use indices are resolved by SectionPiece.
func (s *MixedAlgebraicShellImprint) Faces(operand int) ([]AlgebraicShellImprintFace, error) {
	if s == nil || s.section == nil || operand < 0 || operand > 1 {
		return nil, geom.ErrInvalid
	}
	r := make([]AlgebraicShellImprintFace, len(s.parts[operand].Faces))
	for i, f := range s.parts[operand].Faces {
		r[i] = f
		r[i].Outer = append([]AlgebraicShellImprintUse(nil), f.Outer...)
		r[i].Holes = make([][]AlgebraicShellImprintUse, len(f.Holes))
		for j, h := range f.Holes {
			r[i].Holes[j] = append([]AlgebraicShellImprintUse(nil), h...)
		}
	}
	return r, nil
}
func (s *MixedAlgebraicShellImprint) Edges(operand int) ([]AlgebraicShellImprintEdge, error) {
	if s == nil || s.section == nil || operand < 0 || operand > 1 {
		return nil, geom.ErrInvalid
	}
	return append([]AlgebraicShellImprintEdge(nil), s.parts[operand].Edges...), nil
}
func (s *MixedAlgebraicShellImprint) Vertices(operand int) ([]AlgebraicShellImprintVertex, error) {
	if s == nil || s.section == nil || operand < 0 || operand > 1 {
		return nil, geom.ErrInvalid
	}
	return append([]AlgebraicShellImprintVertex(nil), s.parts[operand].Vertices...), nil
}
func (s *MixedAlgebraicShellImprint) SourceCurveCells(ctx context.Context, operand, edge int, o AlgebraicSeamOptions) (geom.SurfaceCurveCompositionCells, error) {
	if s == nil || s.section == nil || operand < 0 || operand > 1 {
		return geom.SurfaceCurveCompositionCells{}, geom.ErrInvalid
	}
	return s.parts[operand].Subdivision.ModelCurveCells(ctx, edge, o)
}

func (s *MixedAlgebraicShellImprint) Topology(operand int) (AlgebraicSubdivisionTopology, error) {
	if s == nil || s.section == nil || operand < 0 || operand > 1 {
		return AlgebraicSubdivisionTopology{}, geom.ErrInvalid
	}
	return s.parts[operand].Topology, nil
}
func (s *MixedAlgebraicShellImprint) SectionPiece(index int) (MixedAlgebraicSectionPiece, error) {
	if s == nil || s.section == nil || index < 0 || index >= len(s.parts[0].Section) {
		return MixedAlgebraicSectionPiece{}, geom.ErrInvalid
	}
	return mixedSectionPiece(s.section.source, s.parts[0].Section[index])
}

// MixedAlgebraicShellBoolean is an immutable exact indexed material boundary.
// Its section accessor returns tagged original curves and preserves rational
// chart restrictions. No legacy tube-only Boolean value is exposed.
type MixedAlgebraicShellBoolean struct {
	boundary AlgebraicShellBoolean
	section  *MixedAlgebraicShellSection
}

func (s *MixedAlgebraicShellBoolean) MarshalJSON() ([]byte, error) {
	return nil, geom.ErrUnrepresentable
}

func (s *MixedAlgebraicShellBoolean) Topology() (AlgebraicSubdivisionTopology, error) {
	if s == nil || s.section == nil {
		return AlgebraicSubdivisionTopology{}, geom.ErrInvalid
	}
	return s.boundary.Topology, nil
}
func (s *MixedAlgebraicShellBoolean) Faces() []AlgebraicShellBooleanFace {
	if s == nil || s.section == nil {
		return nil
	}
	r := make([]AlgebraicShellBooleanFace, len(s.boundary.faces))
	for i, f := range s.boundary.faces {
		r[i] = f
		r[i].Outer = append([]AlgebraicShellBooleanUse(nil), f.Outer...)
		r[i].Holes = make([][]AlgebraicShellBooleanUse, len(f.Holes))
		for j, h := range f.Holes {
			r[i].Holes[j] = append([]AlgebraicShellBooleanUse(nil), h...)
		}
	}
	return r
}
func (s *MixedAlgebraicShellBoolean) Edges() []AlgebraicShellBooleanEdge {
	if s == nil || s.section == nil {
		return nil
	}
	return append([]AlgebraicShellBooleanEdge(nil), s.boundary.index...)
}
func (s *MixedAlgebraicShellBoolean) Vertices() []AlgebraicShellBooleanVertex {
	if s == nil || s.section == nil {
		return nil
	}
	return append([]AlgebraicShellBooleanVertex(nil), s.boundary.vertices...)
}
func (s *MixedAlgebraicShellBoolean) SectionPiece(edge int) (MixedAlgebraicSectionPiece, error) {
	if s == nil || s.section == nil || edge < 0 || edge >= len(s.boundary.index) || s.boundary.index[edge].SectionPiece < 0 {
		return MixedAlgebraicSectionPiece{}, geom.ErrInvalid
	}
	p := s.boundary.parts[0].Section[s.boundary.index[edge].SectionPiece]
	return mixedSectionPiece(s.section.source, p)
}
func (s *MixedAlgebraicShellBoolean) OriginalFace(face int) (Face, error) {
	if s == nil || s.section == nil {
		return Face{}, geom.ErrInvalid
	}
	return s.boundary.OriginalFace(face)
}
func (s *MixedAlgebraicShellBoolean) SourceCurveCells(ctx context.Context, edge int, o AlgebraicSeamOptions) (geom.SurfaceCurveCompositionCells, error) {
	if s == nil || s.section == nil {
		return geom.SurfaceCurveCompositionCells{}, geom.ErrInvalid
	}
	return s.boundary.SourceCurveCells(ctx, edge, o)
}
func (s *MixedAlgebraicShellBoolean) LocateMaterial(ctx context.Context, p geom.Vec3, o SplineShellBooleanOptions) (parametric.Location, error) {
	if s == nil || s.section == nil {
		return parametric.Outside, geom.ErrInvalid
	}
	location, err := s.boundary.LocateMaterial(ctx, p, o)
	if err == nil || !errors.Is(err, geom.ErrConvergence) {
		return location, err
	}
	// The general inverse may remain unresolved on a nonlinear original
	// wall. A retained exact section vertex is nevertheless a proved exposed
	// result boundary. Compare its original rational model witness, never its
	// display box or a rounded parameter.
	budget := o.Embedding.Patches.MaxOperations
	if budget == 0 || budget > 1000000 {
		budget = 1000000
	}
	query := [3]*big.Rat{ratInset(p.X), ratInset(p.Y), ratInset(p.Z)}
	for _, v := range s.boundary.vertices {
		if e := ctx.Err(); e != nil {
			return parametric.Outside, e
		}
		event := v.SectionPoint
		if event == nil {
			continue
		}
		domain := event.Root().Interval()
		if domain.Lower.Cmp(domain.Upper) != 0 {
			continue
		}
		if budget < 1 {
			return location, err
		}
		witness, e := event.EvaluateRationalWitness(ctx, domain.Lower, geom.ExactCurvePlaneEventOptions{MaxOperations: budget, MaxCoefficientBits: o.Embedding.Patches.MaxCoefficientBits})
		if e != nil {
			return location, e
		}
		budget -= witness.ExactOperations
		same := true
		for i, x := range witness.Point {
			same = same && x.Cmp(query[i]) == 0
		}
		if same {
			return parametric.Boundary, nil
		}
	}
	return location, err
}

// CombineAlgebraicOperations uses exact shared section incidence to select and
// sew material fragments. Restricted pieces are currently admitted only in
// open face cuts; their closed-loop winding is not inferred from proof boxes.
func (s *MixedAlgebraicShellSection) CombineAlgebraicOperations(ctx context.Context, operations []parametric.BooleanOp, o AlgebraicShellBooleanOptions) ([]*MixedAlgebraicShellBoolean, error) {
	if ctx == nil || s == nil || s.source == nil || len(operations) == 0 || len(operations) > 3 || o.Shells.MaxRays < 0 || o.Shells.MaxRays > 12 {
		return nil, geom.ErrInvalid
	}
	var seen [3]bool
	for _, op := range operations {
		if op > parametric.Difference || seen[op] {
			return nil, geom.ErrInvalid
		}
		seen[op] = true
	}
	if o.Shells.MaxRays == 0 {
		o.Shells.MaxRays = 12
	}
	imprint, e := s.ImprintOriginalShells(ctx, o.Imprint)
	if e != nil {
		return nil, e
	}
	var inside [2][]bool
	for i, p := range imprint.parts {
		inside[i], e = classifyAlgebraicImprint(ctx, p, imprint.parts[1-i].Subdivision.source, o.Shells)
		if e != nil {
			return nil, e
		}
	}
	out := make([]*MixedAlgebraicShellBoolean, len(operations))
	for i, op := range operations {
		b, e := sewAlgebraicImprint(ctx, s.source, imprint.parts, inside, op)
		if e != nil {
			return nil, e
		}
		out[i] = &MixedAlgebraicShellBoolean{boundary: b, section: s}
	}
	return out, nil
}
