package step

import (
	"context"
	"io"
	"plainsolid/brep/trimmed"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// GeneralSideOpenDualDiskDescriptor describes the separately qualified external-
// radical unequal domain: the exact transverse upper circle join is strictly
// outside the interval between cutter centers. Field meanings match the equal
// and unequal descriptors, but those categories remain separate.
type GeneralSideOpenDualDiskDescriptor SideOpenDualDiskDescriptor

// WriteGeneralSideOpenDualDiskProduct builds the exact general native cut and
// exports checked model-only topology. Both circular walls must remain lossless
// native minor-arc charts. Joins between or at centers use another domain and
// reject here. This writer does not expand either existing reader.
func WriteGeneralSideOpenDualDiskProduct(ctx context.Context, w io.Writer, d GeneralSideOpenDualDiskDescriptor) error {
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
	return writeSideOpenDualDiskProduct(ctx, w, SideOpenDualDiskDescriptor(d), sideDiskGeneral)
}
