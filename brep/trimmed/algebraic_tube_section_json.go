package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

type nativeAlgebraicTubeSection struct {
	Version int                `json:"version"`
	Kind    string             `json:"kind"`
	Tubes   []nativeTubeRecipe `json:"tubes"`
}

// WriteAlgebraicSplineSectionJSON stores original curve recipes for a proved
// nonempty section. It does not persist seed or coverage proof flags.
func WriteAlgebraicSplineSectionJSON(w io.Writer, s AlgebraicSplinePairSection) error {
	if w == nil || len(s.recipes) == 0 {
		return geom.ErrInvalid
	}
	if e := validateTubeRecipes(s.recipes, 4096); e != nil {
		return e
	}
	f := nativeAlgebraicTubeSection{1, "trimmed-algebraic-tube-section", s.recipes}
	b, e := json.Marshal(f)
	if e != nil {
		return e
	}
	b = append(b, '\n')
	n, e := w.Write(b)
	if e == nil && n != len(b) {
		e = io.ErrShortWrite
	}
	return e
}

// ReadAlgebraicSplineSectionJSON revalidates every tube against common
// original support charts, then independently reproves complete coverage,
// branch topology and the exact original-face terminals.
func ReadAlgebraicSplineSectionJSON(ctx context.Context, r io.Reader, l JSONLimits, trace intersect.SplinePairTraceOptions,
	o geom.TubeAlgebraicGraphEndpointOptions) (AlgebraicSplinePairSection, error) {
	var empty AlgebraicSplinePairSection
	if ctx == nil || r == nil || l.MaxBytes < 0 || l.MaxBytes > 1<<30 || l.MaxControlPoints < 0 || l.MaxEntities < 0 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
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
		return empty, e
	}
	if int64(len(b)) > l.MaxBytes {
		return empty, geom.ErrConvergence
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	var f nativeAlgebraicTubeSection
	if e = d.Decode(&f); e != nil {
		return empty, e
	}
	if e = d.Decode(new(any)); e != io.EOF || f.Version != 1 || f.Kind != "trimmed-algebraic-tube-section" || len(f.Tubes) == 0 {
		return empty, geom.ErrInvalid
	}
	if e := validateTubeRecipes(f.Tubes, l.MaxControlPoints); e != nil {
		return empty, e
	}
	first := f.Tubes[0]
	charts, seeds, e := rebuildAlgebraicTubeSeeds(ctx, [2]nativeTubePatch{first.A, first.B}, f.Tubes, l, trace)
	if e != nil {
		return empty, e
	}
	return SectionSplinePairAlgebraic(ctx, charts[0], charts[1], &seeds, trace, o)
}

func rebuildAlgebraicTubeSeeds(ctx context.Context, input [2]nativeTubePatch, tubes []nativeTubeRecipe, l JSONLimits,
	trace intersect.SplinePairTraceOptions) (charts [2]intersect.Patch, seeds intersect.SplineSeedReport, err error) {
	for i, p := range input {
		s, e := parametric.DecodeSurface(p.Surface, l.MaxControlPoints)
		if e != nil {
			return charts, seeds, e
		}
		if _, ok := s.(*geom.BSplineSurface); !ok {
			return charts, seeds, geom.ErrInvalid
		}
		charts[i] = intersect.Patch{Surface: s, Min: p.Min, Max: p.Max}
	}
	var patches [2]*geom.SurfacePatch
	for i, c := range charts {
		p, e := geom.NewSurfacePatch(c.Surface.(*geom.BSplineSurface), trace.Seeds.Patches)
		if e != nil {
			return charts, seeds, e
		}
		patches[i], e = p.Trim(geom.ParameterRectangle{UMin: c.Min.X, UMax: c.Max.X, VMin: c.Min.Y, VMax: c.Max.Y})
		if e != nil {
			return charts, seeds, e
		}
	}
	for _, tube := range tubes {
		if e := ctx.Err(); e != nil {
			return charts, seeds, e
		}
		if tube.A.Min != input[0].Min || tube.A.Max != input[0].Max || tube.B.Min != input[1].Min || tube.B.Max != input[1].Max ||
			!bytes.Equal(tube.A.Surface, input[0].Surface) || !bytes.Equal(tube.B.Surface, input[1].Surface) {
			return charts, seeds, geom.ErrInvalid
		}
		if len(tube.ExactEndpoints) > 0 {
			return charts, seeds, geom.ErrConvergence
		}
		if _, e := tube.rebuild(ctx, l.MaxControlPoints); e != nil {
			return charts, seeds, e
		}
		for i, c := range tube.Cells {
			if len(seeds.Attempts) >= l.MaxEntities {
				return charts, seeds, geom.ErrConvergence
			}
			a, e := patches[0].Trim(c.DomainA)
			if e != nil {
				return charts, seeds, e
			}
			b, e := patches[1].Trim(c.DomainB)
			if e != nil {
				return charts, seeds, e
			}
			axis := tube.FreeAxis
			if len(tube.Oriented) > 0 {
				axis = tube.Oriented[i].FreeAxis
			}
			options := geom.SurfacePairTubeOptions{FreeAxis: axis, MaxOperations: trace.Seeds.TubeOptions.MaxOperations,
				MaxCoefficientBits: trace.Seeds.TubeOptions.MaxCoefficientBits}
			var q geom.SurfaceTubeCertificate
			var anchor *intersect.TubeBoundaryAnchor
			if c.BoundaryAnchor == nil {
				q, e = geom.CertifySurfacePairTube(a, b, options)
			} else {
				x := *c.BoundaryAnchor
				var uv [4]*big.Rat
				for j, v := range x.Parameters {
					if !finite(v) {
						return charts, seeds, geom.ErrInvalid
					}
					uv[j] = ratInset(v)
				}
				q, e = geom.CertifyBoundaryAnchoredSurfacePairTube(ctx, a, b, geom.SurfaceTubeBoundaryOptions{
					SurfacePairTubeOptions: options, AtUpper: x.AtUpper, Anchor: uv})
				anchor = &x
			}
			if e != nil {
				return charts, seeds, e
			}
			if !q.ProvenBranch {
				return charts, seeds, geom.ErrConvergence
			}
			seeds.Attempts = append(seeds.Attempts, intersect.SplineSeedCell{Cell: geom.SurfaceSurfaceCell{DomainA: c.DomainA, DomainB: c.DomainB}, Tube: &q, BoundaryAnchor: anchor})
		}
	}
	return charts, seeds, nil
}
