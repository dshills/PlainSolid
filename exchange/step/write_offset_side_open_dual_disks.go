package step

import (
	"context"
	"io"
)

const offsetSideDiskProductID = "side-cut-offset-disks"

// OffsetSideOpenDualDiskDescriptor selects unequal cutters whose common center
// lies strictly outside the stock side. The native exact section and original
// per-cutter ownership are certified before emitting model-only analytic STEP.
type OffsetSideOpenDualDiskDescriptor SideOpenDualDiskDescriptor

// WriteOffsetSideOpenDualDiskProduct emits a registered AP203 product for the
// bounded offset blind/through family. No pcurves or rounded controls are emitted.
func WriteOffsetSideOpenDualDiskProduct(ctx context.Context, w io.Writer, d OffsetSideOpenDualDiskDescriptor) error {
	return writeSideOpenDualDiskProduct(ctx, w, SideOpenDualDiskDescriptor(d), sideDiskOffset)
}
