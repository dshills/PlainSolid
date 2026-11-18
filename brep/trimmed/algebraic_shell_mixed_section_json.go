//go:build research

package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math/big"
	"reflect"

	"plainsolid/geom"
)

type nativeMixedIsoPair struct {
	Faces                    [2]int
	SourceOperand, FixedAxis int
	Fixed                    float64
	Cuts                     []string
	Pieces                   []geom.RationalIsoTubePiece
}
type nativeMixedBranchOwner struct {
	Faces, Edges [2]int
	Restricted   bool
	Source       int
}
type nativeMixedShellSection struct {
	Version  int
	Kind     string
	Shells   [2]json.RawMessage
	Pairs    []nativeShellSectionPair
	Rational []nativeMixedIsoPair
	Owners   []nativeMixedBranchOwner
	Wires    [][]AlgebraicShellSectionWireUse
}

func mixedSectionOwners(s *MixedAlgebraicShellSection) []nativeMixedBranchOwner {
	r := make([]nativeMixedBranchOwner, len(s.source.branches))
	for i, b := range s.source.branches {
		r[i] = nativeMixedBranchOwner{b.Faces, b.EndEdges, b.rational != nil, b.rationalSource}
	}
	return r
}

// WriteMixedAlgebraicShellSectionJSON stores original supports and both kinds
// of guidance. Only immutable certified sources are written; no Complete flag
// is accepted. The entire bounded payload is prepared before publishing bytes.
func WriteMixedAlgebraicShellSectionJSON(w io.Writer, s *MixedAlgebraicShellSection) error {
	if w == nil || s == nil || s.source == nil {
		return geom.ErrInvalid
	}
	q := nativeMixedShellSection{Version: 1, Kind: "trimmed-mixed-algebraic-shell-section", Owners: mixedSectionOwners(s), Wires: s.Wires()}
	for i, body := range [2]Solid{s.source.a, s.source.b} {
		var b bytes.Buffer
		if e := WriteJSON(&b, body); e != nil {
			return e
		}
		q.Shells[i] = b.Bytes()
	}
	for i, p := range s.source.sections {
		if len(p.recipes) > 0 {
			q.Pairs = append(q.Pairs, nativeShellSectionPair{s.source.pairs[i], p.recipes})
		}
	}
	for _, p := range s.proposals {
		r := nativeMixedIsoPair{Faces: p.Faces, SourceOperand: p.SourceOperand, FixedAxis: p.FixedAxis, Fixed: p.Fixed, Pieces: p.Pieces}
		for _, v := range p.Cuts {
			r.Cuts = append(r.Cuts, v.RatString())
		}
		q.Rational = append(q.Rational, r)
	}
	if len(q.Pairs) > 64 || len(q.Rational) > 64 || len(q.Owners) > 4096 || len(q.Wires) > 4096 {
		return geom.ErrConvergence
	}
	var all []nativeTubeRecipe
	count := len(q.Owners)
	for _, body := range [2]Solid{s.source.a, s.source.b} {
		count += len(body.vertices) + len(body.edges) + len(body.faces)
	}
	for _, p := range q.Pairs {
		if len(p.Tubes) > 128 {
			return geom.ErrConvergence
		}
		all = append(all, p.Tubes...)
		for _, t := range p.Tubes {
			count += len(t.Cells)
		}
	}
	for _, p := range q.Rational {
		count += len(p.Pieces)
	}
	for _, w := range q.Wires {
		count += len(w)
	}
	if count > 20000 {
		return geom.ErrConvergence
	}
	if err := validateTubeRecipes(all, 4096); err != nil {
		return err
	}
	data, err := json.Marshal(q)
	if err != nil {
		return err
	}
	if len(data) > 32<<20 {
		return geom.ErrConvergence
	}
	data = append(data, '\n')
	n, err := w.Write(data)
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	return err
}

