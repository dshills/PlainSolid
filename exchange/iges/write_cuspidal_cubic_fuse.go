package iges

import (
	"context"
	"io"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// WriteCuspidalCubicFuse transfers the original Fuse as a closed type
// 186 solid, retaining the exact cusp germs and their original coedges.
func WriteCuspidalCubicFuse(ctx context.Context, w io.Writer, fuse *trimmed.CuspidalCubicFuse, unit float64, l Limits, o CuspidalCubicIGESOptions) error {
	if fuse == nil {
		return geom.ErrInvalid
	}
	return writeExactBezierSingleIGESMode(ctx, w, fuse.Boundary(), unit, l, o, true)
}

// VerifyCuspidalCubicFuseIGES authenticates the topology, original
// owner curves and intended physical units against the native Fuse.
func VerifyCuspidalCubicFuseIGES(ctx context.Context, input io.Reader, fuse *trimmed.CuspidalCubicFuse, unit float64, l Limits, o CuspidalCubicIGESOptions) error {
	if fuse == nil {
		return geom.ErrInvalid
	}
	return verifyExactBezierSingleIGES(ctx, input, l, "cuspidal cubic Fuse", func(w io.Writer) error {
		return WriteCuspidalCubicFuse(ctx, w, fuse, unit, l, o)
	})
}
