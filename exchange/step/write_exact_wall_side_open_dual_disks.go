package step

import (
	"context"
	"errors"
	"io"
	"plainsolid/brep/trimmed"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

const exactWallSideDiskProductID = "side-cut-exact-walls"

// ExactWallSideOpenDualDiskDescriptor selects the named external-radical domain
// requiring exact homogeneous native walls that cannot be stored by the ordinary
// float-spline constructor. Model vertices remain exactly represented binary64;
// this does not permit rounded endpoints or arbitrary rational STEP support.
type ExactWallSideOpenDualDiskDescriptor SideOpenDualDiskDescriptor

// WriteExactWallSideOpenDualDiskProduct obtains an immutable, continuously sewn
// native exact-wall solid and emits independently checked analytic model-only
// topology. The AP203 product identifier explicitly registers this dialect.
// Ordinary float-wall shapes and the older three source categories reject here.
func WriteExactWallSideOpenDualDiskProduct(ctx context.Context, w io.Writer, d ExactWallSideOpenDualDiskDescriptor) error {
	if ctx == nil || w == nil || d.Discs[0].Radius == d.Discs[1].Radius {
		return geom.ErrDomain
	}
	section, e := trimmed.CertifyGeneralSideOpenDualDiskSection(ctx, d.Min, d.Max, d.Discs, trimvalidate.Options{})
	if e != nil {
		return e
	}
	for _, q := range section.Join {
		if _, exact := q.Float64(); !exact {
			return geom.ErrUnrepresentable
		}
	}
	lo, hi := sideRat(d.Discs[0].Center.X), sideRat(d.Discs[1].Center.X)
	if lo.Cmp(hi) > 0 {
		lo, hi = hi, lo
	}
	if section.Join[0].Cmp(lo) >= 0 && section.Join[0].Cmp(hi) <= 0 {
		return geom.ErrDomain
	}
	// Record the class through proof, not merely the caller's marker. Other native
	// failures cannot be recast as a request for exact homogeneous wall storage.
	_, e = trimmed.CutGeneralSideOpenDualDisks(ctx, d.Frame, d.Min, d.Max, d.Discs, d.Height, d.Depth, d.Tolerance, trimvalidate.Options{})
	if e == nil {
		return geom.ErrDomain
	}
	if !errors.Is(e, geom.ErrUnrepresentable) {
		return e
	}
	return writeSideOpenDualDiskProduct(ctx, w, SideOpenDualDiskDescriptor(d), sideDiskExactWalls)
}