// ReadMixedAlgebraicShellSectionJSON independently reconstructs every original
// face pair, exact restricted interval and global owner join. Stored incidence
// is only a witness to compare after full reproof, never trusted topology.
func ReadMixedAlgebraicShellSectionJSON(ctx context.Context, r io.Reader, l JSONLimits, o MixedAlgebraicShellSectionOptions) (*MixedAlgebraicShellSection, error) {
	if ctx == nil || r == nil || l.MaxBytes < 0 || l.MaxBytes > 1<<30 || l.MaxControlPoints < 0 || l.MaxEntities < 0 {
		return nil, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if l.MaxBytes == 0 {
		l.MaxBytes = 32 << 20
	}
	if l.MaxControlPoints == 0 {
		l.MaxControlPoints = 4096
	}
	if l.MaxEntities == 0 {
		l.MaxEntities = 20000
	}
	data, err := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > l.MaxBytes {
		return nil, geom.ErrConvergence
	}
	var q nativeMixedShellSection
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&q); err != nil {
		return nil, err
	}
	if err := d.Decode(new(any)); err != io.EOF || q.Version != 1 || q.Kind != "trimmed-mixed-algebraic-shell-section" {
		return nil, geom.ErrInvalid
	}
	if len(q.Pairs) > 64 || len(q.Rational) == 0 || len(q.Rational) > 64 || len(q.Owners) > 4096 || len(q.Wires) > 4096 {
		return nil, geom.ErrConvergence
	}
	count := len(q.Owners)
	var all []nativeTubeRecipe
	for _, p := range q.Pairs {
		if len(p.Tubes) == 0 || len(p.Tubes) > 128 {
			return nil, geom.ErrConvergence
		}
		all = append(all, p.Tubes...)
		for _, t := range p.Tubes {
			count += len(t.Cells)
		}
	}
	proposals := make([]RationalIsoShellPairProposal, len(q.Rational))
	for i, p := range q.Rational {
		if len(p.Pieces) == 0 || len(p.Pieces) > 64 || len(p.Cuts) != len(p.Pieces)+1 {
			return nil, geom.ErrInvalid
		}
		count += len(p.Pieces)
		z := RationalIsoShellPairProposal{Faces: p.Faces, SourceOperand: p.SourceOperand, FixedAxis: p.FixedAxis, Fixed: p.Fixed, Pieces: p.Pieces}
		for _, v := range p.Cuts {
			if len(v) > 10000 {
				return nil, geom.ErrConvergence
			}
			// Reject exponent syntax before big.Rat can allocate an
			// exponent-sized integer from a short untrusted string.
			for i, c := range v {
				if c >= '0' && c <= '9' || c == '/' || c == '-' && i == 0 {
					continue
				}
				return nil, geom.ErrInvalid
			}
			x, ok := new(big.Rat).SetString(v)
			if !ok || x.RatString() != v {
				return nil, geom.ErrInvalid
			}
			if x.Num().BitLen() > 16384 || x.Denom().BitLen() > 16384 {
				return nil, geom.ErrConvergence
			}
			z.Cuts = append(z.Cuts, x)
		}
		proposals[i] = z
	}
	for _, w := range q.Wires {
		count += len(w)
	}
	if count > l.MaxEntities {
		return nil, geom.ErrConvergence
	}
	if err := validateTubeRecipes(all, l.MaxControlPoints); err != nil {
		return nil, err
	}
	var shells [2]Solid
	for i, p := range q.Shells {
		shells[i], err = ReadJSONContext(ctx, bytes.NewReader(p), l)
		if err != nil {
			return nil, err
		}
		count += len(shells[i].vertices) + len(shells[i].edges) + len(shells[i].faces)
	}
	if count > l.MaxEntities {
		return nil, geom.ErrConvergence
	}
	seeds, err := rebuildShellSectionSeeds(ctx, shells, q.Pairs, l, o.Section)
	if err != nil {
		return nil, err
	}
	o.Section.Seeds = seeds
	out, err := SectionSplineShellsMixedAlgebraic(ctx, shells[0], shells[1], proposals, o)
	if err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(q.Owners, mixedSectionOwners(out)) || !reflect.DeepEqual(q.Wires, out.Wires()) {
		return nil, geom.ErrDomain
	}
	return out, nil
}
