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

// WriteNodalCubicCut transfers the separate original loop and arm solids.
// The shared point keeps two exact component-local vertex entities.
func WriteNodalCubicCut(ctx context.Context, w io.Writer, cut *trimmed.NodalCubicCut, name string, unit float64, l Limits, o NodalCubicSTEPOptions) error {
	parts := cut.Components()
	if len(parts) != 2 {
		return geom.ErrInvalid
	}
	p := cut.Contact()
	contacts := []trimmed.PolynomialMixedCutContact{{TwoPolynomialNodeComponentContact: trimmed.TwoPolynomialNodeComponentContact{
		Point: p.Point, Components: [2]int{0, 1}, Vertices: p.Vertices,
	}}}
	return writeClippedCutSTEPBodies(ctx, w, parts, contacts, name, unit, l, o)
}

// VerifyNodalCubicCutSTEP authenticates both separate components and their
// distinct contact occurrences against the same native source proof.
func VerifyNodalCubicCutSTEP(ctx context.Context, input io.Reader, cut *trimmed.NodalCubicCut, name string, unit float64, l Limits, o NodalCubicSTEPOptions) error {
	if cut == nil {
		return geom.ErrInvalid
	}
	return verifyClippedCutSTEPBodies(ctx, input, l, func(w io.Writer) error {
		return WriteNodalCubicCut(ctx, w, cut, name, unit, l, o)
	})
}
