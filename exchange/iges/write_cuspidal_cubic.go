package iges

import (
	"context"
	"io"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// CuspidalCubicIGESOptions controls original coedge reproof and decimal
// representation of derived model and parameter-space poles.
type CuspidalCubicIGESOptions = ClippedPolynomialFuseIGESOptions

// WriteCuspidalCubicCommon transfers the original Common as a closed type
// 186 solid, retaining the exact cusp germs and their original coedges.
func WriteCuspidalCubicCommon(ctx context.Context, w io.Writer, common *trimmed.CuspidalCubicCommon, unit float64, l Limits, o CuspidalCubicIGESOptions) error {
	if common == nil {
		return geom.ErrInvalid
	}
	return writeExactBezierSingleIGES(ctx, w, common.Boundary(), unit, l, o)
}

// VerifyCuspidalCubicCommonIGES authenticates the topology, original
// owner curves and intended physical units against the native Common.
func VerifyCuspidalCubicCommonIGES(ctx context.Context, input io.Reader, common *trimmed.CuspidalCubicCommon, unit float64, l Limits, o CuspidalCubicIGESOptions) error {
	if common == nil {
		return geom.ErrInvalid
	}
	return verifyExactBezierSingleIGES(ctx, input, l, "cuspidal cubic Common", func(w io.Writer) error {
		return WriteCuspidalCubicCommon(ctx, w, common, unit, l, o)
	})
}
