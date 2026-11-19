//go:build research

package trimmed

import (
	"context"
	"io"

	"plainsolid/geom"
)

// WriteAlgebraicTubeLensJSON stores the original section recipe. The lens's
// faces, orientation, and material ownership are derived on read.
func WriteAlgebraicTubeLensJSON(w io.Writer, l AlgebraicTubeLens) error {
	if l.graphs[0] == nil {
		return geom.ErrInvalid
	}
	return WriteTrimmedSplinePairSectionJSON(w, l.section)
}

func ReadAlgebraicTubeLensJSON(ctx context.Context, r io.Reader, limits JSONLimits,
	section AlgebraicTrimmedPairSectionOptions, o AlgebraicTubeLensOptions) (AlgebraicTubeLens, error) {
	q, e := ReadTrimmedSplinePairSectionJSON(ctx, r, limits, section)
	if e != nil {
		return AlgebraicTubeLens{}, e
	}
	return q.SealAlgebraicTubeLens(ctx, o)
}

func WriteAlgebraicProjectedTubeLensJSON(w io.Writer, l AlgebraicProjectedTubeLens) error {
	if l.graphs[0] == nil {
		return geom.ErrInvalid
	}
	return WriteTrimmedSplinePairSectionJSON(w, l.section)
}

func ReadAlgebraicProjectedTubeLensJSON(ctx context.Context, r io.Reader, l JSONLimits,
	section AlgebraicTrimmedPairSectionOptions, o AlgebraicTubeLensOptions) (AlgebraicProjectedTubeLens, error) {
	s, e := ReadTrimmedSplinePairSectionJSON(ctx, r, l, section)
	if e != nil {
		return AlgebraicProjectedTubeLens{}, e
	}
	return s.SealProjectedAlgebraicTubeLens(ctx, o)
}

// WriteAlgebraicTubeRegionsJSON stores original support, trim and tube
// proposals; containment and all face incidence are derived again on read.
func WriteAlgebraicTubeRegionsJSON(w io.Writer, r AlgebraicTubeRegions) error {
	if len(r.regions) == 0 {
		return geom.ErrInvalid
	}
	return WriteTrimmedSplinePairSectionJSON(w, r.section)
}

func ReadAlgebraicTubeRegionsJSON(ctx context.Context, r io.Reader, limits JSONLimits,
	section AlgebraicTrimmedPairSectionOptions, o AlgebraicTubeRegionOptions) (AlgebraicTubeRegions, error) {
	q, e := ReadTrimmedSplinePairSectionJSON(ctx, r, limits, section)
	if e != nil {
		return AlgebraicTubeRegions{}, e
	}
	return q.SealAlgebraicTubeRegions(ctx, o)
}
