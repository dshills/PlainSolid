package step

import (
	"context"
	"io"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// WriteCuspidalCubicCut transfers the original closed Cut shell with
// the two exact cusp germs and their unchanged roof supports.
func WriteCuspidalCubicCut(ctx context.Context, w io.Writer, cut *trimmed.CuspidalCubicCut, name string, unit float64, l Limits, o CuspidalCubicSTEPOptions) error {
	if cut == nil {
		return geom.ErrInvalid
	}
	return writeExactBezierSingleSTEP(ctx, w, cut.Boundary(), name, unit, l, o)
}

// VerifyCuspidalCubicCutSTEP checks the canonical topology, physical
// units, original supports and both directed edge uses against the source.
func VerifyCuspidalCubicCutSTEP(ctx context.Context, input io.Reader, cut *trimmed.CuspidalCubicCut, name string, unit float64, l Limits, o CuspidalCubicSTEPOptions) error {
	if cut == nil {
		return geom.ErrInvalid
	}
	return verifyExactBezierSingleSTEP(ctx, input, l, "cuspidal cubic Cut", func(w io.Writer) error {
		return WriteCuspidalCubicCut(ctx, w, cut, name, unit, l, o)
	})
}
