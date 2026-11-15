package trimmed

import (
	"context"
	"math/big"
)

// IntersectCoincidentOrthogonalTrims admits simple independently validated
// axis-aligned polygon loops, including concave outer/hole boundaries. It
// preserves the rectangular report's exact shared-chart and original owner
// semantics. Every original vertex coordinate enters the arrangement grid;
// neither bounding rectangles nor sample approximations replace the loops.
// Disconnected positive-area pieces remain separate tiles; lower-dimensional
// contacts are returned only where no tile closure already covers them.
func (a Solid) IntersectCoincidentOrthogonalTrims(ctx context.Context, faceA int, b Solid, faceB int, o CoincidentRectangleOverlapOptions) (CoincidentRectangleOverlap, error) {
	return a.intersectCoincidentOrthogonalTrims(ctx, faceA, b, faceB, o, false)
}

// 0 outside, 1 strict interior, 2 boundary. Exact half-open vertical crossings
// count a vertex only once. This is used only for certified axis-aligned loops.
func coincidentOrthogonalLocation(loop [][2]*big.Rat, p [2]*big.Rat) int {
	inside := false
	for i, a := range loop {
		b := loop[(i+1)%len(loop)]
		loX, hiX := a[0], b[0]
		if loX.Cmp(hiX) > 0 {
			loX, hiX = hiX, loX
		}
		loY, hiY := a[1], b[1]
		if loY.Cmp(hiY) > 0 {
			loY, hiY = hiY, loY
		}
		if p[0].Cmp(loX) >= 0 && p[0].Cmp(hiX) <= 0 && p[1].Cmp(loY) >= 0 && p[1].Cmp(hiY) <= 0 {
			return 2
		}
		if a[0].Cmp(b[0]) == 0 && p[1].Cmp(loY) >= 0 && p[1].Cmp(hiY) < 0 && p[0].Cmp(a[0]) < 0 {
			inside = !inside
		}
	}
	if inside {
		return 1
	}
	return 0
}
