//go:build research

package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"io"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

type nativeShellSectionPair struct {
	Faces [2]int             `json:"faces"`
	Tubes []nativeTubeRecipe `json:"tubes"`
}
type nativeAlgebraicShellSection struct {
	Version int                      `json:"version"`
	Kind    string                   `json:"kind"`
	Shells  [2]json.RawMessage       `json:"shells"`
	Pairs   []nativeShellSectionPair `json:"pairs,omitempty"`
}

// WriteAlgebraicShellSectionJSON stores original shells and tube proposals.
// Edge ownership, section incidence and the exclusion of other face pairs
// are independently reproved on read.
func WriteAlgebraicShellSectionJSON(w io.Writer, r AlgebraicShellSectionResult) error {
	if w == nil || r.source == nil {
		return geom.ErrInvalid
	}
	var q nativeAlgebraicShellSection
	q.Version, q.Kind = 1, "trimmed-algebraic-shell-section"
	for i, s := range [2]Solid{r.source.a, r.source.b} {
		var b bytes.Buffer
		if e := WriteJSON(&b, s); e != nil {
			return e
		}
		q.Shells[i] = b.Bytes()
	}
	for i, s := range r.source.sections {
		if len(s.recipes) > 0 {
			q.Pairs = append(q.Pairs, nativeShellSectionPair{r.source.pairs[i], s.recipes})
		}
	}
	var all []nativeTubeRecipe
	for _, p := range q.Pairs {
		all = append(all, p.Tubes...)
	}
	if e := validateTubeRecipes(all, 4096); e != nil {
		return e
	}
	p, e := json.Marshal(q)
	if e != nil {
		return e
	}
	p = append(p, '\n')
	n, e := w.Write(p)
	if e == nil && n != len(p) {
		return io.ErrShortWrite
	}
	return e
}

func ReadAlgebraicShellSectionJSON(ctx context.Context, r io.Reader, l JSONLimits, o AlgebraicShellSectionOptions) (out AlgebraicShellSectionResult, err error) {
	if ctx == nil || r == nil || l.MaxBytes < 0 || l.MaxBytes > 1<<30 || l.MaxControlPoints < 0 || l.MaxEntities < 0 {
		return out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
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
	b, e := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if e != nil {
		return out, e
	}
	if int64(len(b)) > l.MaxBytes {
		return out, geom.ErrConvergence
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	var q nativeAlgebraicShellSection
	if e := d.Decode(&q); e != nil {
		return out, e
	}
	if e := d.Decode(new(any)); e != io.EOF || q.Version != 1 || q.Kind != "trimmed-algebraic-shell-section" || len(q.Pairs) > 32 {
		return out, geom.ErrInvalid
	}
	var all []nativeTubeRecipe
	cells := 0
	for _, p := range q.Pairs {
		if len(p.Tubes) == 0 || len(p.Tubes) > 32 {
			return out, geom.ErrInvalid
		}
		all = append(all, p.Tubes...)
		for _, t := range p.Tubes {
			cells += len(t.Cells)
			if cells > l.MaxEntities {
				return out, geom.ErrConvergence
			}
		}
	}
	if e := validateTubeRecipes(all, l.MaxControlPoints); e != nil {
		return out, e
	}
	var s [2]Solid
	for i, x := range q.Shells {
		s[i], e = ReadJSONContext(ctx, bytes.NewReader(x), l)
		if e != nil {
			return out, e
		}
	}
	if len(s[0].vertices)+len(s[0].edges)+len(s[0].faces)+len(s[1].vertices)+len(s[1].edges)+len(s[1].faces)+cells > l.MaxEntities {
		return out, geom.ErrConvergence
	}
	seeds, e := rebuildShellSectionSeeds(ctx, s, q.Pairs, l, o)
	if e != nil {
		return out, e
	}
	o.Seeds = seeds

	return SectionSplineShellsAlgebraic(ctx, s[0], s[1], o)
}

func rebuildShellSectionSeeds(ctx context.Context, s [2]Solid, pairs []nativeShellSectionPair, l JSONLimits, o AlgebraicShellSectionOptions) ([]AlgebraicShellSectionSeed, error) {
	o.Seeds = nil
	seen := map[[2]int]bool{}
	for _, p := range pairs {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		if seen[p.Faces] {
			return nil, geom.ErrInvalid
		}
		seen[p.Faces] = true
		first := p.Tubes[0]
		patches, seeds, e := rebuildAlgebraicTubeSeeds(ctx, [2]nativeTubePatch{first.A, first.B}, p.Tubes, l, o.Trace)
		if e != nil {
			return nil, e
		}
		for i, ix := range p.Faces {
			if ix < 0 || ix >= len(s[i].faces) {
				return nil, geom.ErrInvalid
			}
			f := s[i].faces[ix]
			b, e := faceUVBounds(f)
			if e != nil {
				return nil, e
			}
			x := patches[i]
			same := sameTrimmedSectionChart(f.Surface, x.Surface)
			base, _ := parametric.SurfaceMapping(f.Surface)
			if _, exact := base.(*geom.ExactBezierSurface); exact {
				// Recipes and shell faces are independently decoded. Require
				// canonical stored identity before using the saved proposals.
				p, e := parametric.EncodeSurface(f.Surface)
				if e != nil {
					return nil, e
				}
				q, e := parametric.EncodeSurface(x.Surface)
				if e != nil {
					return nil, e
				}
				same = bytes.Equal(p, q)
			}
			if !same || b != (geom.ParameterRectangle{UMin: x.Min.X, UMax: x.Max.X, VMin: x.Min.Y, VMax: x.Max.Y}) {
				return nil, geom.ErrDomain
			}
		}
		o.Seeds = append(o.Seeds, AlgebraicShellSectionSeed{p.Faces, seeds})
	}
	return o.Seeds, nil
}
