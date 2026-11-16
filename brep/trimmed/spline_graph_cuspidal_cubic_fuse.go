package trimmed

import (
	"context"
	"io"

	"plainsolid/geom"
)

const nativeCuspidalCubicFuse = "trimmed-cuspidal-cubic-fuse"

// CuspidalCubicFuseOptions bounds the stock, chart and cusp proofs.
type CuspidalCubicFuseOptions = CuspidalCubicCommonOptions

// CuspidalCubicFuseHistory binds the exact signed roof regions and directed
// cusp germs to the original stock and native shell.
type CuspidalCubicFuseHistory = CuspidalCubicCommonHistory

// CuspidalCubicFuse retains the upper envelope of the two original graphs.
type CuspidalCubicFuse struct {
	solid     Solid
	history   CuspidalCubicFuseHistory
	section   *geom.CuspidalCubicSection
	sources   [3]*geom.BSplineSurface
	freeAxis  int
	tolerance float64
}

func (r *CuspidalCubicFuse) Boundary() Solid {
	if r == nil {
		return Solid{}
	}
	return r.solid
}

func (r *CuspidalCubicFuse) History() CuspidalCubicFuseHistory {
	if r == nil {
		return CuspidalCubicFuseHistory{}
	}
	return r.history
}

// FuseCuspidalCubicSplineGraphs builds the upper envelope over a positive
// rectangular stock. It retains both original roof chart laws and both
// singular cusp germs. Native vertices must be exactly represented.
func FuseCuspidalCubicSplineGraphs(ctx context.Context, base, a, c *geom.BSplineSurface, freeAxis int,
	tolerance float64, o CuspidalCubicFuseOptions) (*CuspidalCubicFuse, error) {
	boundary, err := buildCuspidalCubicEnvelope(ctx, base, a, c, freeAxis, tolerance, true, o)
	if err != nil {
		return nil, err
	}
	return &CuspidalCubicFuse{solid: boundary.solid, history: boundary.history, section: boundary.section,
		sources: [3]*geom.BSplineSurface{base, a, c}, freeAxis: freeAxis, tolerance: tolerance}, nil
}

// WriteJSON records the original supports, upper roof selection, and exact
// cusp incidence and owner jets.
func (r *CuspidalCubicFuse) WriteJSON(ctx context.Context, w io.Writer) error {
	if ctx == nil || w == nil || r == nil || r.section == nil {
		return geom.ErrInvalid
	}
	return writeCuspidalCubicEnvelopeRecipe(ctx, w, nativeCuspidalCubicFuse, r.sources, r.freeAxis, r.tolerance, r.history, r.section)
}

// ReadCuspidalCubicFuseJSON re-proves the original cusp and the Fuse boundary
// before checking the stored source and topology ledger.
func ReadCuspidalCubicFuseJSON(ctx context.Context, r io.Reader, limits JSONLimits,
	o CuspidalCubicFuseOptions) (*CuspidalCubicFuse, error) {
	recipe, stock, err := readCuspidalCubicEnvelopeRecipe(ctx, r, limits, nativeCuspidalCubicFuse)
	if err != nil {
		return nil, err
	}
	out, err := FuseCuspidalCubicSplineGraphs(ctx, stock[0], stock[1], stock[2], *recipe.Axis, recipe.Tolerance, o)
	if err != nil {
		return nil, err
	}
	if err := recipe.verify(ctx, out.solid, out.history, out.section, limits); err != nil {
		return nil, err
	}
	return out, nil
}
