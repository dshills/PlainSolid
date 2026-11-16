package step

import (
	"context"
	"io"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// CuspidalCubicSTEPOptions controls the original coedge proof and the optional
// decimal bound for derived model and pcurve poles.
type CuspidalCubicSTEPOptions = ClippedPolynomialFuseSTEPOptions

// WriteCuspidalCubicCommon transfers the original closed Common shell with
// the two exact cusp germs and their unchanged roof supports.
func WriteCuspidalCubicCommon(ctx context.Context, w io.Writer, common *trimmed.CuspidalCubicCommon, name string, unit float64, l Limits, o CuspidalCubicSTEPOptions) error {
	if common == nil {
		return geom.ErrInvalid
	}
	return writeExactBezierSingleSTEP(ctx, w, common.Boundary(), name, unit, l, o)
}

// VerifyCuspidalCubicCommonSTEP checks the canonical topology, physical
// units, original supports and both directed edge uses against the source.
func VerifyCuspidalCubicCommonSTEP(ctx context.Context, input io.Reader, common *trimmed.CuspidalCubicCommon, name string, unit float64, l Limits, o CuspidalCubicSTEPOptions) error {
	if common == nil {
		return geom.ErrInvalid
	}
	return verifyExactBezierSingleSTEP(ctx, input, l, "cuspidal cubic Common", func(w io.Writer) error {
		return WriteCuspidalCubicCommon(ctx, w, common, name, unit, l, o)
	})
}
