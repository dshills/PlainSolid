package iges

import (
	"context"
	"io"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// AcnodalCubicCutIGESOptions controls exact coedge reproof and the decimal
// bound for derived model and parameter-space poles.
type AcnodalCubicCutIGESOptions = ClippedPolynomialFuseIGESOptions

// WriteAcnodalCubicCut transfers the original Cut layer as a closed type
// 186 solid, retaining the regular arc and its two original roof coedges.
func WriteAcnodalCubicCut(ctx context.Context, w io.Writer, cut *trimmed.AcnodalCubicCut, unit float64, l Limits, o AcnodalCubicCutIGESOptions) error {
	if cut == nil {
		return geom.ErrInvalid
	}
	return writeExactBezierSingleIGESMode(ctx, w, cut.Boundary(), unit, l, o, true)
}

// VerifyAcnodalCubicCutIGES authenticates the topology, original
// owner curves and intended physical units against the native Cut.
func VerifyAcnodalCubicCutIGES(ctx context.Context, input io.Reader, cut *trimmed.AcnodalCubicCut, unit float64, l Limits, o AcnodalCubicCutIGESOptions) error {
	if cut == nil {
		return geom.ErrInvalid
	}
	return verifyExactBezierSingleIGES(ctx, input, l, "acnodal cubic Cut", func(w io.Writer) error {
		return WriteAcnodalCubicCut(ctx, w, cut, unit, l, o)
	})
}
