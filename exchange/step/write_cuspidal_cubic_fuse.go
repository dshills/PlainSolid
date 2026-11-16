package step

import (
	"context"
	"io"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// WriteCuspidalCubicFuse transfers the original closed Fuse shell with
// the two exact cusp germs and their unchanged roof supports.
func WriteCuspidalCubicFuse(ctx context.Context, w io.Writer, fuse *trimmed.CuspidalCubicFuse, name string, unit float64, l Limits, o CuspidalCubicSTEPOptions) error {
	if fuse == nil {
		return geom.ErrInvalid
	}
	return writeExactBezierSingleSTEP(ctx, w, fuse.Boundary(), name, unit, l, o)
}

// VerifyCuspidalCubicFuseSTEP checks the canonical topology, physical
// units, original supports and both directed edge uses against the source.
func VerifyCuspidalCubicFuseSTEP(ctx context.Context, input io.Reader, fuse *trimmed.CuspidalCubicFuse, name string, unit float64, l Limits, o CuspidalCubicSTEPOptions) error {
	if fuse == nil {
		return geom.ErrInvalid
	}
	return verifyExactBezierSingleSTEP(ctx, input, l, "cuspidal cubic Fuse", func(w io.Writer) error {
		return WriteCuspidalCubicFuse(ctx, w, fuse, name, unit, l, o)
	})
}
