package step

import (
	"context"
	"io"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// AcnodalCubicCutSTEPOptions controls exact coedge reproof and the decimal
// bound for derived model and parameter-space poles.
type AcnodalCubicCutSTEPOptions = ClippedPolynomialFuseSTEPOptions

// WriteAcnodalCubicCut transfers the original closed Cut shell with
// both regular arc restrictions and their unchanged roof supports.
func WriteAcnodalCubicCut(ctx context.Context, w io.Writer, cut *trimmed.AcnodalCubicCut, name string, unit float64, l Limits, o AcnodalCubicCutSTEPOptions) error {
	if cut == nil {
		return geom.ErrInvalid
	}
	return writeExactBezierSingleSTEPMode(ctx, w, cut.Boundary(), name, unit, l, o, true)
}

// VerifyAcnodalCubicCutSTEP checks the canonical topology, physical
// units, original supports and both directed edge uses against the source.
func VerifyAcnodalCubicCutSTEP(ctx context.Context, input io.Reader, cut *trimmed.AcnodalCubicCut, name string, unit float64, l Limits, o AcnodalCubicCutSTEPOptions) error {
	if cut == nil {
		return geom.ErrInvalid
	}
	return verifyExactBezierSingleSTEP(ctx, input, l, "acnodal cubic Cut", func(w io.Writer) error {
		return WriteAcnodalCubicCut(ctx, w, cut, name, unit, l, o)
	})
}
