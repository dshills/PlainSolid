package step

import (
	"context"
	"io"
	"plainsolid/geom"
)

// UnequalSideOpenDualDiskDescriptor describes the exact qualified unequal-radius
// same-side two-disk cut. Fields have the same placement, stock, unit and depth
// meanings as SideOpenDualDiskDescriptor, but this is a separate shape domain.
// The native rational crossing must be strictly between centers and all model
// coordinates/control data must satisfy the lossless native constructor.
type UnequalSideOpenDualDiskDescriptor SideOpenDualDiskDescriptor

// WriteUnequalSideOpenDualDiskProduct certifies the unequal native construction
// and emits its retained model-only STEP topology with original cylinder radii
// and ownership. Equal radii belong to WriteSideOpenDualDiskProduct. The equal
// reader is not widened; this writer alone does not provide unequal import.
func WriteUnequalSideOpenDualDiskProduct(ctx context.Context, w io.Writer, d UnequalSideOpenDualDiskDescriptor) error {
	if d.Discs[0].Radius == d.Discs[1].Radius {
		return geom.ErrDomain
	}
	return writeSideOpenDualDiskProduct(ctx, w, SideOpenDualDiskDescriptor(d), true)
}
