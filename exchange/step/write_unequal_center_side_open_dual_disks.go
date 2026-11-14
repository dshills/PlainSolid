package step

import (
	"context"
	"io"
	"math/big"
)

const unequalCenterSideDiskProductID = "side-cut-unequal-centers"

// UnequalCenterSideOpenDualDiskDescriptor selects the explicit unequal center
// ordinate family. Join is in the original stock-local coordinates and must
// satisfy both original circles exactly; it is never a snapping hint.
type UnequalCenterSideOpenDualDiskDescriptor struct {
	SideOpenDualDiskDescriptor
	Join [2]*big.Rat
}

// WriteUnequalCenterSideOpenDualDiskProduct proves the native exact-wall solid
// before emitting its rooted model-only analytic graph. Existing common-ordinate
// dialects remain distinct. Rational join inputs are borrowed only during the call.
func WriteUnequalCenterSideOpenDualDiskProduct(ctx context.Context, w io.Writer, d UnequalCenterSideOpenDualDiskDescriptor) error {
	return writeSideOpenDualDiskProduct(ctx, w, d.SideOpenDualDiskDescriptor, sideDiskUnequalCenter, d.Join)
}
