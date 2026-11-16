package step

import (
	"context"
	"io"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// NodalCubicSTEPOptions controls original coedge reproof and an optional
// per-coordinate decimal bound for derived model and pcurve poles.
type NodalCubicSTEPOptions = ClippedPolynomialFuseSTEPOptions

// WriteNodalCubicCommon transfers the exact original Common shell, with its
// four node rays, both original roof charts and the same-owner exterior seam.
func WriteNodalCubicCommon(ctx context.Context, w io.Writer, common *trimmed.NodalCubicCommon, name string, unit float64, l Limits, o NodalCubicSTEPOptions) error {
	if common == nil {
		return geom.ErrInvalid
	}
	return writeExactBezierSingleSTEP(ctx, w, common.Boundary(), name, unit, l, o)
}

// VerifyNodalCubicCommonSTEP checks canonical topology, units, supports and
// all original coedges against a native Common proof.
func VerifyNodalCubicCommonSTEP(ctx context.Context, input io.Reader, common *trimmed.NodalCubicCommon, name string, unit float64, l Limits, o NodalCubicSTEPOptions) error {
	if common == nil {
		return geom.ErrInvalid
	}
	return verifyExactBezierSingleSTEP(ctx, input, l, "nodal cubic Common", func(w io.Writer) error {
		return WriteNodalCubicCommon(ctx, w, common, name, unit, l, o)
	})
}

// WriteNodalCubicFuse transfers the exact original Fuse shell, retaining the
// selected original roof on each side of the loop, arm and owner-chart seam.
func WriteNodalCubicFuse(ctx context.Context, w io.Writer, fuse *trimmed.NodalCubicFuse, name string, unit float64, l Limits, o NodalCubicSTEPOptions) error {
	if fuse == nil {
		return geom.ErrInvalid
	}
	return writeExactBezierSingleSTEP(ctx, w, fuse.Boundary(), name, unit, l, o)
}

// VerifyNodalCubicFuseSTEP checks canonical topology, units, supports and
// all original coedges against a native Fuse proof.
func VerifyNodalCubicFuseSTEP(ctx context.Context, input io.Reader, fuse *trimmed.NodalCubicFuse, name string, unit float64, l Limits, o NodalCubicSTEPOptions) error {
	if fuse == nil {
		return geom.ErrInvalid
	}
	return verifyExactBezierSingleSTEP(ctx, input, l, "nodal cubic Fuse", func(w io.Writer) error {
		return WriteNodalCubicFuse(ctx, w, fuse, name, unit, l, o)
	})
}
