package trimmed

import (
	"context"
	"io"

	"plainsolid/geom"
)

const nativeNodalCubicFuse = "trimmed-nodal-cubic-fuse"

// NodalCubicFuseOptions bounds the original chart, clearance and section proofs.
type NodalCubicFuseOptions = NodalCubicCommonOptions

// NodalCubicFuseHistory records the original nodes, branch chains, sectors and seam.
type NodalCubicFuseHistory = NodalCubicCommonHistory

// NodalCubicFuse retains the exact union over the original shared base and
// the upper selections of both original nodal cubic roof graphs.
type NodalCubicFuse struct {
	solid     Solid
	history   NodalCubicFuseHistory
	sources   [3]*geom.BSplineSurface
	axis      int
	tolerance float64
}

func (r *NodalCubicFuse) Boundary() Solid {
	if r == nil {
		return Solid{}
	}
	return r.solid
}

func (r *NodalCubicFuse) History() NodalCubicFuseHistory {
	if r == nil {
		return NodalCubicFuseHistory{}
	}
	return r.history
}

// FuseNodalCubicSplineGraphs builds the exact upper envelope over the shared
// rectangular footprint. The source contact's canonical axes point along
// positive model X and Y, and all native vertices must be exactly represented.
func FuseNodalCubicSplineGraphs(ctx context.Context, base, a, b *geom.BSplineSurface, freeAxis int,
	tolerance float64, o NodalCubicFuseOptions) (*NodalCubicFuse, error) {
	sources := [3]*geom.BSplineSurface{base, a, b}
	solid, h, err := buildNodalCubicEnvelope(ctx, sources, freeAxis, tolerance, true, o)
	if err != nil {
		return nil, err
	}
	return &NodalCubicFuse{solid: solid, history: h, sources: sources, axis: freeAxis, tolerance: tolerance}, nil
}

// WriteJSON records the original supports and the source-reproved upper
// selection, seam and node incidence.
func (r *NodalCubicFuse) WriteJSON(ctx context.Context, w io.Writer) error {
	if ctx == nil || w == nil || r == nil || r.sources[0] == nil {
		return geom.ErrInvalid
	}
	return writeNodalCubicEnvelopeRecipe(ctx, w, nativeNodalCubicFuse, r.sources, r.axis, r.tolerance, r.history)
}

// ReadNodalCubicFuseJSON reconstructs the original upper envelope and
// verifies its complete native seam and contact ledger.
func ReadNodalCubicFuseJSON(ctx context.Context, r io.Reader, limits JSONLimits, o NodalCubicFuseOptions) (*NodalCubicFuse, error) {
	recipe, source, err := readNodalCubicEnvelopeRecipe(ctx, r, limits, nativeNodalCubicFuse)
	if err != nil {
		return nil, err
	}
	out, err := FuseNodalCubicSplineGraphs(ctx, source[0], source[1], source[2], *recipe.Axis, recipe.Tolerance, o)
	if err != nil {
		return nil, err
	}
	if err := recipe.verify(ctx, out.solid, out.history, limits); err != nil {
		return nil, err
	}
	return out, nil
}
